/*
|--------------------------------------------------------------------------
| Attribute Routes
|--------------------------------------------------------------------------
|
| The API of attribute definitions. Reading takes the contacts:read
| ability; changing takes attributes:write, and the actions check the
| member may manage attributes.
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
attributeRoutes declares the attribute definitions API.
*/
func attributeRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Attributes == nil {
		return
	}

	ac := &v1.AttributeController{Definitions: a.Actions.Attributes, IDs: a.IDs}
	one := "/attribute-definitions/{attribute}"

	routing.Get(workspace, "/attribute-definitions", "attribute-definitions.index", ac.Index).Ability("contacts:read")
	routing.Post(workspace, "/attribute-definitions", "attribute-definitions.store", ac.Store).
		Ability("attributes:write").Status(fhttp.StatusCreated)
	routing.Patch(workspace, one, "attribute-definitions.update", ac.Update).Ability("attributes:write")
	routing.Delete(workspace, one, "attribute-definitions.destroy", ac.Destroy).
		Ability("attributes:write").Status(fhttp.StatusNoContent)
	routing.Get(workspace, "/attribute-problems", "attribute-problems.index", ac.Problems).Ability("contacts:read")
}
