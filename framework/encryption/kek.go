/*
|--------------------------------------------------------------------------
| Key Encryption Keys
|--------------------------------------------------------------------------
|
| Data keys are stored only wrapped by a key encryption key (KEK). The
| local KEK is derived from the app key, so a single-server installation
| needs no other secret. A KMS-backed KEK keeps the wrapping key inside
| the KMS, where it never leaves and every use is audited. Either way,
| changing the KEK rewraps the stored data keys; no data is re-encrypted.
|
| Wrapping binds associated data, so a wrapped key copied to another
| scope's row fails to unwrap.
|
*/

package encryption

import (
	"context"
	"fmt"

	"github.com/tink-crypto/tink-go/v2/aead/subtle"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
KEK wraps and unwraps data keys.
*/
type KEK interface {
	/*
		Wrap encrypts the key material, bound to the associated data.
	*/
	Wrap(ctx context.Context, plaintext, associatedData []byte) ([]byte, error)

	/*
		Unwrap decrypts what Wrap returned for the same associated data.
	*/
	Unwrap(ctx context.Context, wrapped, associatedData []byte) ([]byte, error)
}

/*
kekPurpose names the app key derivation for the local KEK.
*/
const kekPurpose = "key-encryption"

/*
LocalKEK is a KEK derived from an app key, using AES-256-GCM.
*/
type LocalKEK struct {
	cipher *subtle.AESGCM
}

var _ KEK = (*LocalKEK)(nil)

/*
LocalOptions configure a local KEK. Key is an encoded app key other
than the application's own, used to keep unwrapping data keys after the
app key changes; empty means the application's app key.
*/
type LocalOptions struct {
	Key string `koanf:"key"`
}

/*
LocalFactory builds local KEKs for the KEK manager from the app key.
*/
func LocalFactory(appKey []byte) manager.Factory[KEK] {
	return func(_ string, section config.Section) (KEK, error) {
		var opts LocalOptions
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		key := appKey
		if opts.Key != "" {
			parsed, err := ParseAppKey(opts.Key)
			if err != nil {
				return nil, err
			}

			key = parsed
		}

		return NewLocalKEK(key)
	}
}

/*
NewLocalKEK returns the KEK derived from the app key.
*/
func NewLocalKEK(appKey []byte) (*LocalKEK, error) {
	derived, err := DeriveKey(appKey, kekPurpose)
	if err != nil {
		return nil, err
	}

	cipher, err := subtle.NewAESGCM(derived)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	return &LocalKEK{cipher: cipher}, nil
}

/*
Wrap encrypts the key material.
*/
func (k *LocalKEK) Wrap(_ context.Context, plaintext, associatedData []byte) ([]byte, error) {
	wrapped, err := k.cipher.Encrypt(plaintext, associatedData)
	if err != nil {
		return nil, fmt.Errorf("wrap key: %w", err)
	}

	return wrapped, nil
}

/*
Unwrap decrypts key material, or returns ErrDecrypt.
*/
func (k *LocalKEK) Unwrap(_ context.Context, wrapped, associatedData []byte) ([]byte, error) {
	plaintext, err := k.cipher.Decrypt(wrapped, associatedData)
	if err != nil {
		return nil, ErrDecrypt
	}

	return plaintext, nil
}

/*
tinkKEK presents a KEK as the context-aware AEAD Tink wraps keysets
with.
*/
type tinkKEK struct {
	kek KEK
}

/*
EncryptWithContext wraps with the KEK.
*/
func (t tinkKEK) EncryptWithContext(ctx context.Context, plaintext, associatedData []byte) ([]byte, error) {
	return t.kek.Wrap(ctx, plaintext, associatedData)
}

/*
DecryptWithContext unwraps with the KEK.
*/
func (t tinkKEK) DecryptWithContext(ctx context.Context, ciphertext, associatedData []byte) ([]byte, error) {
	return t.kek.Unwrap(ctx, ciphertext, associatedData)
}
