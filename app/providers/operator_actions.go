/*
|--------------------------------------------------------------------------
| Operator Actions
|--------------------------------------------------------------------------
|
| The actions the operator of a server reaches from the command line:
| creating the first user and rescuing accounts. They are built after
| the user and workspace actions they call.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/users"
)

/*
bootOperator builds the operator's actions.
*/
func bootOperator(a *app.App) {
	acts := a.Actions
	acts.CreateFirstUser = &users.CreateFirstUser{
		DB: a.DB, CreateUser: acts.CreateUser, CreateWorkspace: acts.CreateWorkspace,
	}
	acts.OperatorAccounts = &users.OperatorAccounts{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Hash: a.Hash, Sessions: a.Session, AppGrants: a.AppGrants,
		Links: a.Links, MinPasswordLength: a.Config.Auth.Passwords.MinLength,
	}
}
