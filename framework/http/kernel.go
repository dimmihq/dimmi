/*
|--------------------------------------------------------------------------
| HTTP Kernel
|--------------------------------------------------------------------------
|
| Routes ask for middleware by name, such as "auth" or "throttle:api", so
| a route file reads like a policy and the wiring lives in one place. The
| kernel owns that vocabulary: aliases that build a middleware from its
| parameters, and groups that name a list of other entries. It also wraps
| every request in the global middleware, outermost first.
|
| The kernel never listens on a socket. It hands back a handler, and the
| process that owns the server decides how and where to serve it.
|
*/

package http

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

/*
Middleware wraps a handler with work done before or after it.
*/
type Middleware = func(Handler) Handler

/*
MiddlewareFactory builds a middleware from the parameters written after
its alias, so "throttle:api,strict" calls the factory registered as
"throttle" with "api" and "strict".
*/
type MiddlewareFactory func(params ...string) (Middleware, error)

/*
Errors the kernel returns while resolving names.

ErrUnknownMiddleware means a name is neither an alias nor a group.
ErrMiddlewareCycle means a group lists itself, directly or through
another group.
*/
var (
	ErrUnknownMiddleware = errors.New("http: unknown middleware")
	ErrMiddlewareCycle   = errors.New("http: middleware group lists itself")
)

/*
Kernel holds the global middleware and the names routes use for the rest.
*/
type Kernel struct {
	global  []Middleware
	aliases map[string]MiddlewareFactory
	groups  map[string][]string
}

/*
NewKernel returns a kernel with no middleware and no names.
*/
func NewKernel() *Kernel {
	return &Kernel{aliases: map[string]MiddlewareFactory{}, groups: map[string][]string{}}
}

/*
Use adds middleware that wraps every request, the first added outermost.
*/
func (k *Kernel) Use(mw ...Middleware) {
	k.global = append(k.global, mw...)
}

/*
Alias registers the factory routes reach by name. A later alias with the
same name replaces the earlier one.
*/
func (k *Kernel) Alias(name string, factory MiddlewareFactory) {
	k.aliases[name] = factory
}

/*
Group names a list of entries, each an alias with its parameters or
another group, so "api" can stand for "throttle:api" and "bindings".
*/
func (k *Kernel) Group(name string, entries ...string) {
	k.groups[name] = slices.Clone(entries)
}

/*
Resolve builds the middleware the entries name, in order, expanding
groups in place.
*/
func (k *Kernel) Resolve(entries ...string) ([]Middleware, error) {
	return k.expand(entries, nil)
}

/*
Handler wraps the handler in the global middleware.
*/
func (k *Kernel) Handler(next Handler) Handler {
	return Chain(next, k.global...)
}

/*
expand resolves entries, carrying the groups being expanded so a group
that reaches itself is refused instead of recursing forever.
*/
func (k *Kernel) expand(entries, expanding []string) ([]Middleware, error) {
	var out []Middleware

	for _, entry := range entries {
		mw, err := k.entry(entry, expanding)
		if err != nil {
			return nil, err
		}

		out = append(out, mw...)
	}

	return out, nil
}

func (k *Kernel) entry(entry string, expanding []string) ([]Middleware, error) {
	name, rest, hasParams := strings.Cut(entry, ":")

	if members, ok := k.groups[name]; ok && !hasParams {
		if slices.Contains(expanding, name) {
			return nil, fmt.Errorf("resolve %q: %w", name, ErrMiddlewareCycle)
		}

		return k.expand(members, append(slices.Clone(expanding), name))
	}

	factory, ok := k.aliases[name]
	if !ok {
		return nil, fmt.Errorf("resolve %q: %w", entry, ErrUnknownMiddleware)
	}

	var params []string
	if hasParams {
		params = strings.Split(rest, ",")
	}

	mw, err := factory(params...)
	if err != nil {
		return nil, fmt.Errorf("build middleware %q: %w", entry, err)
	}

	return []Middleware{mw}, nil
}

/*
Chain wraps the handler in the middleware, the first listed outermost.
*/
func Chain(h Handler, mw ...Middleware) Handler {
	for _, m := range slices.Backward(mw) {
		h = m(h)
	}

	return h
}
