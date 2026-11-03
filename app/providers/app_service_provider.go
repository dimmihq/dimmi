/*
|--------------------------------------------------------------------------
| App Service Provider
|--------------------------------------------------------------------------
|
| The first provider to run: it ties the product's application to the
| framework's, registers the public id prefixes, the product's
| validation rules and its mail, and once every provider has registered,
| builds the business actions with the services each one uses.
|
*/

package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
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
prefixes, the validation rules and the mailables.
*/
func (p AppServiceProvider) Register(fa *foundation.Application) error {
	p.App.Application = fa
	appmail.Register(fa.Mail)

	common, err := data.CommonPasswords()
	if err != nil {
		return err
	}

	errs := []error{models.RegisterPublicIDs(fa.IDs)}

	for _, rule := range []validation.Rule{
		rules.PasswordLength{Min: p.App.Config.Auth.Passwords.MinLength},
		rules.UncommonPassword{Common: common},
		rules.Registered{Registries: appauth.Registries()},
	} {
		errs = append(errs, fa.Validator.Register(rule))
	}

	return errors.Join(errs...)
}

/*
Boot builds every action.
*/
func (p AppServiceProvider) Boot(ctx context.Context, _ *foundation.Application) error {
	a := p.App

	dummy, err := a.Hash.Make(ctx, str.Random(dummyPasswordLength))
	if err != nil {
		return fmt.Errorf("hash the dummy password: %w", err)
	}

	a.Actions = &app.Actions{
		CreateUser:      &users.CreateUser{DB: a.DB, Clock: a.Clock},
		CreateWorkspace: &workspaces.CreateWorkspace{DB: a.DB, Events: a.Events, Clock: a.Clock},
		StartSession: &auth.StartSession{
			DB: a.DB, Clock: a.Clock, Mail: a.Mail, Links: a.Links, Devices: a.Devices, SecurityLog: a.SecurityLog,
		},
		LoadMe:           &auth.LoadMe{DB: a.DB, Permissions: a.Permissions},
		ListSessions:     &auth.ListSessions{Sessions: a.Session},
		EndSession:       &auth.EndSession{Sessions: a.Session, Events: a.Events, SecurityLog: a.SecurityLog},
		EndOtherSessions: &auth.EndOtherSessions{Sessions: a.Session, SecurityLog: a.SecurityLog},
	}

	bootSignIn(a, dummy)
	bootMailedSecrets(a)

	return nil
}

/*
bootSignIn builds the actions that sign people up and in.
*/
func bootSignIn(a *app.App, dummy string) {
	acts := a.Actions
	acts.CompletePrimaryAuth = &auth.CompletePrimaryAuth{StartSession: acts.StartSession, LoadMe: acts.LoadMe}
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
		DB: a.DB, Mail: a.Mail, Limiter: a.Limiter, Links: a.Links, Devices: a.Devices, Tokens: a.Secrets,
		LoginPolicy: a.LoginPolicy, Lockout: a.Lockout, MagicLink: a.Config.Auth.MagicLink,
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
