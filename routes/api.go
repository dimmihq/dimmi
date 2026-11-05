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
|              works in: named by a browser or app, a token's own, and
|              only once the caller meets what the workspace requires
|
| What a workspace requires is never checked on the user's own routes,
| so a person shut out of a workspace can always set up a second factor
| or leave it.
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
	entitlements      *v1.WorkspaceEntitlementController
	workspace         *v1.CurrentWorkspaceController
	workspaces        *v1.WorkspaceController
	members           *v1.MemberController
	invitations       *v1.InvitationController
	deliveries        *v1.InvitationDeliveryController
	previews          *v1.InvitationPreviewController
	authFactors       *v1.AuthFactorController
	teams             *v1.TeamController
	teamMembers       *v1.TeamMemberController
	auditLogs         *v1.AuditLogController
	securityEvents    *v1.SecurityEventController
	tokens            *v1.PersonalAccessTokenController
	tokenAbilities    *v1.TokenAbilityController
	emailAddresses    *v1.EmailAddressController
}

/*
API declares the /api/v1 routes.
*/
func API(a *app.App) {
	c := newControllers(a)

	a.Router.API("/api/v1", "Dimmi API", a.Config.API.Version, func(api *routing.Router) {
		api.Group("", func(guest *routing.Router) { guestRoutes(guest, c) })
		api.Group("", func(user *routing.Router) { userRoutes(user, c) })

		api.Group("", func(caller *routing.Router) {
			caller.Middleware("csrf", "session", "auth", "api.throttle")

			routing.Get(caller, "/me", "me.show", c.me.Show)
		})

		api.Group("", func(workspace *routing.Router) {
			workspaceRoutes(workspace, c)
			realtimeRoutes(workspace, a)
			emailSenderRoutes(workspace, a)
		})
		api.Group("", func(tokens *routing.Router) { tokenRoutes(tokens, c) })
		api.Group("", func(inbox *routing.Router) { inboxRoutes(inbox, a) })
		identityRoutes(api, a)
		roleRoutes(api, a)
		accountRoutes(api, a)
		api.Group("", func(records *routing.Router) { recordRoutes(records, a) })
		api.Group("", func(widgets *routing.Router) { widgetSettingsRoutes(widgets, a) })
		signInHelpRoutes(api, a)
	})
}

/*
guestRoutes declares the routes anyone reaches before signing in.
*/
func guestRoutes(guest *routing.Router, c controllers) {
	guest.Middleware("csrf", "session")

	routing.Get(guest, "/auth/config", "auth.config.show", c.authConfig.Show)
	routing.Post(guest, "/users", "users.store", c.users.Store)
	routing.Post(guest, "/sessions", "sessions.store", c.sessions.Store)
	routing.Post(guest, "/magic-links", "magic-links.store", c.magicLinks.Store)
	routing.Post(guest, "/password-resets", "password-resets.store", c.resets.Store)
	routing.Patch(guest, "/password-resets/{reset}", "password-resets.update", c.resets.Update)
	routing.Patch(guest, "/email-verifications/{verification}", "email-verifications.update",
		c.verifications.Update)
	routing.Get(guest, "/invitations/preview", "invitations.preview.show", c.previews.Show)
}

/*
userRoutes declares the signed-in user's own routes, which a token
never reaches.
*/
func userRoutes(user *routing.Router, c controllers) {
	user.Middleware("csrf", "session", "auth:user", "api.throttle")

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
	routing.Post(user, "/user/memberships", "user.memberships.store", c.memberships.Store)
	routing.Patch(user, "/workspaces/{workspace}", "workspaces.update", c.workspaces.Update)
	routing.Delete(user, "/user/memberships/{membership}", "user.memberships.destroy", c.memberships.Destroy).
		Status(fhttp.StatusNoContent)
	authFactorRoutes(user, c.authFactors)
	routing.Get(user, "/user/security-events", "user.security-events.index", c.securityEvents.Index)
}

/*
workspaceRoutes declares the routes that work inside the workspace a
request names, or a token's own.
*/
func workspaceRoutes(workspace *routing.Router, c controllers) {
	workspace.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

	routing.Get(workspace, "/workspace", "workspace.show", c.workspace.Show)
	routing.Get(workspace, "/workspace/entitlements", "workspace.entitlements.show", c.entitlements.Show)
	routing.Patch(workspace, "/workspace", "workspace.update", c.workspace.Update).Ability("settings:write")
	routing.Delete(workspace, "/workspace", "workspace.destroy", c.workspace.Destroy).Status(fhttp.StatusOK)
	routing.Get(workspace, "/members", "members.index", c.members.Index).Ability("members:read").
		Use(c.members.ServeCSV)
	routing.Get(workspace, "/members/{member}", "members.show", c.members.Show).Ability("members:read")
	routing.Patch(workspace, "/members/me", "members.me.update", c.members.UpdateMe).Ability("conversations:write")
	routing.Patch(workspace, "/members/{member}", "members.update", c.members.Update).Ability("members:write")
	routing.Patch(workspace, "/members", "members.update-many", c.members.UpdateMany).Ability("members:write")
	routing.Get(workspace, "/audit-logs", "audit-logs.index", c.auditLogs.Index).Ability("settings:read").
		Use(c.auditLogs.ServeCSV)
	routing.Get(workspace, "/audit-logs/{audit_log}", "audit-logs.show", c.auditLogs.Show).Ability("settings:read")
	invitationRoutes(workspace, c)
	teamRoutes(workspace, c)
	emailAddressRoutes(workspace, c.emailAddresses)
}

