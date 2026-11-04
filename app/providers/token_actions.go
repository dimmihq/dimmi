/*
|--------------------------------------------------------------------------
| Personal Access Token Actions
|--------------------------------------------------------------------------
|
| The actions behind personal access tokens: minting one, listing,
| renaming and revoking them, and the abilities a member may hand out.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/tokens"
)

/*
bootTokens builds the actions that manage personal access tokens.
*/
func bootTokens(a *app.App) {
	acts := a.Actions

	acts.CreatePersonalAccessToken = &tokens.CreatePersonalAccessToken{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Mail: a.Mail, Limiter: a.Limiter, Links: a.Links,
		Tokens: a.Tokens, Permissions: a.Permissions, SecurityLog: a.SecurityLog, Config: a.Config.Tokens,
	}
	acts.ListPersonalAccessTokens = &tokens.ListPersonalAccessTokens{DB: a.DB, Clock: a.Clock}
	acts.ShowPersonalAccessToken = &tokens.ShowPersonalAccessToken{DB: a.DB}
	acts.RenamePersonalAccessToken = &tokens.RenamePersonalAccessToken{DB: a.DB, Clock: a.Clock}
	acts.RevokePersonalAccessToken = &tokens.RevokePersonalAccessToken{
		DB: a.DB, Clock: a.Clock, Events: a.Events, SecurityLog: a.SecurityLog,
	}
	acts.ListTokenAbilities = &tokens.ListTokenAbilities{Permissions: a.Permissions}
}
