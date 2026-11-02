/*
|--------------------------------------------------------------------------
| Router
|--------------------------------------------------------------------------
|
| Routes are declared in groups that share a path prefix and middleware,
| and every route has a name, so the rest of the application refers to a
| route by what it is rather than by where it lives. Declaring a route
| only records it; the table is built once when the handler is first
| asked for, so group middleware applies to every route in the group no
| matter the order of the calls, and a mistake such as an unknown
| middleware name or a duplicate name stops the boot with an error.
|
*/

package routing

import (
	"cmp"
	"errors"
	"fmt"
	stdhttp "net/http"
	"slices"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
Errors the router returns when the table is built.

ErrDuplicateName means two routes share a name. ErrNoAPI means a typed
route was declared outside an API group, where it has no document to
belong to. ErrInvalidRoute wraps a route the underlying router or API
layer refused, such as a path parameter the input type does not declare.
*/
var (
	ErrDuplicateName = errors.New("routing: duplicate route name")
	ErrNoAPI         = errors.New("routing: typed route outside an api group")
	ErrInvalidRoute  = errors.New("routing: invalid route")
)

/*
Resolver builds middleware from the names routes ask for. The HTTP
kernel is one.
*/
type Resolver interface {
	/*
		Resolve returns the middleware the entries name, in order.
	*/
	Resolve(entries ...string) ([]http.Middleware, error)
}

/*
Options configure a router.

Middleware resolves the names routes and groups ask for; without it any
name is an error. Errors renders failures. Validator checks the body of
every typed request against its tags before the handler runs; without
it only the input's own Validate hook runs. NotFound answers paths no
route matches; without it the client gets the JSON 404.
*/
type Options struct {
	Middleware Resolver
	Errors     http.Errors
	Validator  *validation.Factory
	NotFound   http.Handler
}

/*
Router declares routes under a path prefix, with the middleware of its
group and of every group around it.
*/
type Router struct {
	table      *table
	parent     *Router
	prefix     string
	api        *apiSpec
	middleware []string
	uses       []http.Middleware
}

/*
table is the state every group of one router shares.
*/
type table struct {
	opts    Options
	routes  []*Route
	names   map[string]*Route
	apis    []*apiSpec
	once    sync.Once
	handler http.Handler
	err     error
}

/*
New returns an empty router.
*/
func New(opts Options) *Router {
	return &Router{table: &table{opts: opts, names: map[string]*Route{}}}
}

/*
Group declares routes under the prefix inside the function. Middleware
added to the group in the function applies to all of them.
*/
func (r *Router) Group(prefix string, fn func(g *Router)) {
	fn(r.child(prefix, r.api))
}

/*
API declares the typed routes of one API document inside the function.
Its paths are documented relative to the prefix, and the document is
served at the prefix under /openapi.json.
*/
func (r *Router) API(prefix, title, version string, fn func(g *Router)) {
	spec := &apiSpec{prefix: r.prefix + prefix, title: title, version: version}
	r.table.apis = append(r.table.apis, spec)
	fn(r.child(prefix, spec))
}

/*
Middleware adds middleware by name to every route of the group.
*/
func (r *Router) Middleware(names ...string) {
	r.middleware = append(r.middleware, names...)
}

/*
Use adds middleware to every route of the group.
*/
func (r *Router) Use(mw ...http.Middleware) {
	r.uses = append(r.uses, mw...)
}

/*
Handle declares a route answered by a plain handler, such as a page, a
webhook or a file download.
*/
func (r *Router) Handle(method, path, name string, h http.Handler) *Route {
	route := r.add(method, path, name)
	route.handler = h

	return route
}

/*
Routes returns every declared route, ordered by path and then method.
*/
func (r *Router) Routes() []*Route {
	routes := slices.Clone(r.table.routes)
	slices.SortStableFunc(routes, func(a, b *Route) int {
		return cmp.Or(strings.Compare(a.path, b.path), strings.Compare(a.method, b.method))
	})

	return routes
}

/*
Handler builds the route table on first use and returns the handler that
serves it. Routes declared after the first call are not served.
*/
func (r *Router) Handler() (http.Handler, error) {
	t := r.table
	t.once.Do(func() { t.handler, t.err = t.build() })

	return t.handler, t.err
}

func (r *Router) child(prefix string, spec *apiSpec) *Router {
	return &Router{table: r.table, parent: r, prefix: r.prefix + prefix, api: spec}
}

func (r *Router) add(method, path, name string) *Route {
	route := &Route{group: r, method: method, path: r.prefix + path, name: name, api: r.api}
	r.table.routes = append(r.table.routes, route)

	if _, taken := r.table.names[name]; !taken && name != "" {
		r.table.names[name] = route
	}

	return route
}

/*
build registers every route on a fresh router and API documents.
*/
func (t *table) build() (http.Handler, error) {
	if err := t.checkNames(); err != nil {
		return nil, err
	}

	mux := chi.NewMux()
	mux.NotFound(t.notFound)
	mux.MethodNotAllowed(methodNotAllowed(mux, t.opts.Errors))

	routes := map[string]*Route{}
	for _, spec := range t.apis {
		spec.api = newAPI(spec, &adapter{mux: mux, prefix: spec.prefix, routes: routes})
	}

	for _, route := range t.routes {
		chain, err := t.chain(route)
		if err != nil {
			return nil, err
		}

		route.chain = chain
		if route.handler == nil {
			routes[route.name] = route
		}

		if err := t.register(mux, route); err != nil {
			return nil, err
		}
	}

	return mux, nil
}

func (t *table) checkNames() error {
	seen := map[string]bool{}

	for _, route := range t.routes {
		if route.name == "" {
			continue
		}

		if seen[route.name] {
			return fmt.Errorf("route %q: %w", route.name, ErrDuplicateName)
		}

		seen[route.name] = true
	}

	return nil
}

/*
chain resolves the route's middleware: the route context first, so every
middleware can see which route matched, then each group's from the
outermost in, then the route's own.
*/
func (t *table) chain(route *Route) ([]http.Middleware, error) {
	var groups []*Router
	for g := route.group; g != nil; g = g.parent {
		groups = append(groups, g)
	}

	chain := []http.Middleware{withRoute(route)}

	for _, g := range slices.Backward(groups) {
		mw, err := t.resolve(route, g.middleware)
		if err != nil {
			return nil, err
		}

		chain = append(append(chain, mw...), g.uses...)
	}

	mw, err := t.resolve(route, route.middleware)
	if err != nil {
		return nil, err
	}

	return append(append(chain, mw...), route.uses...), nil
}

func (t *table) resolve(route *Route, names []string) ([]http.Middleware, error) {
	if len(names) == 0 {
		return nil, nil
	}

	if t.opts.Middleware == nil {
		return nil, fmt.Errorf("route %s %s: %w: %v", route.method, route.path, http.ErrUnknownMiddleware, names)
	}

	mw, err := t.opts.Middleware.Resolve(names...)
	if err != nil {
		return nil, fmt.Errorf("route %s %s: %w", route.method, route.path, err)
	}

	return mw, nil
}

/*
register adds one route. The router and the API layer panic on routes
they cannot serve; that panic becomes ErrInvalidRoute so the boot fails
with the route's name instead of a stack trace.
*/
func (t *table) register(mux *chi.Mux, route *Route) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("route %s %s: %w: %v", route.method, route.path, ErrInvalidRoute, p)
		}
	}()

	if route.handler != nil {
		mux.With(route.chain...).Method(route.method, route.path, route.handler)

		return nil
	}

	if route.api == nil {
		return fmt.Errorf("route %q: %w", route.name, ErrNoAPI)
	}

	route.register(route.api.api, route.operation(), t.opts)

	return nil
}

func (t *table) notFound(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if t.opts.NotFound != nil {
		t.opts.NotFound.ServeHTTP(w, r)

		return
	}

	t.opts.Errors.Render(w, r, http.StatusError(stdhttp.StatusNotFound))
}

/*
methodNotAllowed answers with the methods the path does accept in the
Allow header, which the router does not pass on to a custom handler.
*/
func methodNotAllowed(mux *chi.Mux, errs http.Errors) stdhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		for _, method := range methods() {
			if mux.Match(chi.NewRouteContext(), method, r.URL.Path) {
				w.Header().Add("Allow", method)
			}
		}

		errs.Render(w, r, http.StatusError(stdhttp.StatusMethodNotAllowed))
	}
}

func methods() []string {
	return []string{
		stdhttp.MethodGet, stdhttp.MethodHead, stdhttp.MethodPost, stdhttp.MethodPut,
		stdhttp.MethodPatch, stdhttp.MethodDelete, stdhttp.MethodOptions,
	}
}
