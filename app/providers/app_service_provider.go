/*
|--------------------------------------------------------------------------
| App Service Provider
|--------------------------------------------------------------------------
|
| The first provider to run: it ties the product's application to the
| framework's, registers the public id prefixes, the product's
| validation rules and its mail, and once every provider has registered,
| builds the business actions with the services each one uses. Actions
| that set up new workspaces share one list of initializers, which the
| parts of the product needing defaults fill while registering, in the
| order their defaults build on each other: the default inbox first.
|
*/

package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	idp "github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/rules"
	"github.com/dimmihq/dimmi/resources/data"
)

/*
dummyPasswordLength is the length of the random password behind the
hash checked when a sign-in names no account with a password.
*/
const dummyPasswordLength = 32

/*
AppServiceProvider wires the application and its actions.
*/
type AppServiceProvider struct {
	App *app.App
}

/*
Register binds the framework application and registers the public id
prefixes, the identity providers, the validation rules, the mailables
and the workspace initializers. The providers come before the rules,
since their keys are registry values the rules accept.
*/
func (p AppServiceProvider) Register(fa *foundation.Application) error {
	p.App.Application = fa
	appmail.Register(fa.Mail)
	p.App.WorkspaceInitializers.Register(&inboxes.EnsureDefaultInbox{Clock: fa.Clock})

	identity := p.App.IdentityProviders
	if err := identity.Register(idp.Google(fa.Social), idp.GitHub(fa.Social)); err != nil {
		return err
	}

	common, err := data.CommonPasswords()
	if err != nil {
		return err
	}

	registerEmail(p.App, fa)

	errs := []error{models.RegisterPublicIDs(fa.IDs)}

	for _, rule := range []validation.Rule{
		rules.PasswordLength{Min: p.App.Config.Auth.Passwords.MinLength},
		rules.UncommonPassword{Common: common},
		rules.Registered{Registries: appauth.Registries(identity.Keys())},
		rules.Slug{},
		rules.Timezone{},
		rules.Locale{Supported: fa.Lang.Locales()},
	} {
		errs = append(errs, fa.Validator.Register(rule))
	}

	return errors.Join(errs...)
}

/*
Boot builds every action and registers the jobs' workers.
*/
func (p AppServiceProvider) Boot(ctx context.Context, _ *foundation.Application) error {
	a := p.App

	dummy, err := a.Hash.Make(ctx, str.Random(dummyPasswordLength))
	if err != nil {
		return fmt.Errorf("hash the dummy password: %w", err)
	}

	a.Actions = &app.Actions{
		CreateUser: &users.CreateUser{DB: a.DB, Clock: a.Clock},
		CreateWorkspace: &workspaces.CreateWorkspace{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Initializers: a.WorkspaceInitializers,
		},
		StartSession: &auth.StartSession{
			DB: a.DB, Clock: a.Clock, Mail: a.Mail, Links: a.Links, Devices: a.Devices, SecurityLog: a.SecurityLog,
		},
		LoadMe:           &auth.LoadMe{DB: a.DB, Permissions: a.Permissions},
		ListSessions:     &auth.ListSessions{Sessions: a.Session},
		EndSession:       &auth.EndSession{Sessions: a.Session, Events: a.Events, SecurityLog: a.SecurityLog},
		EndOtherSessions: &auth.EndOtherSessions{Sessions: a.Session, SecurityLog: a.SecurityLog},
	}

	bootInvitations(a)
	bootSignIn(a, dummy)
	bootMailedSecrets(a)
	bootIdentities(a)
	bootWorkspaces(a)
	bootWorkspaceLifecycle(a)
	bootFactors(a)
	bootTeams(a)
	bootTokens(a)

	if err := bootAudit(a); err != nil {
		return err
	}

	if err := bootInbox(a); err != nil {
		return err
	}

	if err := bootEmail(a); err != nil {
		return err
	}

	return errors.Join(registerJobs(a), registerWorkspaceJobs(a))
}

