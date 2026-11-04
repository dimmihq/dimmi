/*
|--------------------------------------------------------------------------
| Member Admin Routes
|--------------------------------------------------------------------------
|
| An admin helping a teammate who cannot sign in, or changing many
| members at once from a spreadsheet. Only the admin themselves, by
| browser or app, ever does it, never a token, and inside the workspace
| they share with the teammates. Mailing a reset link and importing
| members also need a recent proof of who is asking. The teammate
| finishes a reset signed out: the link's token is the proof.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
memberImportMaxBytes is the largest member import upload: a 2 MB file
and room for the multipart framing around it.
*/
const memberImportMaxBytes = 2<<20 + 16<<10

/*
signInHelpRoutes declares the API routes that help a teammate back in.
*/
func signInHelpRoutes(api *routing.Router, a *app.App) {
	acts := a.Actions
	help := &v1.MemberSignInHelpController{
		Start: acts.StartTwoFactorReset, Complete: acts.CompleteTwoFactorReset, Clear: acts.ClearSignInLock,
		IDs: a.IDs, Cookies: a.Cookies,
	}

	imports := &v1.MemberImportController{Import: acts.ImportMembers}

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
		routing.Post(admin, "/member-imports", "member-imports.store", imports.Store).
			Middleware("password.confirm").Status(fhttp.StatusOK).Use(middleware.BodyLimit(memberImportMaxBytes))
	})
}
