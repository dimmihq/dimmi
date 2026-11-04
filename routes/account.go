/*
|--------------------------------------------------------------------------
| Account Routes
|--------------------------------------------------------------------------
|
| A person's own account. Anyone holding a valid credential may read the
| profile; changing anything takes the person themselves, by browser or
| app, never a token. Moving to a new address and deleting the account
| also need a recent proof of who is asking. The link that confirms a new
| address works signed out: the token it carries is the proof.
|
| What a workspace requires is never checked here, so a person shut out of
| a workspace can still manage themselves.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/users"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
)

/*
avatarUploadOverhead is room for the multipart framing around a picture
at the size limit, so a picture just under it is judged by its own bytes.
*/
const avatarUploadOverhead = 16 << 10

/*
Account declares the pages that serve pictures.
*/
func Account(a *app.App) {
	avatars := &web.AvatarController{Disks: a.Storage}

	a.Router.Handle(fhttp.MethodGet, "/avatars/{file}", "avatars.show", fhttp.HandlerFunc(avatars.Show))
}

/*
accountRoutes declares the API routes of the person's own account.
*/
func accountRoutes(api *routing.Router, a *app.App) {
	acts := a.Actions
	profile := &v1.AccountController{
		UpdateProfile: acts.UpdateProfile, UpdatePassword: acts.UpdatePassword, DeleteAccount: acts.DeleteAccount,
		Cookies: a.Cookies,
	}
	avatar := &v1.UserAvatarController{UpdateAvatar: acts.UpdateAvatar, DeleteAvatar: acts.DeleteAvatar}
	changes := &v1.EmailChangeController{
		Request: acts.RequestEmailChange, Confirm: acts.ConfirmEmailChange, Cookies: a.Cookies,
	}
	preview := &v1.UserDeletionPreviewController{Preview: acts.PreviewAccountDeletion}

	api.Group("", func(guest *routing.Router) {
		guest.Middleware("csrf", "session")

		routing.Patch(guest, "/email-changes/{change}", "email-changes.update", changes.Update)
	})

	api.Group("", func(caller *routing.Router) {
		caller.Middleware("csrf", "session", "auth")

		routing.Get(caller, "/user", "user.show", profile.Show)
	})

	api.Group("", func(user *routing.Router) {
		user.Middleware("csrf", "session", "auth:user")

		routing.Patch(user, "/user", "user.update", profile.Update)
		routing.Delete(user, "/user", "user.destroy", profile.Destroy).
			Middleware("password.confirm").Status(fhttp.StatusNoContent)
		routing.Get(user, "/user/deletion-preview", "user.deletion-preview.show", preview.Show)
		routing.Patch(user, "/user/password", "user.password.update", profile.ChangePassword)
		routing.Put(user, "/user/avatar", "user.avatar.update", avatar.Update).
			Use(middleware.BodyLimit(users.AvatarMaxBytes + avatarUploadOverhead))
		routing.Delete(user, "/user/avatar", "user.avatar.destroy", avatar.Destroy).Status(fhttp.StatusNoContent)
		routing.Post(user, "/email-changes", "email-changes.store", changes.Store).
			Middleware("password.confirm").Status(fhttp.StatusCreated)
	})
}
