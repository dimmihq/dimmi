/*
|--------------------------------------------------------------------------
| Resolve Workspace
|--------------------------------------------------------------------------
|
| Workspace routes run inside the workspace the request works in. Once
| the caller is known, this middleware decides that workspace, refuses
| callers who may not work in it, and scopes the rest of the request to
| it: database transactions see only its rows, and controllers read the
| caller's membership and permissions from the context.
|
*/

package middleware

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
)

/*
WorkspaceHeader is the header in which a browser or app names the
workspace it works in.
*/
const WorkspaceHeader = "Dimmi-Workspace"

/*
ResolveWorkspace returns the middleware that scopes a request to its
workspace, rendering the refusal of a caller who may not work there.
*/
func ResolveWorkspace(resolve *workspaces.ResolveWorkspace, errs fhttp.Errors) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			p, _ := appauth.CurrentPrincipal(r.Context())

			membership, err := resolve.Handle(r.Context(), workspaces.ResolveWorkspaceInput{
				Principal: p, Named: r.Header.Get(WorkspaceHeader), Writes: writes(r.Method),
			})
			if err != nil {
				errs.Render(w, r, err)

				return
			}

			ctx := tenancy.With(authz.WithMembership(r.Context(), membership), membership.Workspace.ID.String())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

/*
writes reports whether a request with the method may change data.
*/
func writes(method string) bool {
	return method != fhttp.MethodGet && method != fhttp.MethodHead && method != fhttp.MethodOptions
}
