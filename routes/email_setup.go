/*
|--------------------------------------------------------------------------
| Email Setup Routes
|--------------------------------------------------------------------------
|
| Setting up forwarding is proven by a test: one is sent and then read
| back while it waits for its mail. Both take email:manage, which the
| action checks; the routes check the token abilities.
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
emailSetupRoutes declares the forwarding tests of the workspace's
addresses.
*/
func emailSetupRoutes(workspace *routing.Router, a *app.App) {
	c := &v1.EmailAddressTestController{
		Send: a.Actions.SendEmailAddressTest, Show: a.Actions.ShowEmailAddressTest, IDs: a.IDs,
	}

	routing.Post(workspace, "/email-addresses/{address}/tests", "email-addresses.tests.store", c.Store).
		Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, "/email-addresses/{address}/tests/{test}", "email-addresses.tests.show", c.ShowTest).
		Ability("settings:write")
}
