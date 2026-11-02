/*
|--------------------------------------------------------------------------
| Password Rules
|--------------------------------------------------------------------------
|
| A password is judged by what makes it hard to guess: its length, and
| whether it is one of the passwords people use most, which attackers
| try first. Length counts characters as people type them, so a
| password in another script is not punished for its bytes. Composition
| rules such as "one digit, one symbol" are deliberately absent: they
| make passwords harder to remember without making them harder to guess.
|
*/

package rules

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
PasswordLength requires at least the configured number of characters.
It is applied as `validate:"password_length"`.
*/
type PasswordLength struct {
	Min int
}

var _ validation.Rule = PasswordLength{}

/*
Name returns the tag that applies the rule.
*/
func (r PasswordLength) Name() string {
	return "password_length"
}

/*
Message returns the English message, naming the minimum.
*/
func (r PasswordLength) Message() string {
	return fmt.Sprintf("The {field} must be at least %d characters.", r.Min)
}

/*
Passes reports whether the password is long enough.
*/
func (r PasswordLength) Passes(_ context.Context, value any, _ string) bool {
	password, ok := value.(string)

	return ok && utf8.RuneCountInString(password) >= r.Min
}

/*
UncommonPassword refuses the passwords people use most. It is applied as
`validate:"password"`. Common holds the refused passwords in lower case.
*/
type UncommonPassword struct {
	Common map[string]struct{}
}

var _ validation.Rule = UncommonPassword{}

/*
Name returns the tag that applies the rule.
*/
func (r UncommonPassword) Name() string {
	return "password"
}

/*
Message returns the English message.
*/
func (r UncommonPassword) Message() string {
	return "The {field} is too common. Choose another one."
}

/*
Passes reports whether the password is not on the list, ignoring case.
*/
func (r UncommonPassword) Passes(_ context.Context, value any, _ string) bool {
	password, ok := value.(string)

	_, common := r.Common[strings.ToLower(password)]

	return ok && !common
}
