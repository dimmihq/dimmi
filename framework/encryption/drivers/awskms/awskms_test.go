package awskms_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/encryption/drivers/awskms"
)

/*
unreachable returns an AWS configuration whose KMS endpoint refuses
connections, so no test ever reaches AWS.
*/
func unreachable() aws.Config {
	return aws.Config{
		Region:       "eu-west-1",
		Credentials:  credentials.NewStaticCredentialsProvider("id", "key", ""),
		BaseEndpoint: aws.String("http://127.0.0.1:1"),
		Retryer:      func() aws.Retryer { return aws.NopRetryer{} },
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("requires a key", func(t *testing.T) {
		t.Parallel()

		_, err := awskms.New(unreachable(), awskms.Options{})
		require.ErrorIs(t, err, awskms.ErrKeyRequired)
		_, err = awskms.New(unreachable(), awskms.Options{KeyARN: "aws-kms://"})
		require.ErrorIs(t, err, awskms.ErrKeyRequired)
	})

	t.Run("builds from a section without calling kms", func(t *testing.T) {
		t.Parallel()
		factory := awskms.Factory(unreachable())

		_, err := factory("kms", config.Section{"key_arn": "aws-kms://arn:aws:kms:eu-west-1:123456789012:key/abc"})
		require.NoError(t, err)

		_, err = factory("kms", config.Section{"key_arn": []int{1}})
		require.Error(t, err)
	})

	t.Run("reports kms failures", func(t *testing.T) {
		t.Parallel()
		kek, err := awskms.New(unreachable(), awskms.Options{KeyARN: "arn:aws:kms:eu-west-1:123456789012:key/abc"})
		require.NoError(t, err)

		_, err = kek.Wrap(t.Context(), []byte("dek"), []byte("tenant/1"))
		require.ErrorContains(t, err, "kms encrypt")
		_, err = kek.Unwrap(t.Context(), []byte("wrapped"), nil)
		require.ErrorContains(t, err, "kms decrypt")
	})
}
