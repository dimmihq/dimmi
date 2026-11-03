/*
|--------------------------------------------------------------------------
| Time Zone And Locale Rules
|--------------------------------------------------------------------------
|
| A workspace's time zone and language are what members and customers
| fall back to, so only values the server can actually use are stored:
| a time zone of the IANA database, and a language the server has
| translations for. Requests apply them as
| `validate:"timezone"` and `validate:"locale"`.
|
*/

package rules

import (
	"context"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
Timezone accepts names of the IANA time zone database, such as
Europe/Lisbon.
*/
type Timezone struct{}

var _ validation.Rule = Timezone{}

/*
Name returns the tag that applies the rule.
*/
func (Timezone) Name() string {
	return "timezone"
}

/*
Message returns the English message.
*/
func (Timezone) Message() string {
	return "The {field} must be a valid time zone."
}

/*
Passes reports whether the value names a time zone. Local, which means
whatever the server runs in, is not one.
*/
func (Timezone) Passes(_ context.Context, value any, _ string) bool {
	name, ok := value.(string)
	if !ok || name == "" || name == "Local" {
		return false
	}

	_, err := time.LoadLocation(name)

	return err == nil
}

/*
Locale accepts the languages the server has translations for.
*/
type Locale struct {
	Supported []string
}

var _ validation.Rule = Locale{}

/*
Name returns the tag that applies the rule.
*/
func (Locale) Name() string {
	return "locale"
}

/*
Message returns the English message.
*/
func (Locale) Message() string {
	return "The selected {field} is not a supported language."
}

/*
Passes reports whether the value is a supported language.
*/
func (l Locale) Passes(_ context.Context, value any, _ string) bool {
	name, ok := value.(string)

	return ok && slices.Contains(l.Supported, name)
}
