/*
|--------------------------------------------------------------------------
| Contact Fields
|--------------------------------------------------------------------------
|
| What the inbox stores about a customer arrives from teammates, the API
| and channels, so it is normalized the same way everywhere: emails in
| lower case without surrounding space, phone numbers as digits with an
| optional leading plus.
|
*/

package contacts

import (
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/mail"
)

/*
NormalizeEmail returns the address in lower case without surrounding
space, nil when empty.
*/
func NormalizeEmail(email *string) *string {
	if email == nil {
		return nil
	}

	e := mail.CanonicalAddress(*email)
	if e == "" {
		return nil
	}

	return &e
}

/*
NormalizePhone returns the number as its digits, keeping a leading
plus, nil when it has none.
*/
func NormalizePhone(phone *string) *string {
	if phone == nil {
		return nil
	}

	p := strings.TrimSpace(*phone)
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}

		return -1
	}, p)

	if digits == "" {
		return nil
	}

	if strings.HasPrefix(p, "+") {
		digits = "+" + digits
	}

	return &digits
}
