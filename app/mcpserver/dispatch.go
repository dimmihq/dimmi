/*
|--------------------------------------------------------------------------
| Calling The API
|--------------------------------------------------------------------------
|
| A tool does not reach into the application on its own: it makes the
| very request the REST API would receive, with the caller's own token,
| and hands it to the same handler. Authentication, the workspace's
| rules, permissions, rate limits, validation and the audit trail all
| apply exactly as they do to the API, so an agent can never do more, or
| differently, than the person who connected it could with a token.
|
*/

package mcpserver

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
apiPrefix is where the API the tools call is served.
*/
const apiPrefix = "/api/v1"

/*
forwarded lists the headers of the agent's request every call carries:
its credential, its language and where it comes from.
*/
func forwarded() []string {
	return []string{
		"Authorization", "Accept-Language", "User-Agent", "X-Forwarded-For", "X-Forwarded-Proto", "X-Real-Ip",
	}
}

/*
originKey is the context key under which the agent's request travels.
*/
type originKey struct{}

/*
withOrigin keeps the parts of the agent's request each call repeats.
*/
func withOrigin(ctx context.Context, r *fhttp.Request) context.Context {
	return context.WithValue(ctx, originKey{}, mcp.OriginOf(r, forwarded()...))
}

/*
call makes the API request for an operation with the given arguments.
*/
func call(ctx context.Context, api fhttp.Handler, op mcp.Operation, args map[string]any) (mcp.Response, error) {
	from, _ := ctx.Value(originKey{}).(mcp.Origin)

	return op.Call(ctx, mcp.API{Handler: api, Prefix: apiPrefix}, args, from)
}
