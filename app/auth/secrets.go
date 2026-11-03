/*
|--------------------------------------------------------------------------
| Mailed Secrets
|--------------------------------------------------------------------------
|
| Links and codes sent by mail prove that the reader owns the inbox, so
| they are guarded like passwords. A link carries a long random token of
| which only the SHA-256 is stored; a stolen table yields no working
| link. A six-digit code is short enough to type, so its digest is keyed
| with a secret derived from the app key and bound to the row it belongs
| to: a stolen table cannot be searched for codes, and a code never
| works for another row.
|
*/

package auth

import (
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
The sizes of mailed secrets: a token of 43 letters and digits carries
more than 250 bits, a code has six digits.
*/
const (
	secretLength = 43
	codeDigits   = 6
)

/*
CodeKeyPurpose names the key derived from the app key that seals codes.
*/
const CodeKeyPurpose = "mailed-codes"

/*
NewSecret returns a fresh random token and the digest to store for it.
*/
func NewSecret() (string, []byte) {
	plain := str.Random(secretLength)

	return plain, Digest(plain)
}

/*
Digest returns the stored form of a token.
*/
func Digest(plain string) []byte {
	return hashing.SHA256([]byte(plain))
}

/*
Codes makes and checks six-digit codes with the key derived for them.
*/
type Codes struct {
	Key []byte
}

/*
New returns a fresh random code.
*/
func (c Codes) New() string {
	return str.RandomDigits(codeDigits)
}

/*
Seal returns the stored form of the code of the row with the id.
*/
func (c Codes) Seal(id ids.UUID, code string) []byte {
	return hashing.HMAC(c.Key, []byte(id.String()+":"+NormalizeCode(code)))
}

/*
Matches reports, in constant time, whether the code is the one sealed
for the row with the id.
*/
func (c Codes) Matches(id ids.UUID, code string, sealed []byte) bool {
	return hashing.Equal(c.Seal(id, code), sealed)
}

/*
NormalizeCode keeps only the digits of a typed code, so "123-456" and
"123 456" both read as the code mailed as 123-456.
*/
func NormalizeCode(code string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}

		return -1
	}, code)
}

/*
FormatCode writes a code made by New the way mail shows it, in two
halves.
*/
func FormatCode(code string) string {
	return code[:codeDigits/2] + "-" + code[codeDigits/2:]
}
