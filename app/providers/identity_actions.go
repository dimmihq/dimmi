/*
|--------------------------------------------------------------------------
| Identity Wiring
|--------------------------------------------------------------------------
|
| The actions behind a company's sign-in rules and its custom roles, and
| what happens when one changes: the audit entry, and every open
| connection of its members closing so the new rule bites on the next
| thing they do.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/accesspolicy"
	"github.com/dimmihq/dimmi/app/actions/roles"
	"github.com/dimmihq/dimmi/app/actions/sso"
)

/*
bootIdentity builds the actions of a workspace's sign-in rules and
custom roles.
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

	bootRoles(a)
	bootSSO(a)
}

/*
bootRoles builds the actions of a workspace's custom roles.
*/
func bootRoles(a *app.App) {
	acts := a.Actions
	acts.ListRoles = &roles.ListRoles{DB: a.DB}
	acts.ShowRole = &roles.ShowRole{DB: a.DB}
	acts.CreateRole = &roles.CreateRole{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.UpdateRole = &roles.UpdateRole{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.DeleteRole = &roles.DeleteRole{DB: a.DB, Events: a.Events}
	acts.AssignRole = &roles.AssignRole{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.RoleNames = &roles.RoleNames{DB: a.DB}
	acts.ListPermissions = &roles.ListPermissions{Catalog: a.Permissions}
}

/*
bootSSO builds the actions of a company's identity provider connections
and proven domains.
*/
func bootSSO(a *app.App) {
	acts := a.Actions
	acts.CreateSSOConnection = &sso.CreateConnection{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Keys: a.Keys, Links: a.Links,
	}
	acts.UpdateSSOConnection = &sso.UpdateConnection{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Keys: a.Keys, Links: a.Links,
	}
	acts.ListSSOConnections = &sso.ListConnections{DB: a.DB}
	acts.ShowSSOConnection = &sso.ShowConnection{DB: a.DB}
	acts.DeleteSSOConnection = &sso.DeleteConnection{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.OrganizationDomains = &sso.Domains{DB: a.DB, Events: a.Events, Clock: a.Clock, DNS: a.DNS}
	acts.SSOOperator = &sso.Operator{DB: a.DB, Events: a.Events, Clock: a.Clock}
}
