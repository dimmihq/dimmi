/*
|--------------------------------------------------------------------------
| Import Routes
|--------------------------------------------------------------------------
|
| The API of imports of customers and companies from a CSV: upload,
| look, dry run, start, follow, download the failed rows and undo. Every
| route takes contacts:write; the actions also check imports:manage, or
| imports:rollback to undo.
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
importRoutes declares the imports API.
*/
func importRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Inbox == nil || a.Actions.Inbox.Imports == nil {
		return
	}

	c := &v1.ImportController{Imports: a.Actions.Inbox.Imports, IDs: a.IDs}
	one := "/imports/{import}"

	routing.Post(workspace, "/imports", "imports.store", c.Store).
		Ability("contacts:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, "/imports", "imports.index", c.Index).Ability("contacts:write")
	routing.Get(workspace, one, "imports.show", c.Get).Ability("contacts:write")
	routing.Post(workspace, one+"/previews", "imports.previews.store", c.Preview).
		Ability("contacts:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/runs", "imports.runs.store", c.Run).
		Ability("contacts:write").Status(fhttp.StatusAccepted)
	routing.Get(workspace, one+"/errors", "imports.errors.show", c.Errors).Ability("contacts:write")
	routing.Post(workspace, one+"/rollbacks", "imports.rollbacks.store", c.Rollback).
		Ability("contacts:write").Status(fhttp.StatusAccepted)
}