/*
invitationRoutes declares the invitations of the workspace in play.
Inviting reaches other people, so it needs a verified email where the
install asks for one.
*/
func invitationRoutes(workspace *routing.Router, c controllers) {
	routing.Get(workspace, "/invitations", "invitations.index", c.invitations.Index).Ability("members:read")
	routing.Post(workspace, "/invitations", "invitations.store", c.invitations.Store).
		Ability("members:write").Middleware("verified").Status(fhttp.StatusOK)
	routing.Delete(workspace, "/invitations/{invitation}", "invitations.destroy", c.invitations.Destroy).
		Ability("members:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, "/invitations/{invitation}/deliveries", "invitations.deliveries.store",
		c.deliveries.Store).Ability("members:write")
}

/*
teamRoutes declares the workspace's teams and their members. Anyone who
may see the members sees the teams; changing them takes members:write.
*/
func teamRoutes(workspace *routing.Router, c controllers) {
	routing.Get(workspace, "/teams", "teams.index", c.teams.Index).Ability("members:read")
	routing.Post(workspace, "/teams", "teams.store", c.teams.Store).Ability("members:write")
	routing.Get(workspace, "/teams/{team}", "teams.show", c.teams.Show).Ability("members:read")
	routing.Patch(workspace, "/teams/{team}", "teams.update", c.teams.Update).Ability("members:write")
	routing.Delete(workspace, "/teams/{team}", "teams.destroy", c.teams.Destroy).Ability("members:write").
		Status(fhttp.StatusNoContent)
	routing.Post(workspace, "/teams/{team}/members", "teams.members.store", c.teamMembers.Store).
		Ability("members:write").Status(fhttp.StatusOK)
	routing.Delete(workspace, "/teams/{team}/members/{member}", "teams.members.destroy", c.teamMembers.Destroy).
		Ability("members:write").Status(fhttp.StatusNoContent)
}

/*
tokenRoutes declares the caller's personal access tokens in the
workspace in play. Only the user themselves reaches them, never a
token, so a stolen token can neither mint another nor hide its tracks.
Minting needs a verified email and a recent confirmation.
*/
func tokenRoutes(tokens *routing.Router, c controllers) {
	tokens.Middleware("csrf", "session", "auth:user", "workspace", "api.throttle")

	routing.Get(tokens, "/tokens", "tokens.index", c.tokens.Index)
	routing.Post(tokens, "/tokens", "tokens.store", c.tokens.Store).
		Middleware("verified", "password.confirm")
	routing.Get(tokens, "/tokens/abilities", "tokens.abilities.index", c.tokenAbilities.Index)
	routing.Get(tokens, "/tokens/{token}", "tokens.show", c.tokens.ShowToken)
	routing.Patch(tokens, "/tokens/{token}", "tokens.update", c.tokens.Update)
	routing.Delete(tokens, "/tokens/{token}", "tokens.destroy", c.tokens.Destroy).Status(fhttp.StatusNoContent)
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
		AuditRetentionDays: *a.Config.Audit.RetentionDays,
	}
	acts := a.Actions
	cursors := pagination.NewCursors(a.DerivedKey(v1.CursorKeyPurpose))

	c := controllers{
		authConfig: &v1.AuthConfigController{
			Auth: a.Config.Auth, Factors: a.Factors.Types(), Providers: a.Providers, Server: server,
		},
		users: &v1.UserController{RegisterUser: acts.RegisterUser, Cookies: a.Cookies, Server: server},
		me:    &v1.MeController{LoadMe: acts.LoadMe, IDs: a.IDs, Server: server},
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
		authFactors: &v1.AuthFactorController{
			List: acts.ListAuthFactors, Add: acts.AddAuthFactor, Activate: acts.ActivateAuthFactor,
			Change: acts.UpdateAuthFactor, Remove: acts.RemoveAuthFactor, RemoveAll: acts.RemoveAllAuthFactors,
			IDs: a.IDs, Cookies: a.Cookies,
		},
		securityEvents: &v1.SecurityEventController{List: acts.ListSecurityEvents, Cursors: cursors},
	}
	workspaceControllers(a, &c, cursors)
	membershipControllers(a, &c, cursors)
	teamControllers(a, &c, cursors)

	return c
}

