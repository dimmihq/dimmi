/*
|--------------------------------------------------------------------------
| Authenticate Widget Session
|--------------------------------------------------------------------------
|
| Requests a visitor makes after starting a session carry its bearer token
| and nothing else: no app id, no cookie. The session decides the widget and
| the workspace, which go into the request's context for the origin check and
| every query after it. A token in a query string is never read, so a token
| never lands in an access log.
|
*/

package middleware

import (
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/widget"
)

/*
AuthenticateWidget returns the middleware that admits requests with a live
visitor session.
*/
func AuthenticateWidget(auth *widget.AuthenticateWidgetSession, errs fhttp.Errors) fhttp.Middleware {
	return admitSessions(auth, widget.AuthenticateWidgetSessionInput{}, errs)
}

/*
AuthenticateWidgetReviving is [AuthenticateWidget] that also admits an
expired identified session, for the route that revives it with a fresh
identity token.
*/
func AuthenticateWidgetReviving(auth *widget.AuthenticateWidgetSession, errs fhttp.Errors) fhttp.Middleware {
	return admitSessions(auth, widget.AuthenticateWidgetSessionInput{AdmitExpired: true}, errs)
}

/*
admitSessions authenticates with the input given, its token filled in
from the request.
*/
func admitSessions(
	auth *widget.AuthenticateWidgetSession, in widget.AuthenticateWidgetSessionInput, errs fhttp.Errors,
) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")

			asked := in
			asked.Token = strings.TrimSpace(token)

			found, err := auth.Handle(r.Context(), asked)
			if err != nil {
				errs.Render(w, r, err)

				return
			}

			ctx := widget.WithSession(widget.WithResolved(r.Context(), found.Resolved), found.Session)

			next.ServeHTTP(w, r.WithContext(tenancy.With(ctx, found.Resolved.Workspace.ID.String())))
		})
	}
}
