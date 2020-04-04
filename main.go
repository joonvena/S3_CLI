package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func listBuckets(svc *s3.S3, err error) {

	result, err := svc.ListBuckets(nil)
	if err != nil {
		exitErrorf("Unable to list buckets, %v", err)
	}

	for _, b := range result.Buckets {
		fmt.Printf("%v %v\n", aws.StringValue(b.Name), aws.TimeValue(b.CreationDate))
	}
}

func deleteBuckets(svc *s3.S3, err error, region *string, bucket *string) {
	var bucketsToBeDeleted []string

	result, err := svc.ListBuckets(nil)
	if err != nil {
		exitErrorf("Unable to list buckets, %v", err)
	}

	for _, b := range result.Buckets {
		if strings.Contains(aws.StringValue(b.Name), *bucket) {
			bucketsToBeDeleted = append(bucketsToBeDeleted, aws.StringValue(b.Name))
		}
	}

	stringSlices := strings.Join(bucketsToBeDeleted[:], "\n")
	fmt.Print(stringSlices)

	fmt.Println("\n\nAre you sure you want to delete these buckets and their content? (y/n)")

	reader := bufio.NewReader(os.Stdin)
	char, _, err := reader.ReadRune()

	if err != nil {
		fmt.Println(err)
		return
	}

	if char == 121 {
		for _, bucket := range bucketsToBeDeleted {
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
	} else {
		fmt.Println("Wrong input provided!")
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
		listBuckets(svc, err)

	case "delete":
		deleteBuckets(svc, err, region, bucket)
	}

}
