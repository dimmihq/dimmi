/*
|--------------------------------------------------------------------------
| Resolve Widget App
|--------------------------------------------------------------------------
|
| A public messenger route names its widget, in the path or, for starting a
| session, in the body. A missing widget, a malformed id and a deleted one
| answer the same, so app ids cannot be probed. The rest of the request runs
| inside the widget's workspace.
|
*/

package middleware

import (
	"bytes"
	"encoding/json"
	"io"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxWidgetBody is the most a messenger JSON body may hold.
*/
const maxWidgetBody = 1 << 20

/*
ResolveWidgetApp returns the middleware that finds the widget a request
names.
*/
func ResolveWidgetApp(directory *widget.Directory, registry *ids.Registry, errs fhttp.Errors) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			public := r.PathValue("app")
			if public == "" {
				public = bodyAppID(w, r)
			}

			id, err := registry.Decode(string(models.PrefixWidget), public)
			if err != nil {
				errs.Render(w, r, exceptions.ErrAppNotFound)

				return
			}

			resolved, err := directory.Find(r.Context(), id)
			if err != nil {
				errs.Render(w, r, err)

				return
			}

			ctx := tenancy.With(widget.WithResolved(r.Context(), resolved), resolved.Workspace.ID.String())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

/*
bodyAppID peeks at the JSON body's app_id and leaves the body as it was for
the route. A body too large to read ends up refused by the route.
*/
func bodyAppID(w fhttp.ResponseWriter, r *fhttp.Request) string {
	body, err := io.ReadAll(fhttp.MaxBytesReader(w, r.Body, maxWidgetBody))
	if err != nil {
		return ""
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	var named struct {
		AppID string `json:"app_id"`
	}

	if err := json.Unmarshal(body, &named); err != nil {
		return ""
	}

	return named.AppID
}
