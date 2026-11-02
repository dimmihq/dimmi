package encryption_test

import (
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/encryption/drivers/awskms"
	"github.com/dimmihq/dimmi/framework/encryption/drivertest"
)

func TestFailures(t *testing.T) {
	t.Parallel()

	t.Run("reports a store that cannot be reached", func(t *testing.T) {
		t.Parallel()
		store, kek := drivertest.NewPostgresStore(t), localKEK(t)
		writer := newKeys(t, encryption.KeysOptions{KEK: kek, Store: store})
		ciphertext, err := writer.Encrypt(t.Context(), tenant("1"), []byte("x"), nil)
		require.NoError(t, err)
		reader := newKeys(t, encryption.KeysOptions{KEK: kek, Store: store})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err = reader.Encrypt(ctx, tenant("1"), nil, nil)
		require.ErrorIs(t, err, context.Canceled)
		_, err = reader.Decrypt(ctx, ciphertext, nil)
		require.ErrorIs(t, err, context.Canceled)
		_, err = reader.MAC(ctx, tenant("1"), nil)
		require.ErrorIs(t, err, context.Canceled)
		_, err = reader.NewStreamWriter(ctx, tenant("1"), io.Discard)
		require.ErrorIs(t, err, context.Canceled)
		_, err = reader.NewStreamReader(ctx, tenant("1"), nil)
		require.ErrorIs(t, err, context.Canceled)
		require.ErrorIs(t, reader.Rotate(ctx, tenant("1")), context.Canceled)
		require.ErrorIs(t, reader.RewrapAll(ctx), context.Canceled)
	})

	t.Run("reports a kek that cannot be reached", func(t *testing.T) {
		t.Parallel()
		kek, err := awskms.New(aws.Config{
			Region:       "eu-west-1",
			Credentials:  credentials.NewStaticCredentialsProvider("id", "key", ""),
			BaseEndpoint: aws.String("http://127.0.0.1:1"),
			Retryer:      func() aws.Retryer { return aws.NopRetryer{} },
		}, awskms.Options{KeyARN: "arn:aws:kms:eu-west-1:123456789012:key/abc"})
		require.NoError(t, err)
		keys := newKeys(t, encryption.KeysOptions{KEK: kek})

		_, err = keys.Encrypt(t.Context(), tenant("1"), []byte("x"), nil)

		require.ErrorContains(t, err, "kms encrypt")
	})
}
