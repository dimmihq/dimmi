/*
|--------------------------------------------------------------------------
| API Routes
|--------------------------------------------------------------------------
|
| The public REST API under /api/v1. Routes are grouped by who may call
| them, and every group that a browser can reach with its cookie runs
| cross-origin protection on unsafe requests, signing in and signing up
| included, so no other site can sign a browser into an account it
| chose.
|
|   guest      anyone; an existing session is allowed and ignored
|   user       the user themselves, by browser or by their own app
|   caller     every valid credential, access tokens included
|   workspace  every valid credential, inside the workspace the request
|              works in: named by a browser or app, a token's own
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
controllers are the API's controllers, built once with the actions
they call.
*/
type controllers struct {
	authConfig        *v1.AuthConfigController
	users             *v1.UserController
	me                *v1.MeController
	sessions          *v1.SessionController
	magicLinks        *v1.MagicLinkController
	resets            *v1.PasswordResetController
	verifications     *v1.EmailVerificationController
	confirmation      *v1.UserConfirmationController
	confirmationCodes *v1.UserConfirmationCodeController
	identities        *v1.UserIdentityController
	memberships       *v1.MembershipController
	workspace         *v1.CurrentWorkspaceController
	members           *v1.MemberController
	authFactors       *v1.AuthFactorController
	emailAddresses    *v1.EmailAddressController
}

/*
API declares the /api/v1 routes.
*/
func API(a *app.App) {
	c := newControllers(a)

	a.Router.API("/api/v1", "Dimmi API", a.Config.API.Version, func(api *routing.Router) {
		api.Group("", func(guest *routing.Router) {
			guest.Middleware("csrf", "session")

			routing.Get(guest, "/auth/config", "auth.config.show", c.authConfig.Show)
			routing.Post(guest, "/users", "users.store", c.users.Store)
			routing.Post(guest, "/sessions", "sessions.store", c.sessions.Store)
			routing.Post(guest, "/magic-links", "magic-links.store", c.magicLinks.Store)
			routing.Post(guest, "/password-resets", "password-resets.store", c.resets.Store)
			routing.Patch(guest, "/password-resets/{reset}", "password-resets.update", c.resets.Update)
			routing.Patch(guest, "/email-verifications/{verification}", "email-verifications.update",
				c.verifications.Update)
		})

		api.Group("", func(user *routing.Router) {
			user.Middleware("csrf", "session", "auth:user")

			routing.Get(user, "/sessions", "sessions.index", c.sessions.Index)
			routing.Delete(user, "/sessions", "sessions.destroy-many", c.sessions.DestroyMany).
				Middleware("password.confirm").Status(fhttp.StatusNoContent)
			routing.Delete(user, "/sessions/{session}", "sessions.destroy", c.sessions.Destroy).
				Status(fhttp.StatusNoContent)
			routing.Post(user, "/email-verifications", "email-verifications.store", c.verifications.Store)
			routing.Get(user, "/user/confirmation", "user.confirmation.show", c.confirmation.Show)
			routing.Post(user, "/user/confirmation", "user.confirmation.store", c.confirmation.Store)
			routing.Post(user, "/user/confirmation-codes", "user.confirmation-codes.store",
				c.confirmationCodes.Store)
			routing.Get(user, "/user/identities", "user.identities.index", c.identities.Index)
			routing.Delete(user, "/user/identities/{identity}", "user.identities.destroy", c.identities.Destroy).
				Middleware("password.confirm").Status(fhttp.StatusNoContent)
			routing.Get(user, "/user/memberships", "user.memberships.index", c.memberships.Index)
			routing.Delete(user, "/user/memberships/{membership}", "user.memberships.destroy", c.memberships.Destroy).
				Status(fhttp.StatusNoContent)
			authFactorRoutes(user, c.authFactors)
		})

		api.Group("", func(caller *routing.Router) {
			caller.Middleware("csrf", "session", "auth")

			routing.Get(caller, "/me", "me.show", c.me.Show)
		})

		api.Group("", func(workspace *routing.Router) { workspaceRoutes(workspace, c) })
	})
}

/*
workspaceRoutes declares the routes that work inside the workspace a
request names, or a token's own.
*/
func workspaceRoutes(workspace *routing.Router, c controllers) {
	workspace.Middleware("csrf", "session", "auth", "workspace")

	routing.Get(workspace, "/workspace", "workspace.show", c.workspace.Show)
	routing.Patch(workspace, "/workspace", "workspace.update", c.workspace.Update).Ability("settings:write")
	routing.Get(workspace, "/members", "members.index", c.members.Index).Ability("members:read")
	routing.Get(workspace, "/members/{member}", "members.show", c.members.Show).Ability("members:read")
	routing.Patch(workspace, "/members/{member}", "members.update", c.members.Update).Ability("members:write")
	emailAddressRoutes(workspace, c.emailAddresses)
}

