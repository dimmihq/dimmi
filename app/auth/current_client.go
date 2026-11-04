/*
|--------------------------------------------------------------------------
| Current Client
|--------------------------------------------------------------------------
|
| Work that records who did something, such as the audit log, runs deep
| inside actions that know nothing of HTTP. The request being answered
| travels in the context, so they read the client and the request id
| from here, the way they read the caller; work started outside a
| request finds neither.
|
*/

package auth

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
CurrentClient returns the client of the request in the context, and
false outside a request. It carries no device token.
*/
func CurrentClient(ctx context.Context) (Client, bool) {
	r, ok := fhttp.RequestFrom(ctx)
	if !ok {
		return Client{}, false
	}

	return Client{IP: fhttp.ClientIP(r), UserAgent: r.UserAgent()}, true
}

/*
CurrentRequestID returns the id of the request in the context, or ""
outside a request.
*/
func CurrentRequestID(ctx context.Context) string {
	return fhttp.RequestID(ctx)
}
