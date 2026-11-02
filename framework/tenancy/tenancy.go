/*
|--------------------------------------------------------------------------
| Tenant Context
|--------------------------------------------------------------------------
|
| Every request that touches customer data belongs to exactly one tenant.
| Middleware resolves it once and stores it in the request context; the
| database layer reads it back so row-level security can keep tenants
| apart even when a query forgets to filter.
|
*/

package tenancy

import (
	"context"
	"errors"
)

/*
ErrMissing is returned by Required when the context carries no tenant.
*/
var ErrMissing = errors.New("tenancy: no tenant in context")

type contextKey struct{}

/*
With returns a copy of the context scoped to the given tenant.
*/
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

/*
From returns the tenant in the context and reports whether there is one.

An empty id counts as no tenant, so a half-built context never scopes
queries to the empty string.
*/
func From(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKey{}).(string)

	return id, ok && id != ""
}

/*
Required returns the tenant in the context, or ErrMissing.
*/
func Required(ctx context.Context) (string, error) {
	id, ok := From(ctx)
	if !ok {
		return "", ErrMissing
	}

	return id, nil
}
