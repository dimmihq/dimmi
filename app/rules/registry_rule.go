/*
|--------------------------------------------------------------------------
| Registry Rule
|--------------------------------------------------------------------------
|
| A registry column only checks the shape of its values in the database,
| so the application must refuse values nobody registered before they
| reach a row. Requests name the registry in the tag, as in
| `validate:"registered=auth_methods"`.
|
*/

package rules

import (
	"context"

	"github.com/dimmihq/dimmi/framework/validation"

	"github.com/dimmihq/dimmi/app/auth"
)

/*
Registered accepts only values of the registry its parameter names. An
unknown registry accepts nothing.
*/
type Registered struct {
	Registries map[string]auth.Registry
}

var _ validation.Rule = Registered{}

/*
Name returns the tag that applies the rule.
*/
func (r Registered) Name() string {
	return "registered"
}

/*
Message returns the English message.
*/
func (r Registered) Message() string {
	return "The selected {field} is invalid."
}

/*
Passes reports whether the value is registered in the named registry.
*/
func (r Registered) Passes(_ context.Context, value any, param string) bool {
	text, ok := value.(string)
	registry, known := r.Registries[param]

	return ok && known && registry.Has(text)
}
