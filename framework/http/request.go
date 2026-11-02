/*
|--------------------------------------------------------------------------
| Request Hooks
|--------------------------------------------------------------------------
|
| An input type can carry its own checks, the way a form request does:
| whether the caller may make this request at all, and rules that tags
| cannot express, such as one field required only when another is set.
| The router runs them before the handler, authorization first, so a
| caller who may not act never learns which fields were wrong.
|
*/

package http

import (
	"context"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
Authorizer is an input that decides whether the caller may make the
request. Returning a fault such as Forbidden or NotFound stops it.
*/
type Authorizer interface {
	/*
		Authorize returns nil when the request may go ahead.
	*/
	Authorize(ctx context.Context) error
}

/*
Validatable is an input with rules beyond its tags. It runs after the
tag rules pass and may use the factory to check parts of itself.
*/
type Validatable interface {
	/*
		Validate returns nil, a validation error or a fault.
	*/
	Validate(ctx context.Context, v *validation.Factory) error
}
