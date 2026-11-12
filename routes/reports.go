/*
|--------------------------------------------------------------------------
| Report Routes
|--------------------------------------------------------------------------
|
| Reports read conversations, so they need conversations:read and count
| only what the caller may see; the help center report also needs
| articles:read. Each teammate manages their own digest.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
reportRoutes declares the reports API.
*/
func reportRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Reports == nil {
		return
	}

	c := &v1.ReportController{Reports: a.Actions.Reports, Digests: a.Actions.ReportDigests, IDs: a.IDs}

	routing.Get(workspace, "/reports", "reports.index", c.Index).Ability("conversations:read")
	routing.Get(workspace, "/reports/{report}", "reports.show", c.Show).Ability("conversations:read")
	routing.Get(workspace, "/reports/{report}/conversations", "reports.conversations.index", c.Conversations).
		Ability("conversations:read")
	routing.Get(workspace, "/report-digest", "report-digest.show", c.ShowDigest).Ability("conversations:read")
	routing.Put(workspace, "/report-digest", "report-digest.update", c.UpdateDigest).Ability("conversations:read")
	routing.Delete(workspace, "/report-digest", "report-digest.destroy", c.DestroyDigest).
		Ability("conversations:read").Status(fhttp.StatusNoContent)
}
