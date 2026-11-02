/*
|--------------------------------------------------------------------------
| Scopes and PKCE
|--------------------------------------------------------------------------
|
| Scopes only ever narrow: a request asks for at most what its client
| registered, a user approves at most what was asked, and a refreshed
| token carries at most what the grant holds. PKCE ties a code to the app
| that started the flow: only the holder of the verifier whose SHA-256
| is the challenge can redeem it, so a code intercepted on its way back
| is useless. The plain method would send the verifier itself through
| the browser, so only S256 is accepted.
|
*/

package oauth

import (
	"encoding/base64"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
The PKCE method the server accepts and the length bounds of a verifier
and a challenge.
*/
const (
	methodS256   = "S256"
	minPKCEChars = 43
	maxPKCEChars = 128
)

/*
unreservedChars are the characters a PKCE verifier or challenge may use.
*/
const unreservedChars = base62 + "-._~"

/*
parseScopes splits a scope parameter into its scopes, once each, in the
order given.
*/
func parseScopes(raw string) []string {
	var scopes []string

	for scope := range strings.FieldsSeq(raw) {
		if !slices.Contains(scopes, scope) {
			scopes = append(scopes, scope)
		}
	}

	return scopes
}

/*
subset reports whether every item of want is in have.
*/
func subset(want, have []string) bool {
	for _, item := range want {
		if !slices.Contains(have, item) {
			return false
		}
	}

	return true
}

/*
pkceValue reports whether the value is a well-formed verifier or
challenge: 43 to 128 unreserved characters.
*/
func pkceValue(value string) bool {
	return len(value) >= minPKCEChars && len(value) <= maxPKCEChars && strings.Trim(value, unreservedChars) == ""
}

/*
verifyPKCE reports whether the verifier hashes to the S256 challenge,
comparing in constant time.
*/
func verifyPKCE(verifier, challenge string) bool {
	if !pkceValue(verifier) {
		return false
	}

	sum := base64.RawURLEncoding.EncodeToString(hashing.SHA256([]byte(verifier)))

	return hashing.Equal([]byte(sum), []byte(challenge))
}
