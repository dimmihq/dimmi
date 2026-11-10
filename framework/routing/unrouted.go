/*
|--------------------------------------------------------------------------
| Serving A Request Within A Request
|--------------------------------------------------------------------------
|
| A handler may serve a second request through the application's own
| routes, such as an MCP tool making the API call it stands for. That
| request must be routed from scratch: if it carried the first request's
| routing state, matched route or current request, it would be matched
| and checked as the first one. Unrouted clears exactly those, keeping
| the rest of the context, such as its cancellation and tracing.
|
*/

package routing

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/dimmihq/dimmi/framework/http"
)

/*
Unrouted returns the context with no routing state, matched route or
current request, for serving another request within a request.
*/
func Unrouted(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, chi.RouteCtxKey, (*chi.Context)(nil))
	ctx = context.WithValue(ctx, routeKey{}, (*Route)(nil))

	return http.WithRequest(ctx, nil)
}