/*
workspaceControllers builds into c the controllers of the workspace in
play: its settings, members, audit log, tokens and email addresses.
*/
func workspaceControllers(a *app.App, c *controllers, cursors *pagination.Cursors) {
	acts := a.Actions

	c.workspace = &v1.CurrentWorkspaceController{
		UpdateWorkspace: acts.UpdateWorkspace, DeleteWorkspace: acts.DeleteWorkspace,
	}
	c.entitlements = &v1.WorkspaceEntitlementController{Entitlements: a.Entitlements, Limits: a.Config.API.Limits}
	c.members = &v1.MemberController{
		ListMembers: acts.ListMembers, ShowMember: acts.ShowMember, UpdateMember: acts.UpdateMember, IDs: a.IDs,
		AccessTeams: acts.ListAccessTeams, AssignRole: acts.AssignRole, RoleNames: acts.RoleNames,
		MemberTeams: acts.ListMemberTeams, Cursors: cursors, Lockout: a.Lockout, Export: acts.ExportMembers,
		Errors: a.Errors,
	}
	c.auditLogs = &v1.AuditLogController{
		List: acts.ListAuditLogs, Find: acts.ShowAuditLog, Export: acts.ExportAuditLogs, IDs: a.IDs, Cursors: cursors,
		Errors: a.Errors,
	}
	c.tokens = tokenController(a, cursors)
	c.tokenAbilities = &v1.TokenAbilityController{List: acts.ListTokenAbilities}
	c.emailAddresses = emailAddressController(a, cursors)
}

/*
emailAddressController builds the controller of the workspace's email
addresses.
*/
func emailAddressController(a *app.App, cursors *pagination.Cursors) *v1.EmailAddressController {
	acts := a.Actions

	return &v1.EmailAddressController{
		Create: acts.CreateEmailAddress, List: acts.ListEmailAddresses, Show: acts.ShowEmailAddress,
		Change: acts.UpdateEmailAddress, Remove: acts.DeleteEmailAddress, Test: acts.SendEmailAddressTest,
		Activity: acts.ListInboundEmails, IDs: a.IDs, Provider: a.EmailChannel.Transport, Cursors: cursors,
		Senders: a.EmailDomains, Guides: acts.RecommendGuide,
	}
}

/*
tokenController builds the controller of the caller's personal access
tokens.
*/
func tokenController(a *app.App, cursors *pagination.Cursors) *v1.PersonalAccessTokenController {
	acts := a.Actions

	return &v1.PersonalAccessTokenController{
		Create: acts.CreatePersonalAccessToken, List: acts.ListPersonalAccessTokens,
		Show: acts.ShowPersonalAccessToken, Rename: acts.RenamePersonalAccessToken,
		Revoke: acts.RevokePersonalAccessToken, IDs: a.IDs, Cookies: a.Cookies, Cursors: cursors,
	}
}

/*
teamControllers builds into c the controllers of the workspace's teams
and their members.
*/
func teamControllers(a *app.App, c *controllers, cursors *pagination.Cursors) {
	acts := a.Actions

	c.teams = &v1.TeamController{
		ListTeams: acts.ListTeams, ShowTeam: acts.ShowTeam, CreateTeam: acts.CreateTeam,
		UpdateTeam: acts.UpdateTeam, DeleteTeam: acts.DeleteTeam, IDs: a.IDs,
		Cursors: cursors,
	}
	c.teamMembers = &v1.TeamMemberController{Add: acts.AddTeamMembers, Remove: acts.RemoveTeamMember, IDs: a.IDs}
}

/*
membershipControllers builds into c the controllers that bring people
into workspaces and take workspaces away: memberships, invitations and
the trash.
*/
func membershipControllers(a *app.App, c *controllers, cursors *pagination.Cursors) {
	acts := a.Actions

	c.memberships = &v1.MembershipController{
		List: acts.ListMemberships, Accept: acts.AcceptInvitation, Leave: acts.LeaveWorkspace, IDs: a.IDs,
	}
	c.workspaces = &v1.WorkspaceController{Restore: acts.RestoreWorkspace, IDs: a.IDs}

	c.invitations = &v1.InvitationController{
		List: acts.ListInvitations, Create: acts.CreateInvitations, Revoke: acts.RevokeInvitation, IDs: a.IDs,
		Cursors: cursors,
	}
	c.deliveries = &v1.InvitationDeliveryController{Resend: acts.ResendInvitation, IDs: a.IDs}
	c.previews = &v1.InvitationPreviewController{Preview: acts.PreviewInvitation}
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