/*
bootSignIn builds the actions that sign people up and in.
*/
func bootSignIn(a *app.App, dummy string) {
	acts := a.Actions
	acts.CompletePrimaryAuth = &auth.CompletePrimaryAuth{
		DB: a.DB, Tokens: a.Secrets, Factors: a.Factors, StartSession: acts.StartSession, LoadMe: acts.LoadMe,
	}
	acts.SendEmailVerification = &auth.SendEmailVerification{
		DB: a.DB, Mail: a.Mail, Links: a.Links, Tokens: a.Secrets,
	}

	acts.AttemptLogin = &auth.AttemptLogin{
		DB: a.DB, Hash: a.Hash, Clock: a.Clock, Mail: a.Mail, Limiter: a.Limiter, Links: a.Links,
		LoginPolicy: a.LoginPolicy, Lockout: a.Lockout, SecurityLog: a.SecurityLog,
		CompletePrimaryAuth: acts.CompletePrimaryAuth, DummyHash: dummy,
	}

	acts.RegisterUser = &auth.RegisterUser{
		DB: a.DB, Hash: a.Hash, Events: a.Events, Limiter: a.Limiter, Signup: a.Config.Auth.Signup,
		LoginPolicy: a.LoginPolicy, CreateUser: acts.CreateUser, CreateWorkspace: acts.CreateWorkspace,
		SendVerification: acts.SendEmailVerification, StartSession: acts.StartSession, LoadMe: acts.LoadMe,
		Preview: acts.PreviewInvitation, Accept: acts.AcceptInvitation,
	}

	acts.ConfirmUser = &auth.ConfirmUser{
		Hash: a.Hash, Lockout: a.Lockout, Tokens: a.Secrets,
		TTL: a.Config.Auth.PasswordConfirmation.TTL,
	}
	acts.CheckConfirmation = &auth.CheckConfirmation{Clock: a.Clock, TTL: a.Config.Auth.PasswordConfirmation.TTL}
}

/*
bootMailedSecrets builds the actions that mail links and codes and
redeem them.
*/
func bootMailedSecrets(a *app.App) {
	acts := a.Actions
	acts.ResendEmailVerification = &auth.ResendEmailVerification{Limiter: a.Limiter, Send: acts.SendEmailVerification}
	acts.VerifyEmail = &auth.VerifyEmail{
		DB: a.DB, Clock: a.Clock, Events: a.Events, IDs: a.IDs, Limiter: a.Limiter, Tokens: a.Secrets,
	}

	acts.SendPasswordResetLink = &auth.SendPasswordResetLink{
		DB: a.DB, Clock: a.Clock, Mail: a.Mail, Limiter: a.Limiter, Lockout: a.Lockout, Links: a.Links,
	}

	acts.ResetPassword = &auth.ResetPassword{
		DB: a.DB, Clock: a.Clock, Hash: a.Hash, Mail: a.Mail, Limiter: a.Limiter, IDs: a.IDs, Links: a.Links,
		LoginPolicy: a.LoginPolicy, Sessions: a.Session, AppGrants: a.AppGrants, SecurityLog: a.SecurityLog,
		CompletePrimaryAuth: acts.CompletePrimaryAuth,
	}

	acts.SendMagicLink = &auth.SendMagicLink{
		DB: a.DB, Clock: a.Clock, Mail: a.Mail, Limiter: a.Limiter, Links: a.Links, Devices: a.Devices,
		Tokens: a.Secrets, LoginPolicy: a.LoginPolicy, Lockout: a.Lockout, MagicLink: a.Config.Auth.MagicLink,
		Signup: a.Config.Auth.Signup,
	}

	acts.ConsumeMagicLink = &auth.ConsumeMagicLink{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Limiter: a.Limiter, Lockout: a.Lockout, Tokens: a.Secrets,
		LoginPolicy: a.LoginPolicy, MagicLink: a.Config.Auth.MagicLink, SecurityLog: a.SecurityLog,
		CreateUser: acts.CreateUser, CompletePrimaryAuth: acts.CompletePrimaryAuth,
		ClaimAccount: &auth.ClaimAccount{
			Clock: a.Clock, Mail: a.Mail, Links: a.Links, Sessions: a.Session, AppGrants: a.AppGrants,
			SecurityLog: a.SecurityLog,
		},
	}

	acts.SendConfirmCode = &auth.SendConfirmCode{DB: a.DB, Mail: a.Mail, Limiter: a.Limiter, Tokens: a.Secrets}
}

