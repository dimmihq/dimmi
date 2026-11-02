/*
|--------------------------------------------------------------------------
| App Key
|--------------------------------------------------------------------------
|
| The app key is the one secret every installation must keep. It is
| written "base64:" followed by 32 random bytes, and nothing uses it
| directly: each purpose derives its own key from it with HKDF, so the
| key that encrypts cookies can never decrypt a wrapped data key.
|
*/

package encryption

import (
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

/*
ErrInvalidAppKey is returned for an app key that is not "base64:"
followed by standard base64 of 32 bytes.
*/
var ErrInvalidAppKey = errors.New("encryption: app key must be base64: followed by 32 bytes")

/*
appKeyPrefix marks an encoded app key; appKeySize is its length in
bytes.
*/
const (
	appKeyPrefix = "base64:"
	appKeySize   = 32
)

/*
GenerateAppKey returns a new random app key, encoded. The system random
source never fails, so there is no error to return.
*/
func GenerateAppKey() string {
	key := make([]byte, appKeySize)
	_, _ = rand.Read(key)

	return appKeyPrefix + base64.StdEncoding.EncodeToString(key)
}

/*
ParseAppKey decodes an app key written by GenerateAppKey.
*/
func ParseAppKey(s string) ([]byte, error) {
	encoded, ok := strings.CutPrefix(s, appKeyPrefix)
	if !ok {
		return nil, ErrInvalidAppKey
	}

	key, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(key) != appKeySize {
		return nil, ErrInvalidAppKey
	}

	return key, nil
}

/*
DeriveKey returns the 32-byte key for one purpose, derived from the app
key with HKDF-SHA256. Different purposes give unrelated keys.
*/
func DeriveKey(appKey []byte, purpose string) ([]byte, error) {
	if len(appKey) != appKeySize {
		return nil, ErrInvalidAppKey
	}

	return hkdf.Key(sha256.New, appKey, nil, purpose, appKeySize)
}
