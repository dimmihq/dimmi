/*
|--------------------------------------------------------------------------
| Digests
|--------------------------------------------------------------------------
|
| Fast hashes for values that are already random or need no stretching:
| API token lookups, cursor signatures, cache keys. They are never used
| for passwords. Every comparison of secrets goes through Equal, so its
| timing does not reveal how many leading bytes matched.
|
*/

package hashing

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

/*
SHA256 returns the SHA-256 digest of the data.
*/
func SHA256(data []byte) []byte {
	sum := sha256.Sum256(data)

	return sum[:]
}

/*
SHA256Hex returns the SHA-256 digest of the data as lowercase hex.
*/
func SHA256Hex(data []byte) string {
	return hex.EncodeToString(SHA256(data))
}

/*
HMAC returns the HMAC-SHA256 of the message under the key.
*/
func HMAC(key, message []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)

	return mac.Sum(nil)
}

/*
Equal reports whether a and b are equal in time that depends only on
their lengths.
*/
func Equal(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