/*
emailAddressRoutes declares the workspace's email addresses. Reading
them takes email:read, changing them email:manage, and the log of what
arrived, which shows senders and subjects, email:read_raw.
*/
func emailAddressRoutes(workspace *routing.Router, c *v1.EmailAddressController) {
	routing.Get(workspace, "/email-addresses", "email-addresses.index", c.Index).Ability("settings:read")
	routing.Post(workspace, "/email-addresses", "email-addresses.store", c.Store).
		Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, "/email-addresses/{address}", "email-addresses.show", c.ShowAddress).
		Ability("settings:read")
	routing.Patch(workspace, "/email-addresses/{address}", "email-addresses.update", c.Update).
		Ability("settings:write")
	routing.Delete(workspace, "/email-addresses/{address}", "email-addresses.destroy", c.Destroy).
		Ability("settings:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, "/email-addresses/{address}/tests", "email-addresses.tests.store", c.StoreTest).
		Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, "/email-addresses/{address}/inbound-emails", "email-addresses.inbound-emails.index",
		c.InboundEmails).Ability("settings:read")
}

/*
newControllers builds the controllers with the actions and the facts
about the server their answers carry.
*/
func newControllers(a *app.App) controllers {
	server := v1.Server{
		Edition: string(a.Config.App.Edition), Version: a.Version, APIVersion: a.Config.API.Version,
		Region: a.Config.App.Region, MailConfigured: a.MailConfigured(), Clock: a.Clock,
	}
	acts := a.Actions

	return controllers{
		authConfig: &v1.AuthConfigController{Auth: a.Config.Auth, Providers: a.Providers, Server: server},
		users:      &v1.UserController{RegisterUser: acts.RegisterUser, Cookies: a.Cookies, Server: server},
		me:         &v1.MeController{LoadMe: acts.LoadMe, IDs: a.IDs, Server: server},
		sessions: &v1.SessionController{
			AttemptLogin: acts.AttemptLogin, ConsumeMagicLink: acts.ConsumeMagicLink,
			TwoFactor: acts.CompleteTwoFactorChallenge, ListSessions: acts.ListSessions,
			EndSession: acts.EndSession, EndOtherSessions: acts.EndOtherSessions, IDs: a.IDs, Cookies: a.Cookies,
			Server: server,
		},
		magicLinks: &v1.MagicLinkController{SendMagicLink: acts.SendMagicLink, Cookies: a.Cookies},
		resets: &v1.PasswordResetController{
			SendPasswordResetLink: acts.SendPasswordResetLink, ResetPassword: acts.ResetPassword, Cookies: a.Cookies,
			Server: server,
		},
		verifications: &v1.EmailVerificationController{
			Resend: acts.ResendEmailVerification, Verify: acts.VerifyEmail, Cookies: a.Cookies,
		},
		confirmation: &v1.UserConfirmationController{
			Check: acts.CheckConfirmation, Confirm: acts.ConfirmUser, Cookies: a.Cookies,
		},
		confirmationCodes: &v1.UserConfirmationCodeController{
			SendConfirmCode: acts.SendConfirmCode, Cookies: a.Cookies,
		},
		identities: &v1.UserIdentityController{
			List: acts.ListOAuthIdentities, Unlink: acts.UnlinkOAuthIdentity, IDs: a.IDs, Cookies: a.Cookies,
		},
		memberships: &v1.MembershipController{List: acts.ListMemberships, Leave: acts.LeaveWorkspace, IDs: a.IDs},
		workspace:   &v1.CurrentWorkspaceController{UpdateWorkspace: acts.UpdateWorkspace},
		members: &v1.MemberController{
			ListMembers: acts.ListMembers, ShowMember: acts.ShowMember, UpdateMember: acts.UpdateMember, IDs: a.IDs,
			Cursors: pagination.NewCursors(a.DerivedKey(v1.CursorKeyPurpose)),
		},
		authFactors: &v1.AuthFactorController{
			List: acts.ListAuthFactors, Add: acts.AddAuthFactor, Activate: acts.ActivateAuthFactor,
			Change: acts.UpdateAuthFactor, Remove: acts.RemoveAuthFactor, RemoveAll: acts.RemoveAllAuthFactors,
			IDs: a.IDs, Cookies: a.Cookies,
		},
		emailAddresses: emailAddressController(a),
	}
}

/*
emailAddressController builds the controller of the workspace's email
addresses.
*/
func emailAddressController(a *app.App) *v1.EmailAddressController {
	acts := a.Actions

	return &v1.EmailAddressController{
		Create: acts.CreateEmailAddress, List: acts.ListEmailAddresses, Show: acts.ShowEmailAddress,
		Change: acts.UpdateEmailAddress, Remove: acts.DeleteEmailAddress, Test: acts.SendEmailAddressTest,
		Activity: acts.ListInboundEmails, IDs: a.IDs, Provider: a.EmailChannel.Transport,
		Cursors: pagination.NewCursors(a.DerivedKey(v1.CursorKeyPurpose)),
	}
}

/*
authFactorRoutes declares the caller's own auth factors. Adding and
removing factors needs a recent confirmation; activating needs only the
code, which proves the factor works.
*/
func authFactorRoutes(user *routing.Router, c *v1.AuthFactorController) {
	routing.Get(user, "/user/auth-factors", "user.auth-factors.index", c.Index)
	routing.Post(user, "/user/auth-factors", "user.auth-factors.store", c.Store).Middleware("password.confirm")
	routing.Patch(user, "/user/auth-factors/{factor}", "user.auth-factors.update", c.Update)
	routing.Delete(user, "/user/auth-factors", "user.auth-factors.destroy-many", c.DestroyMany).
		Middleware("password.confirm").Status(fhttp.StatusNoContent)
	routing.Delete(user, "/user/auth-factors/{factor}", "user.auth-factors.destroy", c.Destroy).
		Middleware("password.confirm").Status(fhttp.StatusNoContent)
}
