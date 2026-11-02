package aws_test

import (
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/aws"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func section(t *testing.T) config.Section {
	t.Helper()

	return config.Section{
		"region":            "eu-west-1",
		"endpoint":          testkit.Env(t, "AWS_ENDPOINT_URL_S3"),
		"access_key_id":     testkit.Env(t, "AWS_ACCESS_KEY_ID"),
		"secret_access_key": testkit.Env(t, "AWS_SECRET_ACCESS_KEY"),
	}
}

func newClient(t *testing.T, allowHosts ...string) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowHosts: allowHosts})
	require.NoError(t, err)

	return client
}

func listBuckets(t *testing.T, client *httpclient.Client) error {
	t.Helper()

	cfg, err := aws.Config(t.Context(), section(t), client)
	require.NoError(t, err)

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.RetryMaxAttempts = 1
	})
	_, err = s3Client.ListBuckets(t.Context(), &s3.ListBucketsInput{})

	return err
}

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("reads region, endpoint and static keys from the section", func(t *testing.T) {
		t.Parallel()

		cfg, err := aws.Config(t.Context(), section(t), newClient(t))
		require.NoError(t, err)

		creds, err := cfg.Credentials.Retrieve(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "eu-west-1", cfg.Region)
		assert.Equal(t, testkit.Env(t, "AWS_ENDPOINT_URL_S3"), *cfg.BaseEndpoint)
		assert.Equal(t, testkit.Env(t, "AWS_ACCESS_KEY_ID"), creds.AccessKeyID)
	})

	t.Run("calls services through the safe client", func(t *testing.T) {
		t.Parallel()
		endpoint, err := url.Parse(testkit.Env(t, "AWS_ENDPOINT_URL_S3"))
		require.NoError(t, err)

		require.NoError(t, listBuckets(t, newClient(t, endpoint.Host)))
		require.ErrorIs(t, listBuckets(t, newClient(t)), safedial.ErrBlocked)
	})

	t.Run("defaults the region and leaves credentials to the chain", func(t *testing.T) {
		t.Parallel()

		cfg, err := aws.Config(t.Context(), config.Section{}, newClient(t))

		require.NoError(t, err)
		assert.Equal(t, "us-east-1", cfg.Region)
		assert.Nil(t, cfg.BaseEndpoint)
	})

	t.Run("rejects a section it cannot decode", func(t *testing.T) {
		t.Parallel()

		_, err := aws.Config(t.Context(), config.Section{"region": map[string]any{"bad": true}}, newClient(t))

		require.Error(t, err)
	})
}
