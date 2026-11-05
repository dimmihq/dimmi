/*
|--------------------------------------------------------------------------
| Onboarding Routes
|--------------------------------------------------------------------------
|
| A new workspace's setup steps: recording how far setup has come, which
| changes the workspace, and starting the practice conversations, which
| is working the inbox.
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
onboardingRoutes declares the setup routes of the workspace in play.
*/
func onboardingRoutes(r *routing.Router, a *app.App) {
	r.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

	c := &v1.OnboardingController{
		Save: a.Actions.UpdateOnboarding, TestConversations: a.Actions.Inbox.CreateTestConversations,
	}

	routing.Patch(r, "/workspace/onboarding", "workspace.onboarding.update", c.Update).Ability("settings:write")
	routing.Post(r, "/workspace/onboarding/test-conversations", "workspace.onboarding.test-conversations.store",
		c.StoreTestConversations).Ability("conversations:write").Status(fhttp.StatusCreated)
}
