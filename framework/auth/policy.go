/*
|--------------------------------------------------------------------------
| Policies
|--------------------------------------------------------------------------
|
| A permission says what a caller may do in general; a policy says
| whether they may do it to this particular thing, such as editing only
| records they own. Policies are plain typed functions registered on the
| gate for a permission and a resource type, so a check reads like the
| rule it enforces and is tested like any other function.
|
*/

package auth

import (
	"context"
	"reflect"
)

/*
Rule decides whether a caller who holds a permission may use it on the
resource. It may return a fault, such as NotFound to hide that the
resource exists.
*/
type Rule func(ctx context.Context, p Principal, resource any) (bool, error)

type ruleKey struct {
	permission string
	resource   reflect.Type
}

/*
Policy decides whether a caller who holds a permission may use it on a
resource of type T.
*/
type Policy[T any] func(ctx context.Context, p Principal, resource T) (bool, error)

/*
RegisterPolicy registers the policy for the permission on resources of
type T.
*/
func RegisterPolicy[T any](g *Gate, permission string, rule Policy[T]) {
	key := ruleKey{permission: permission, resource: reflect.TypeFor[T]()}
	g.rules[key] = func(ctx context.Context, p Principal, resource any) (bool, error) {
		typed, _ := resource.(T)

		return rule(ctx, p, typed)
	}
}
