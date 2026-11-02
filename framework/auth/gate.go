/*
|--------------------------------------------------------------------------
| Gate
|--------------------------------------------------------------------------
|
| Authorization asks for permissions, never for roles. Which permissions
| a caller holds is the application's business, worked out by its
| resolver from whatever it keeps: roles, plans, a token's abilities.
| The framework only asks whether the permission is among them.
|
| Some permissions also depend on the thing being acted on: anyone may
| edit records, but only their own. Rules registered for a permission and
| a resource type refine the answer, and run only for callers who hold
| the permission in the first place.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
)

/*
ErrForbidden is the fault for a caller who may not do what they asked.
*/
var ErrForbidden = fault.New(fault.Forbidden, "forbidden", "This action is unauthorized.")

/*
ErrNoPermission is returned when a route asks for the permission
middleware without naming a permission.
*/
var ErrNoPermission = errors.New("auth: no permission named")

/*
PermissionResolver works out the permissions a caller holds.
*/
type PermissionResolver interface {
	/*
		Permissions returns every permission the principal holds.
	*/
	Permissions(ctx context.Context, p Principal) ([]string, error)
}

/*
PermissionsFunc adapts a function to a PermissionResolver.
*/
type PermissionsFunc func(ctx context.Context, p Principal) ([]string, error)

/*
Permissions calls the function.
*/
func (f PermissionsFunc) Permissions(ctx context.Context, p Principal) ([]string, error) {
	return f(ctx, p)
}

/*
Gate answers whether a caller may do something.
*/
type Gate struct {
	resolver PermissionResolver
	rules    map[ruleKey]Rule
	errors   http.Errors
}

/*
NewGate returns a gate on the application's permission resolver, whose
middleware renders refusals with the errors renderer.
*/
func NewGate(resolver PermissionResolver, errs http.Errors) *Gate {
	return &Gate{resolver: resolver, rules: map[ruleKey]Rule{}, errors: errs}
}

/*
Define registers the rule for the permission on any resource without a
policy of its own.
*/
func (g *Gate) Define(permission string, rule Rule) {
	g.rules[ruleKey{permission: permission}] = rule
}

/*
Can reports whether the principal may use the permission on the
resource, which may be nil. The principal must hold the permission, and
the rule for the resource's type, or else the permission's own rule,
must agree when there is one.
*/
func (g *Gate) Can(ctx context.Context, p Principal, permission string, resource any) (bool, error) {
	held, err := g.resolver.Permissions(ctx, p)
	if err != nil {
		return false, fmt.Errorf("resolve permissions: %w", err)
	}

	if !slices.Contains(held, permission) {
		return false, nil
	}

	rule, ok := g.rules[ruleKey{permission: permission, resource: reflect.TypeOf(resource)}]
	if !ok {
		rule, ok = g.rules[ruleKey{permission: permission}]
	}

	if !ok {
		return true, nil
	}

	return rule(ctx, p, resource)
}

/*
Authorize returns nil when the caller in the context may use the
permission on the resource, ErrUnauthenticated when there is no caller,
and ErrForbidden or the rule's own fault when they may not.
*/
func (g *Gate) Authorize(ctx context.Context, permission string, resource any) error {
	p, ok := From(ctx)
	if !ok {
		return ErrUnauthenticated
	}

	allowed, err := g.Can(ctx, p, permission, resource)
	if err != nil {
		return err
	}

	if !allowed {
		return ErrForbidden
	}

	return nil
}

/*
Middleware builds the route middleware that requires every permission
in the parameters, registered as a kernel alias such as "can", so a
route reads "can:records.update".
*/
func (g *Gate) Middleware(params ...string) (http.Middleware, error) {
	if len(params) == 0 {
		return nil, ErrNoPermission
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, permission := range params {
				if err := g.Authorize(r.Context(), permission, nil); err != nil {
					g.errors.Render(w, r, err)

					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}, nil
}
