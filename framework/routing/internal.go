/*
|--------------------------------------------------------------------------
| Internal Routes
|--------------------------------------------------------------------------
|
| Some API routes exist for the product's own client rather than for
| integrators: signing in, a person's own account, previews a settings
| screen draws, live screen state. They stay in the API, the contract
| and every SDK, served and checked like any other route, but the public
| API reference leaves them out. The document marks each one with
| x-internal: true, and the docs site skips what carries it.
|
*/

package routing

import "github.com/danielgtaylor/huma/v2"

/*
Internal marks the route as built for the product's own client, so the public API
reference leaves it out.
*/
func (r *Route) Internal() *Route {
	r.internal = true

	return r
}

/*
Internal marks every route of the group as built for the product's own client.
*/
func (r *Router) Internal() {
	r.internal = true
}

/*
IsInternal reports whether the route, or any group around it, is marked
internal.
*/
func (r *Route) IsInternal() bool {
	if r.internal {
		return true
	}

	for g := r.group; g != nil; g = g.parent {
		if g.internal {
			return true
		}
	}

	return false
}

/*
markInternal flags an internal route's operation in the API document.
*/
func (r *Route) markInternal(op *huma.Operation) {
	if !r.IsInternal() {
		return
	}

	if op.Extensions == nil {
		op.Extensions = map[string]any{}
	}

	op.Extensions["x-internal"] = true
}
