/*
|--------------------------------------------------------------------------
| Test Buckets
|--------------------------------------------------------------------------
|
| Each test gets an empty bucket on the S3-compatible server the standard
| AWS variables point at (AWS_ENDPOINT_URL_S3, AWS_ACCESS_KEY_ID and
| friends), and the bucket is emptied and removed when the test ends.
|
*/

package testkit

import (
	"context"
	"testing"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

/*
NewBucket creates an empty bucket for the test and returns its name.
*/
func NewBucket(tb testing.TB) string {
	tb.Helper()

	Env(tb, "AWS_ENDPOINT_URL_S3")

	cfg, err := awsconfig.LoadDefaultConfig(tb.Context())
	if err != nil {
		tb.Fatalf("load aws config: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
	name := UniqueName("t")

	if _, err := client.CreateBucket(tb.Context(), &s3.CreateBucketInput{Bucket: &name}); err != nil {
		tb.Fatalf("create test bucket: %v", err)
	}

	tb.Cleanup(func() { removeBucket(context.WithoutCancel(tb.Context()), client, name) })

	return name
}

func removeBucket(ctx context.Context, client *s3.Client, name string) {
	objects, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &name})
	if err != nil {
		return
	}

	for _, object := range objects.Contents {
		_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &name, Key: object.Key})
	}

	_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &name})
}
