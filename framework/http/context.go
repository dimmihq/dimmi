/*
|--------------------------------------------------------------------------
| Request Context
|--------------------------------------------------------------------------
|
| Middleware learns things about a request that handlers further in need:
| who is calling, from which address, under which request id. Each fact is
| stored once in the request context by the middleware that owns it, and
| read back through these functions, so no layer depends on another's
| internals. Guards store the principal; everything else reads it.
|
*/

package http

import (
	"context"
	"net"
	"net/http"
)

type (
	principalKey struct{}
	clientIPKey  struct{}
	requestIDKey struct{}
	requestKey   struct{}
)

/*
WithPrincipal returns a copy of the context carrying the authenticated
caller, as stored by a guard.
*/
func WithPrincipal(ctx context.Context, principal any) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

/*
Principal returns the authenticated caller in the context when there is
one of the asked type.
*/
func Principal[T any](ctx context.Context) (T, bool) {
	principal, ok := ctx.Value(principalKey{}).(T)

	return principal, ok
}

/*
WithClientIP returns a copy of the context carrying the client's address
as worked out from trusted proxy headers.
*/
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

/*
ClientIP returns the client's address: the one worked out from trusted
proxies when that ran, otherwise the address of the connection.
*/
func ClientIP(r *Request) string {
	if ip, ok := r.Context().Value(clientIPKey{}).(string); ok && ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

/*
WithRequestID returns a copy of the context carrying the request id.
*/
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

/*
RequestID returns the request id in the context, or "" when there is
none.
*/
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)

	return id
}

/*
WithRequest returns a copy of the context carrying the request, for
typed handlers that receive only a context and their input.
*/
func WithRequest(ctx context.Context, r *Request) context.Context {
	return context.WithValue(ctx, requestKey{}, r)
}

/*
RequestFrom returns the request carried in the context, such as the one
a typed handler is answering.
*/
func RequestFrom(ctx context.Context) (*http.Request, bool) {
	r, ok := ctx.Value(requestKey{}).(*http.Request)

	return r, ok && r != nil
}
