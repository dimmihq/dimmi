/*
|--------------------------------------------------------------------------
| Webhook Secrets
|--------------------------------------------------------------------------
|
| A webhook secret is shared with one receiver and signs every request
| sent to it. Secrets are written the Standard Webhooks way, "whsec_"
| followed by standard base64, so every official verification library
| accepts them unchanged. A Secret prints redacted: only Encode gives the
| real value, for storing it encrypted and showing it once to its owner.
|
*/

package webhooks

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

/*
ErrInvalidSecret is returned for a secret that is not "whsec_" followed
by standard base64 of 24 to 64 bytes.
*/
var ErrInvalidSecret = errors.New("webhooks: invalid secret")

/*
Secret lengths in bytes: new secrets use the longest the receivers'
libraries expect, parsing accepts the whole range the standard allows.
*/
const (
	secretPrefix  = "whsec_"
	newSecretSize = 32
	minSecretSize = 24
	maxSecretSize = 64
)

/*
Secret is the key that signs webhooks for one receiver.
*/
type Secret struct {
	key []byte
}

/*
NewSecret returns a new random secret. The system random source never
fails, so there is no error to return.
*/
func NewSecret() Secret {
	key := make([]byte, newSecretSize)
	_, _ = rand.Read(key)

	return Secret{key: key}
}

/*
ParseSecret reads a secret written by Encode.
*/
func ParseSecret(s string) (Secret, error) {
	encoded, ok := strings.CutPrefix(s, secretPrefix)
	if !ok {
		return Secret{}, ErrInvalidSecret
	}

	key, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(key) < minSecretSize || len(key) > maxSecretSize {
		return Secret{}, ErrInvalidSecret
	}

	return Secret{key: key}, nil
}

/*
Encode returns the secret as "whsec_" followed by standard base64.
*/
func (s Secret) Encode() string {
	return secretPrefix + base64.StdEncoding.EncodeToString(s.key)
}

/*
String returns the secret redacted, so logging it leaks nothing.
*/
func (s Secret) String() string {
	return secretPrefix + "[redacted]"
}
