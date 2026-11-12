/*
|--------------------------------------------------------------------------
| S3 Disk
|--------------------------------------------------------------------------
|
| One driver covers Amazon S3 and every store that speaks its API:
| Cloudflare R2, Backblaze B2, Garage, SeaweedFS and others. They differ
| only by endpoint, region and whether the bucket goes in the path or in
| the host name, so switching provider is a configuration change.
|
| Credentials left empty come from the standard AWS chain: environment,
| shared config files, or the instance role on EC2. Many compatible
| stores send no response checksums, so skipping their validation is not
| logged.
|
*/

package s3

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
ErrBucketRequired is returned when an s3 disk names no bucket.
*/
var ErrBucketRequired = errors.New("s3 disk: bucket is required")

/*
Options configure an s3 disk.

Endpoint is the store's API URL; empty means AWS, or AWS_ENDPOINT_URL_S3
when set. PublicEndpoint is where browsers reach the same store when the
app talks to it on an address they cannot, such as a store inside a
cluster behind its own public host: signed links name it, so the
signature holds for the host browsers send. ForcePathStyle puts the
bucket in the path, as SeaweedFS and Garage expect. URL is the public
base URL the files are served from.
*/
type Options struct {
	Bucket          string `koanf:"bucket"`
	Region          string `default:"us-east-1" koanf:"region"`
	Endpoint        string `koanf:"endpoint"`
	PublicEndpoint  string `koanf:"public_endpoint"`
	ForcePathStyle  bool   `koanf:"force_path_style"`
	AccessKeyID     string `koanf:"access_key_id"`
	SecretAccessKey string `koanf:"secret_access_key"`
	URL             string `koanf:"url"`
}

/*
Factory builds s3 disks for the disk manager. The context bounds loading
credentials; pass the application's root context.
*/
func Factory(ctx context.Context) manager.Factory[filesystem.Filesystem] {
	return func(_ string, section config.Section) (filesystem.Filesystem, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(ctx, opts)
	}
}

/*
New opens an s3 disk.
*/
func New(ctx context.Context, opts Options) (*filesystem.Disk, error) {
	if opts.Bucket == "" {
		return nil, ErrBucketRequired
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(opts.Region)}
	if opts.AccessKeyID != "" {
		static := credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, "")
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(static))
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsConfig, clientOptions(opts, opts.Endpoint))
	signer := client
	if opts.PublicEndpoint != "" {
		signer = s3.NewFromConfig(awsConfig, clientOptions(opts, opts.PublicEndpoint))
	}

	return filesystem.NewDisk(&store{
		client:   client,
		presign:  s3.NewPresignClient(signer),
		uploader: transfermanager.New(client),
		bucket:   opts.Bucket,
	}, filesystem.DiskOptions{URL: opts.URL}), nil
}

/*
clientOptions points a client at the endpoint, AWS when empty, in the
disk's addressing style.
*/
func clientOptions(opts Options, endpoint string) func(*s3.Options) {
	return func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}

		o.UsePathStyle = opts.ForcePathStyle
		o.DisableLogOutputChecksumValidationSkipped = true
	}
}
