/*
|--------------------------------------------------------------------------
| App Service Provider
|--------------------------------------------------------------------------
|
| The first provider to run: it ties the product's application to the
| framework's, registers the public id prefixes and the product's
| validation rules, and once every provider has registered, builds the
| business actions with the services each one uses.
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
prefixes and validation rules.
*/
func (p AppServiceProvider) Register(fa *foundation.Application) error {
	p.App.Application = fa

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
		CreateUser:       &users.CreateUser{DB: a.DB, Clock: a.Clock},
		CreateWorkspace:  &workspaces.CreateWorkspace{DB: a.DB, Events: a.Events, Clock: a.Clock},
		StartSession:     &auth.StartSession{Devices: a.Devices, SecurityLog: a.SecurityLog},
		LoadMe:           &auth.LoadMe{DB: a.DB, Permissions: a.Permissions},
		ListSessions:     &auth.ListSessions{Sessions: a.Session},
		EndSession:       &auth.EndSession{Sessions: a.Session, Events: a.Events, SecurityLog: a.SecurityLog},
		EndOtherSessions: &auth.EndOtherSessions{Sessions: a.Session, SecurityLog: a.SecurityLog},
	}

	a.Actions.AttemptLogin = &auth.AttemptLogin{
		DB: a.DB, Hash: a.Hash, Clock: a.Clock, LoginPolicy: a.LoginPolicy, Lockout: a.Lockout,
		SecurityLog: a.SecurityLog, StartSession: a.Actions.StartSession, LoadMe: a.Actions.LoadMe, DummyHash: dummy,
	}

	a.Actions.RegisterUser = &auth.RegisterUser{
		DB: a.DB, Hash: a.Hash, Events: a.Events, Limiter: a.Limiter, Signup: a.Config.Auth.Signup,
		LoginPolicy: a.LoginPolicy, CreateUser: a.Actions.CreateUser, CreateWorkspace: a.Actions.CreateWorkspace,
		StartSession: a.Actions.StartSession, LoadMe: a.Actions.LoadMe,
	}

	return nil
}
