/*
|--------------------------------------------------------------------------
| Widget Session Tokens
|--------------------------------------------------------------------------
|
| A visitor's bearer token is random and shown once, when the session
| starts; the install keeps only its hash, so a leaked table names no
| working token. The prefix lets anyone recognize a messenger token in a
| log or a secret scanner, and lets the server refuse anything else before
| touching the database.
|
*/

package widget

import (
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
SessionTokenPrefix starts every visitor session token.
*/
const SessionTokenPrefix = "dmi_ws_"

/*
sessionTokenLength is how many random characters follow the prefix.
*/
const sessionTokenLength = 43

/*
NewSessionToken returns a fresh token and the hash the install stores.
*/
func NewSessionToken() (token string, hash []byte) {
	token = SessionTokenPrefix + str.Random(sessionTokenLength)

	return token, HashSessionToken(token)
}

/*
HashSessionToken returns the stored hash of a token.
*/
func HashSessionToken(token string) []byte {
	return hashing.SHA256([]byte(token))
}

/*
WellFormedSessionToken reports whether the text could be a session token,
so anything else is refused without a lookup.
*/
func WellFormedSessionToken(token string) bool {
	rest, ok := strings.CutPrefix(token, SessionTokenPrefix)

	return ok && len(rest) == sessionTokenLength
}
