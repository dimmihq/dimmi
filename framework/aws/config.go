/*
|--------------------------------------------------------------------------
| AWS Configuration
|--------------------------------------------------------------------------
|
| Every AWS service client starts from one configuration built here, so
| API calls leave through the safe HTTP client like any other outbound
| request. A self-hosted S3-compatible store on a private address must
| then be allowed in the HTTP client's configuration, exactly like any
| other intranet service.
|
| Credentials left empty come from the standard AWS chain: environment,
| shared config files, or the instance or task role. Roles are served on
| link-local metadata addresses the safe client refuses, so the chain
| keeps the SDK's own transport for fetching credentials; only service
| calls use the safe client.
|
*/

package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
Options are the configuration section of an AWS account.

Endpoint replaces the AWS endpoint of every service, for S3-compatible
stores and local emulators; empty uses AWS, or the AWS_ENDPOINT_URL
variables when set. Static keys are used when the access key ID is set.
*/
type Options struct {
	Region          string `default:"us-east-1" koanf:"region"`
	Endpoint        string `koanf:"endpoint"`
	AccessKeyID     string `koanf:"access_key_id"`
	SecretAccessKey string `koanf:"secret_access_key"`
}

/*
Config builds the SDK configuration from the section, with service calls
sent through the client.
*/
func Config(ctx context.Context, section config.Section, client *httpclient.Client) (awssdk.Config, error) {
	var opts Options
	if err := section.Decode(&opts); err != nil {
		return awssdk.Config{}, fmt.Errorf("aws config: %w", err)
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(opts.Region)}
	if opts.AccessKeyID != "" {
		static := credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, "")
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(static))
	}

	if opts.Endpoint != "" {
		loadOpts = append(loadOpts, awsconfig.WithBaseEndpoint(opts.Endpoint))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return awssdk.Config{}, fmt.Errorf("load aws config: %w", err)
	}

	cfg.HTTPClient = client.StdClient()

	return cfg, nil
}
