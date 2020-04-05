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

type Bucket struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
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
		//fmt.Printf("%v %v\n", aws.StringValue(b.Name), aws.TimeValue(b.CreationDate))
		// getBucketRegion(svc, b.Name)

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
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Region"})

	for _, v := range allContent {
		table.Append([]string{v.Name, v.Region})
	}

	table.Render()
}

func deleteBuckets(svc *s3.S3, err error, region *string, bucket *string) {
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

	// Create table to display buckets
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Region"})

	for _, v := range allContent {
		table.Append([]string{v.Name, v.Region})
	}

	table.Render()

	fmt.Println("\n\nAre you sure you want to delete these buckets and their content? (y/n)")

	reader := bufio.NewReader(os.Stdin)
	char, _, err := reader.ReadRune()

	if err != nil {
		fmt.Println(err)
		return
	}

	if char == 121 {
		for _, bucket := range allContent {
			iter := s3manager.NewDeleteListIterator(svc, &s3.ListObjectsInput{
				Bucket: aws.String(bucket.Name),
			})

			if err := s3manager.NewBatchDeleteWithClient(svc).Delete(aws.BackgroundContext(), iter); err != nil {
				exitErrorf("Unable to delete objects from bucket %q, %v", bucket.Name, err)
			}
			fmt.Printf("Deleted object(s) from bucket: %v", bucket.Name)

			_, err = svc.DeleteBucket(&s3.DeleteBucketInput{
				Bucket: aws.String(bucket.Name),
			})
			if err != nil {
				exitErrorf("Unable to delete bucket %q, %v", bucket.Name, err)
			}

			// Wait until bucket is deleted before finishing
			fmt.Printf(" Waiting for bucket %q to be deleted...\n", bucket.Name)

			err = svc.WaitUntilBucketNotExists(&s3.HeadBucketInput{
				Bucket: aws.String(bucket.Name),
			})
		}
	} else {
		fmt.Println("Deletion cancelled!")
	}

}

func main() {

	// Argument what to do
	fooCmd := flag.NewFlagSet("foo", flag.ExitOnError)

	// Profile & Region arguments are needed (eg. dev, eu-west-1)
	profile := fooCmd.String("profile", "dev", "profile to use")
	region := fooCmd.String("region", "eu-west-1", "aws region")
	bucket := fooCmd.String("bucket", "bucket-1234", "bucket name")

	fooCmd.Parse(os.Args[2:])

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: *profile,

		Config: aws.Config{
			Region: aws.String(*region),
		},

		SharedConfigState: session.SharedConfigEnable,
	})

	svc := s3.New(sess)

	switch os.Args[1] {

	case "list":
		listBuckets(svc, err, region)

	case "delete":
		deleteBuckets(svc, err, region, bucket)
	}

}
