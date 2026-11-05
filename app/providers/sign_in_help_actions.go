/*
|--------------------------------------------------------------------------
| Sign-In Help Actions
|--------------------------------------------------------------------------
|
| The actions an admin uses to help a teammate who cannot sign in, and
| the one the teammate finishes it with. They turn a second factor off,
| so they are built after the factor actions they call.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/members"
)

/*
bootSignInHelp builds the actions that help a teammate back in.
*/
func bootSignInHelp(a *app.App) {
	acts := a.Actions

	acts.StartTwoFactorReset = &members.StartTwoFactorReset{
		DB: a.DB, Events: a.Events, Mail: a.Mail, Limiter: a.Limiter, Links: a.Links, Tokens: a.Secrets,
		SecurityLog: a.SecurityLog,
	}
	acts.CompleteTwoFactorReset = &members.CompleteTwoFactorReset{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Mail: a.Mail, Hash: a.Hash, Limiter: a.Limiter, IDs: a.IDs,
		Links: a.Links, Tokens: a.Secrets, SecurityLog: a.SecurityLog, Lockout: a.Lockout, Sessions: a.Session,
		AppGrants: a.AppGrants, RemoveAll: acts.RemoveAllAuthFactors,
	}
	acts.ClearSignInLock = &members.ClearSignInLock{DB: a.DB, Events: a.Events, Lockout: a.Lockout}
}
