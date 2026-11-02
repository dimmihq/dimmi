/*
|--------------------------------------------------------------------------
| Secrets
|--------------------------------------------------------------------------
|
| Codes, tokens and client secrets share one shape: the application's
| prefix, a tag naming the kind of secret, and 43 random letters and
| digits, about 256 bits. The tag lets people and secret scanners tell a
| leaked refresh token from an access token at a glance, and lets the
| server turn away a value of the wrong kind without a lookup. Only a
| SHA-256 digest is stored, which is enough for a value this random:
| there is nothing to guess from it.
|
*/

package oauth

import (
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
The tags after the prefix, one per kind of secret, and the tag of the
client ids the server makes up.
*/
const (
	kindCode         = "oac_"
	kindAccess       = "oat_"
	kindRefresh      = "ort_"
	kindClientSecret = "ocs_"
	kindClientID     = "oci_"
)

/*
The random lengths of a secret and of a client id after their tags.
*/
const (
	secretLength   = 43
	clientIDLength = 26
)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

/*
newSecret returns a fresh secret of the kind.
*/
func (s *Server) newSecret(kind string) string {
	return s.opts.Prefix + kind + str.Random(secretLength)
}

/*
wellFormed reports whether the value has the prefix and tag of the
kind followed by the right number of base62 characters.
*/
func (s *Server) wellFormed(plain, kind string) bool {
	rest, ok := strings.CutPrefix(plain, s.opts.Prefix+kind)
	if !ok || len(rest) != secretLength {
		return false
	}

	return strings.Trim(rest, base62) == ""
}

func digest(plain string) []byte {
	return hashing.SHA256([]byte(plain))
}
