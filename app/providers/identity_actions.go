/*
|--------------------------------------------------------------------------
| Identity Wiring
|--------------------------------------------------------------------------
|
| The actions behind a company's sign-in rules, and what happens when one
| changes: the audit entry, and every open connection of its members
| closing so the new rule bites on the next thing they do.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/accesspolicy"
)

/*
bootIdentity builds the actions of a workspace's sign-in rules.
*/
func bootIdentity(a *app.App) {
	acts := a.Actions
	acts.ShowAccessPolicy = &accesspolicy.ShowAccessPolicy{
		DB: a.DB, Clock: a.Clock, Providers: a.Providers, MagicLink: a.Config.Auth.MagicLink.Enabled,
	}
	acts.ClearAccessPolicy = &accesspolicy.ClearAccessPolicy{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.UpdateAccessPolicy = &accesspolicy.UpdateAccessPolicy{
		DB: a.DB, Events: a.Events, Mail: a.Mail, Links: a.Links, Show: acts.ShowAccessPolicy,
		MaxAge: a.Config.Auth.Sessions.AbsoluteLifetime, IdleLimit: a.Config.Auth.Sessions.IdleLifetime,
	}
}
