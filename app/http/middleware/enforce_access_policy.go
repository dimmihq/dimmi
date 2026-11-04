/*
|--------------------------------------------------------------------------
| Enforce Access Policy
|--------------------------------------------------------------------------
|
| Right after a request is scoped to its workspace, the workspace's own
| requirements are checked for the caller, whatever credential they
| used. A caller who does not meet one is refused with what is missing
| and where to fix it, before any controller runs.
|
*/

package middleware

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
EnforceAccessPolicy returns the middleware that refuses callers who do
not meet the requirements of the workspace in play. It runs after
ResolveWorkspace; without a caller or a workspace it refuses, since a
route that skipped them must not pass for one that checked.
*/
func EnforceAccessPolicy(policy appauth.AccessPolicy, errs fhttp.Errors) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			p, ok := appauth.CurrentPrincipal(r.Context())
			membership, scoped := authz.CurrentMembership(r.Context())

			if !ok || !scoped {
				errs.Render(w, r, exceptions.ErrForbidden)

				return
			}

			if err := policy.Evaluate(r.Context(), p, membership.Workspace); err != nil {
				errs.Render(w, r, err)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
