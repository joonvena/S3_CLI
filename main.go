package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/olekukonko/tablewriter"
)

// Bucket type for building the list of buckets
type Bucket struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

func createTable(allContent []Bucket) *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Region"})

	for _, v := range allContent {
		table.Append([]string{v.Name, v.Region})
	}

	return table

}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func confirmationPrompt() rune {
	fmt.Println("\n\nAre you sure you want to delete these buckets and their content? (y/n)")

	reader := bufio.NewReader(os.Stdin)
	char, _, err := reader.ReadRune()

	if err != nil {
		fmt.Println(err)
	}

	return char
}

func getBucketRegion(svc *s3.S3, bucket *string) string {

	ctx := context.Background()
	region, err := s3manager.GetBucketRegionWithClient(ctx, svc, *bucket)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			fmt.Fprintf(os.Stderr, "unable to find bucket %s's region not found\n", *bucket)
		}
		fmt.Println("Error happened", err)
	}
	return region
}

func listBuckets(svc *s3.S3, err error, region *string) {
	var allContent []Bucket
	result, err := svc.ListBuckets(nil)
	if err != nil {
		exitErrorf("Unable to list buckets, %v", err)
	}

	for _, b := range result.Buckets {

		input := []byte(fmt.Sprintf(`[{
			"name": "%v",
			"region": "%v"	
		}]`, aws.StringValue(b.Name), getBucketRegion(svc, b.Name)))

		var tmpBuckets []Bucket
		err := json.Unmarshal(input, &tmpBuckets)
		if err != nil {
			log.Fatal(err)
		}

		allContent = append(allContent, tmpBuckets...)

	}

	// Create table to display buckets
	table := createTable(allContent)
	table.Render()

}

func delete(svc *s3.S3, err error, bucket string) {
	iter := s3manager.NewDeleteListIterator(svc, &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	})

	if err := s3manager.NewBatchDeleteWithClient(svc).Delete(aws.BackgroundContext(), iter); err != nil {
		exitErrorf("Unable to delete objects from bucket %q, %v", bucket, err)
	}
	fmt.Printf("Deleted object(s) from bucket: %v", bucket)

	_, err = svc.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		exitErrorf("Unable to delete bucket %q, %v", bucket, err)
	}

	// Wait until bucket is deleted before finishing
	fmt.Printf(" Waiting for bucket %q to be deleted...\n", bucket)

	err = svc.WaitUntilBucketNotExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
}

func deleteBuckets(svc *s3.S3, err error, region, bucket, profile *string) {
	var allContent []Bucket

	result, err := svc.ListBuckets(nil)
	if err != nil {
		exitErrorf("Unable to list buckets, %v", err)
	}

	for _, b := range result.Buckets {
		if strings.Contains(aws.StringValue(b.Name), *bucket) {
			input := []byte(fmt.Sprintf(`[{
				"name": "%v",
				"region": "%v"	
			}]`, aws.StringValue(b.Name), getBucketRegion(svc, b.Name)))

			var tmpBuckets []Bucket
			err := json.Unmarshal(input, &tmpBuckets)
			if err != nil {
				log.Fatal(err)
			}

			allContent = append(allContent, tmpBuckets...)
		}
	}

	if allContent != nil {

		// Create table to display buckets
		table := createTable(allContent)
		table.Render()

		// Ask user to confirm deletion
		prompt := confirmationPrompt()

		if prompt == 121 {
			for _, bucket := range allContent {
				if region != aws.String(bucket.Region) {
					svc, err := createSession(*profile, *aws.String(bucket.Region))
					delete(svc, err, bucket.Name)
				} else {
					delete(svc, err, bucket.Name)
				}
			}
		} else {
			fmt.Println("Deletion cancelled!")
		}
	} else {
		fmt.Println("Buckets not found")
	}

}

func createSession(profile, region string) (*s3.S3, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profile,

		Config: aws.Config{
			Region: aws.String(region),
		},

		SharedConfigState: session.SharedConfigEnable,
	})

	svc := s3.New(sess)
	return svc, err
}

func main() {

	// Argument what to do
	cmdArgs := flag.NewFlagSet("s3_cli", flag.ExitOnError)

	profile := cmdArgs.String("profile", "dev", "profile to use")
	region := cmdArgs.String("region", "eu-west-1", "aws region")
	bucket := cmdArgs.String("bucket", "", "bucket name")

	cmdArgs.Parse(os.Args[2:])

	svc, err := createSession(*profile, *region)

	switch os.Args[1] {

	case "list":
		listBuckets(svc, err, region)

	case "delete":
		deleteBuckets(svc, err, region, bucket, profile)

	default:
		fmt.Println(`Invalid command. "list" and "delete" available`)
		os.Exit(1)
	}

}
