/*
|--------------------------------------------------------------------------
| Host Names
|--------------------------------------------------------------------------
|
| A host typed with letters outside ASCII, such as bücher.example, is
| stored and dialed in its ASCII form, so two spellings of one host
| compare equal and every resolver understands it.
|
*/

package httpclient

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/idna"
)

/*
ErrInvalidHost refuses a host name that has no valid ASCII form.
*/
var ErrInvalidHost = errors.New("httpclient: invalid host name")

/*
ASCIIHost returns the host's lowercase ASCII form, punycode for
international names. IP literals and ASCII names come back lowercased.
*/
func ASCIIHost(host string) (string, error) {
	ascii, err := idna.Lookup.ToASCII(strings.TrimSuffix(host, "."))
	if err != nil || ascii == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidHost, host)
	}

	return strings.ToLower(ascii), nil
}
