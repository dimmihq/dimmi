/*
|--------------------------------------------------------------------------
| Routes
|--------------------------------------------------------------------------
|
| A typed route takes its input as a struct and returns its output as a
| struct, so the binding of path, query, header and body, the schema
| checks and the OpenAPI document all come from the same Go types. The
| handler sees only a context and its input, and returns a fault when the
| request cannot be served; turning that into a response is not its job.
|
| Each route remembers its name, middleware and required abilities, so a
| guard can read them from the request and a command can list them.
|
*/

package routing

import (
	"context"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"github.com/dimmihq/dimmi/framework/http"
)

/*
Endpoint answers a typed request. The input's path, query and header
fields come from their tags and its Body field from the request body;
the output's Body field becomes the response body and its other tagged
fields become headers.
*/
type Endpoint[I, O any] func(ctx context.Context, in *I) (*O, error)

/*
Route is one declared route. Its setters return the route so options
chain after the declaration.
*/
type Route struct {
	group      *Router
	api        *apiSpec
	method     string
	path       string
	name       string
	middleware []string
	uses       []http.Middleware
	abilities  []string
	summary    string
	tags       []string
	status     int
	handler    http.Handler
	register   func(api huma.API, op huma.Operation, opts Options)
	chain      []http.Middleware
}

type routeKey struct{}

/*
Get declares a typed GET route.
*/
func Get[I, O any](g *Router, path, name string, h Endpoint[I, O]) *Route {
	return typed(g.add(http.MethodGet, path, name), h)
}

/*
Post declares a typed POST route. It answers 201 unless told otherwise.
*/
func Post[I, O any](g *Router, path, name string, h Endpoint[I, O]) *Route {
	return typed(g.add(http.MethodPost, path, name), h).Status(http.StatusCreated)
}

/*
Patch declares a typed PATCH route.
*/
func Patch[I, O any](g *Router, path, name string, h Endpoint[I, O]) *Route {
	return typed(g.add(http.MethodPatch, path, name), h)
}

/*
Put declares a typed PUT route, which replaces a singleton's whole
content.
*/
func Put[I, O any](g *Router, path, name string, h Endpoint[I, O]) *Route {
	return typed(g.add(http.MethodPut, path, name), h)
}

/*
Delete declares a typed DELETE route.
*/
func Delete[I, O any](g *Router, path, name string, h Endpoint[I, O]) *Route {
	return typed(g.add(http.MethodDelete, path, name), h)
}

/*
CurrentRoute returns the route that matched the request in the context.
*/
func CurrentRoute(ctx context.Context) (*Route, bool) {
	route, ok := ctx.Value(routeKey{}).(*Route)

	return route, ok
}

/*
Middleware adds middleware by name to this route, after its groups'.
*/
func (r *Route) Middleware(names ...string) *Route {
	r.middleware = append(r.middleware, names...)

	return r
}

/*
Use adds middleware to this route, after its groups'.
*/
func (r *Route) Use(mw ...http.Middleware) *Route {
	r.uses = append(r.uses, mw...)

	return r
}

/*
Ability records an ability a token must hold to call this route. Guards
read it from the matched route; the route itself enforces nothing.
*/
func (r *Route) Ability(abilities ...string) *Route {
	r.abilities = append(r.abilities, abilities...)

	return r
}

/*
Summary sets the one-line description in the API document.
*/
func (r *Route) Summary(summary string) *Route {
	r.summary = summary

	return r
}

/*
Tags sets the groups the route is listed under in the API document.
*/
func (r *Route) Tags(tags ...string) *Route {
	r.tags = append(r.tags, tags...)

	return r
}

/*
Status sets the status a typed route answers with on success, such as
202 for work that finishes later.
*/
func (r *Route) Status(status int) *Route {
	r.status = status

	return r
}

/*
Method returns the route's request method.
*/
func (r *Route) Method() string {
	return r.method
}

/*
Path returns the route's full path pattern.
*/
func (r *Route) Path() string {
	return r.path
}

/*
Name returns the route's name.
*/
func (r *Route) Name() string {
	return r.name
}

/*
MiddlewareNames returns the middleware names of the route's groups,
outermost first, followed by its own.
*/
func (r *Route) MiddlewareNames() []string {
	var names []string
	for g := r.group; g != nil; g = g.parent {
		names = append(slices.Clone(g.middleware), names...)
	}

	return append(names, r.middleware...)
}

/*
Abilities returns the abilities the route requires.
*/
func (r *Route) Abilities() []string {
	return slices.Clone(r.abilities)
}

/*
operation describes the route to the API layer, with paths relative to
the API's prefix and the error envelope as the documented error body.
*/
func (r *Route) operation() huma.Operation {
	op := huma.Operation{
		OperationID:   r.name,
		Method:        r.method,
		Path:          r.path[len(r.api.prefix):],
		Summary:       r.summary,
		Tags:          r.tags,
		DefaultStatus: r.status,
		Responses:     map[string]*huma.Response{"default": errorResponse(r.api.api)},
	}

	if op.Path == "" {
		op.Path = "/"
	}

	if len(r.abilities) > 0 {
		op.Extensions = map[string]any{"x-abilities": r.Abilities()}
	}

	return op
}

func typed[I, O any](route *Route, h Endpoint[I, O]) *Route {
	route.register = func(api huma.API, op huma.Operation, opts Options) {
		huma.Register(api, op, func(ctx context.Context, in *I) (*O, error) {
			if err := check(ctx, in, opts.Validator); err != nil {
				return nil, opts.Errors.Resolve(ctx, err)
			}

			out, err := h(ctx, in)
			if err != nil {
				return nil, opts.Errors.Resolve(ctx, err)
			}

			return out, nil
		})
	}

	return route
}

/*
withRoute puts the matched route and the request in the context, for
the middleware after it and for typed handlers.
*/
func withRoute(route *Route) http.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), routeKey{}, route)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r.WithContext(http.WithRequest(ctx, r)))
		})
	}
}
