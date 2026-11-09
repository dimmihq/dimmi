/*
|--------------------------------------------------------------------------
| Verify Widget Origin
|--------------------------------------------------------------------------
|
| The last gate before a messenger route: is this site allowed, is the
| widget on, and is its workspace in good standing. Every request is noted
| for install status on the way through, the config request a page always
| makes included, so a widget shows as installed with no extra request. That
| note is a write to memory, because this path has a budget of milliseconds.
|
*/

package middleware

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/support/widgetinstall"
	"github.com/dimmihq/dimmi/app/support/widgetorigin"
)

/*
InstallRecorder notes which sites a widget is seen on.
*/
type InstallRecorder interface {
	/*
		Record notes one request of a site, allowed or not. A blank origin
		is a client without one.
	*/
	Record(v widgetinstall.Visit)
}

/*
HelpSites recognizes the sites of a workspace's own help centers.
*/
type HelpSites interface {
	/*
		Owns reports whether the origin is one of the workspace's help
		centers.
	*/
	Owns(ctx context.Context, workspaceID ids.UUID, origin string) bool
}

/*
VerifyWidgetOrigin returns the middleware that stops requests a widget does
not accept. Install is this install's own origin, which is always allowed,
because the hosted page and the preview run there, and so are the
workspace's own help centers, which show its messenger.
*/
func VerifyWidgetOrigin(install string, tracker InstallRecorder, help HelpSites, errs fhttp.Errors) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			resolved, ok := widget.CurrentResolved(r.Context())
			if !ok {
				errs.Render(w, r, exceptions.ErrAppNotFound)

				return
			}

			asking := r.Header.Get("Origin")
			allowed := widgetorigin.OriginAllowed(
				resolved.Widget.AllowedOrigins, resolved.Widget.NativeAppsEnabled, asking, install,
			) || (asking != "" && help.Owns(r.Context(), resolved.Workspace.ID, asking))

			tracker.Record(widgetinstall.Visit{
				WorkspaceID: resolved.Workspace.ID, WidgetID: resolved.Widget.ID, Origin: asking, Allowed: allowed,
			})

			if err := refusal(resolved, allowed); err != nil {
				errs.Render(w, r, err)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

/*
refusal returns why the widget does not serve this request, nil when it
does.
*/
func refusal(resolved widget.Resolved, allowed bool) error {
	switch {
	case !allowed:
		return exceptions.ErrOriginNotAllowed
	case !resolved.Widget.Enabled:
		return exceptions.ErrWidgetDisabled
	case resolved.Workspace.SuspendedAt != nil:
		return exceptions.ErrWorkspaceSuspended
	}

	return nil
}
