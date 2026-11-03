/*
|--------------------------------------------------------------------------
| Slug Rule
|--------------------------------------------------------------------------
|
| A slug becomes part of an address, so it is held to the plainest shape
| that reads well in one: lowercase ASCII letters and digits in words
| joined by single dashes. Length is left to the request's own limits.
| Requests apply it as `validate:"slug"`.
|
*/

package rules

import (
	"context"
	"regexp"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
Slug accepts lowercase words of letters and digits joined by single
dashes, such as acme-support.
*/
type Slug struct{}

var _ validation.Rule = Slug{}

/*
Name returns the tag that applies the rule.
*/
func (Slug) Name() string {
	return "slug"
}

/*
Message returns the English message.
*/
func (Slug) Message() string {
	return "The {field} may only contain lowercase letters, numbers and single dashes."
}

/*
Passes reports whether the value has the shape of a slug.
*/
func (Slug) Passes(_ context.Context, value any, _ string) bool {
	text, ok := value.(string)

	return ok && regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(text)
}
