/*
|--------------------------------------------------------------------------
| Address Forms
|--------------------------------------------------------------------------
|
| One mailbox can be typed many ways: composed or decomposed accents,
| fullwidth letters, the long s, a domain in Unicode or in its ASCII
| form, upper or lower case. Stored and compared in a single form, each
| of them names one account. Invisible and control characters never
| belong in an address, so an address carrying them is refused rather
| than quietly made into another one.
|
*/

package mail

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

/*
NormalizeAddress returns an email address in the one form it is stored
and compared in: compatibility-normalized, lowercase, its domain in
ASCII. ErrInvalidAddress refuses an address with invisible or control
characters, without a local part, or whose domain is no valid name.
*/
func NormalizeAddress(raw string) (string, error) {
	address := norm.NFKC.String(strings.TrimSpace(raw))
	if strings.ContainsFunc(address, hidden) {
		return "", fmt.Errorf("%w: invisible characters", ErrInvalidAddress)
	}

	at := strings.LastIndex(address, "@")
	if at < 1 || at == len(address)-1 {
		return "", fmt.Errorf("%w: %q", ErrInvalidAddress, address)
	}

	domain, err := idna.Lookup.ToASCII(address[at+1:])
	if err != nil {
		return "", fmt.Errorf("%w: domain: %w", ErrInvalidAddress, err)
	}

	return strings.ToLower(address[:at]) + "@" + strings.ToLower(domain), nil
}

/*
CanonicalAddress returns the address in its normal form, or trimmed and
lowercase when it has none, for looking up what was stored before.
*/
func CanonicalAddress(raw string) string {
	address, err := NormalizeAddress(raw)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(raw))
	}

	return address
}

/*
hidden reports whether a character cannot be seen or typed as part of
an address, such as a zero-width space or a control character.
*/
func hidden(r rune) bool {
	return unicode.Is(unicode.Cf, r) || unicode.IsControl(r) || unicode.IsSpace(r)
}
