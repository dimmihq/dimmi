/*
|--------------------------------------------------------------------------
| AWS KMS Key Encryption Key
|--------------------------------------------------------------------------
|
| In the cloud, data keys are wrapped by a KMS key that never leaves AWS:
| every wrap and unwrap is a KMS call, allowed by IAM and recorded in
| CloudTrail. Associated data travels as the KMS encryption context, in
| the same form Tink's own KMS integration uses, so keysets wrapped here
| and by any Tink library open each other.
|
| The AWS configuration comes from the caller, which already resolved
| region and credentials for every AWS service the application uses.
|
*/

package awskms

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
ErrKeyRequired is returned when no KMS key is configured.
*/
var ErrKeyRequired = errors.New("awskms: key_arn is required")

/*
contextKey is the encryption context entry holding the associated data,
as Tink names it; tinkURIPrefix is how Tink writes a KMS key URI.
*/
const (
	contextKey    = "additionalData"
	tinkURIPrefix = "aws-kms://"
)

/*
Options configure the KEK. KeyARN is the KMS key's ARN, or an alias ARN;
a Tink "aws-kms://" URI is accepted too.
*/
type Options struct {
	KeyARN string `koanf:"key_arn"`
}

/*
KEK wraps data keys with one KMS key.
*/
type KEK struct {
	client *kms.Client
	keyID  string
}

var _ encryption.KEK = (*KEK)(nil)

/*
Factory builds KMS KEKs for the KEK manager on the caller's AWS
configuration.
*/
func Factory(cfg aws.Config) manager.Factory[encryption.KEK] {
	return func(_ string, section config.Section) (encryption.KEK, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(cfg, opts)
	}
}

/*
New returns the KEK for the configured key. Nothing is sent to KMS until
the first wrap or unwrap.
*/
func New(cfg aws.Config, opts Options) (*KEK, error) {
	keyID := strings.TrimPrefix(opts.KeyARN, tinkURIPrefix)
	if keyID == "" {
		return nil, ErrKeyRequired
	}

	return &KEK{client: kms.NewFromConfig(cfg), keyID: keyID}, nil
}

/*
Wrap encrypts the key material with the KMS key.
*/
func (k *KEK) Wrap(ctx context.Context, plaintext, associatedData []byte) ([]byte, error) {
	out, err := k.client.Encrypt(ctx, &kms.EncryptInput{
		KeyId:             aws.String(k.keyID),
		Plaintext:         plaintext,
		EncryptionContext: encryptionContext(associatedData),
	})
	if err != nil {
		return nil, fmt.Errorf("kms encrypt: %w", err)
	}

	return out.CiphertextBlob, nil
}

/*
Unwrap decrypts key material with the KMS key. Naming the key makes KMS
refuse a ciphertext made under any other key.
*/
func (k *KEK) Unwrap(ctx context.Context, wrapped, associatedData []byte) ([]byte, error) {
	out, err := k.client.Decrypt(ctx, &kms.DecryptInput{
		KeyId:             aws.String(k.keyID),
		CiphertextBlob:    wrapped,
		EncryptionContext: encryptionContext(associatedData),
	})
	if err != nil {
		return nil, fmt.Errorf("kms decrypt: %w", err)
	}

	return out.Plaintext, nil
}

/*
encryptionContext carries the associated data as hex, and nothing when
there is none, matching Tink.
*/
func encryptionContext(associatedData []byte) map[string]string {
	if len(associatedData) == 0 {
		return nil
	}

	return map[string]string{contextKey: hex.EncodeToString(associatedData)}
}