/*
bootIdentities builds the actions that sign in with identity providers
and manage the linked accounts.
*/
func bootIdentities(a *app.App) {
	acts := a.Actions
	acts.LinkOAuthIdentity = &auth.LinkOAuthIdentity{
		DB: a.DB, Clock: a.Clock, Mail: a.Mail, Links: a.Links, Providers: a.Providers, SecurityLog: a.SecurityLog,
	}

	acts.HandleOAuthCallback = &auth.HandleOAuthCallback{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue, LoginPolicy: a.LoginPolicy,
		Signup: a.Config.Auth.Signup, CreateUser: acts.CreateUser, Link: acts.LinkOAuthIdentity,
		CompletePrimaryAuth: acts.CompletePrimaryAuth, ClaimAccount: acts.ConsumeMagicLink.ClaimAccount,
	}

	acts.ListOAuthIdentities = &auth.ListOAuthIdentities{DB: a.DB}
	acts.UnlinkOAuthIdentity = &auth.UnlinkOAuthIdentity{
		DB: a.DB, Clock: a.Clock, Providers: a.Providers, MagicLink: a.Config.Auth.MagicLink,
		SecurityLog: a.SecurityLog,
	}
}

/*
bootWorkspaces builds the actions that work inside workspaces and on
their members.
*/
func bootWorkspaces(a *app.App) {
	acts := a.Actions
	acts.ResolveWorkspace = &workspaces.ResolveWorkspace{
		DB: a.DB, Clock: a.Clock, IDs: a.IDs, Permissions: a.Permissions,
	}
	acts.UpdateWorkspace = &workspaces.UpdateWorkspace{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Mail: a.Mail, Links: a.Links,
		ConfirmWindow: a.Config.Auth.PasswordConfirmation.TTL,
	}
	acts.ListMemberships = &workspaces.ListMemberships{DB: a.DB}
	acts.RepairWorkspaces = &workspaces.RepairWorkspaces{
		DB: a.DB, Clock: a.Clock, Initializers: a.WorkspaceInitializers,
	}
	acts.ListMembers = &members.ListMembers{DB: a.DB}
	acts.ShowMember = &members.ShowMember{DB: a.DB}
	acts.UpdateMember = &members.UpdateMember{
		DB: a.DB, Events: a.Events, Clock: a.Clock, ConfirmWindow: a.Config.Auth.PasswordConfirmation.TTL,
	}
	acts.LeaveWorkspace = &members.LeaveWorkspace{DB: a.DB, Events: a.Events, Clock: a.Clock}
}

/*
registerJobs registers the worker of every product job.
*/
func registerJobs(a *app.App) error {
	disk, err := a.Storage.Default()
	if err != nil {
		return fmt.Errorf("avatar disk: %w", err)
	}

	return errors.Join(
		queue.Register(a.Queue, &jobs.FetchUserAvatar{DB: a.DB, Clock: a.Clock, HTTP: a.HTTP, Disk: disk}),
		queue.Register(a.Queue, &jobs.NotifyExpiringTokens{DB: a.DB, Clock: a.Clock, Mail: a.Mail, Links: a.Links}),
		queue.Register(a.Queue, &jobs.PurgeExpiredAuthRecords{
			DB: a.DB, Clock: a.Clock, PendingFactor: a.Config.Auth.Factors.PendingTTL,
			SessionIdle: a.Config.Auth.Sessions.IdleLifetime,
		}),
	)
}
