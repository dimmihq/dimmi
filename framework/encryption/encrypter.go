/*
|--------------------------------------------------------------------------
| App Key Encrypter
|--------------------------------------------------------------------------
|
| The encrypter seals small values that never live in the database:
| cookies, signed state in links, values handed to the browser and back.
| It uses AES-256-GCM under a key derived from the app key, so a value
| that was tampered with fails to decrypt instead of decrypting to
| garbage.
|
| When the app key is rotated, the previous keys keep decrypting what
| they encrypted, while everything new is encrypted with the current one.
|
*/

package encryption

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/tink-crypto/tink-go/v2/aead/subtle"
)

/*
ErrDecrypt is returned when a ciphertext was not made by these keys,
was changed, or is bound to other associated data.
*/
var ErrDecrypt = errors.New("encryption: cannot decrypt")

/*
encrypterPurpose names the app key derivation for the encrypter.
*/
const encrypterPurpose = "encrypter"

/*
Encrypter encrypts values with the app key.
*/
type Encrypter interface {
	/*
		Encrypt returns the plaintext encrypted and authenticated.
	*/
	Encrypt(plaintext []byte) ([]byte, error)

	/*
		Decrypt returns the plaintext, or ErrDecrypt.
	*/
	Decrypt(ciphertext []byte) ([]byte, error)

	/*
		EncryptString returns the value encrypted as unpadded base64url,
		safe in URLs and cookies.
	*/
	EncryptString(value string) (string, error)

	/*
		DecryptString reverses EncryptString.
	*/
	DecryptString(ciphertext string) (string, error)
}

/*
AppEncrypter is the Encrypter on the app key.
*/
type AppEncrypter struct {
	ciphers []*subtle.AESGCM
}

var _ Encrypter = (*AppEncrypter)(nil)

/*
NewEncrypter returns an encrypter on the app key. Previous app keys only
decrypt.
*/
func NewEncrypter(appKey []byte, previous ...[]byte) (*AppEncrypter, error) {
	keys := append([][]byte{appKey}, previous...)
	ciphers := make([]*subtle.AESGCM, 0, len(keys))

	for _, key := range keys {
		derived, err := DeriveKey(key, encrypterPurpose)
		if err != nil {
			return nil, err
		}

		cipher, err := subtle.NewAESGCM(derived)
		if err != nil {
			return nil, fmt.Errorf("create cipher: %w", err)
		}

		ciphers = append(ciphers, cipher)
	}

	return &AppEncrypter{ciphers: ciphers}, nil
}

/*
Encrypt returns the plaintext encrypted with the current app key.
*/
func (e *AppEncrypter) Encrypt(plaintext []byte) ([]byte, error) {
	ciphertext, err := e.ciphers[0].Encrypt(plaintext, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	return ciphertext, nil
}

/*
Decrypt tries the current app key, then each previous one.
*/
func (e *AppEncrypter) Decrypt(ciphertext []byte) ([]byte, error) {
	for _, cipher := range e.ciphers {
		if plaintext, err := cipher.Decrypt(ciphertext, nil); err == nil {
			return plaintext, nil
		}
	}

	return nil, ErrDecrypt
}

/*
EncryptString returns the value encrypted as unpadded base64url.
*/
func (e *AppEncrypter) EncryptString(value string) (string, error) {
	ciphertext, err := e.Encrypt([]byte(value))
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

/*
DecryptString reverses EncryptString.
*/
func (e *AppEncrypter) DecryptString(ciphertext string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", ErrDecrypt
	}

	plaintext, err := e.Decrypt(raw)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
