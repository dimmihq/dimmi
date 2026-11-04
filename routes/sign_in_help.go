/*
|--------------------------------------------------------------------------
| Sign-In Help Routes
|--------------------------------------------------------------------------
|
| An admin helping a teammate who cannot sign in. Only the admin
| themselves, by browser or app, ever does it, never a token, and inside
| the workspace they share with the teammate. Mailing a reset link also
| needs a recent proof of who is asking. The teammate finishes the reset
| signed out: the link's token is the proof.
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
signInHelpRoutes declares the API routes that help a teammate back in.
*/
func signInHelpRoutes(api *routing.Router, a *app.App) {
	acts := a.Actions
	help := &v1.MemberSignInHelpController{
		Start: acts.StartTwoFactorReset, Complete: acts.CompleteTwoFactorReset, Clear: acts.ClearSignInLock,
		IDs: a.IDs, Cookies: a.Cookies,
	}

	api.Group("", func(guest *routing.Router) {
		guest.Middleware("csrf", "session")

		routing.Patch(guest, "/two-factor-resets/{reset}", "two-factor-resets.update", help.UpdateReset)
	})

	api.Group("", func(admin *routing.Router) {
		admin.Middleware("csrf", "session", "auth:user", "workspace", "access.policy", "api.throttle")

		routing.Post(admin, "/members/{member}/two-factor-resets", "members.two-factor-resets.store",
			help.StoreReset).Middleware("password.confirm").Status(fhttp.StatusCreated)
		routing.Delete(admin, "/members/{member}/sign-in-lock", "members.sign-in-lock.destroy", help.DestroyLock).
			Status(fhttp.StatusNoContent)
	})
}
