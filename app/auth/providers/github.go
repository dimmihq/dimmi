/*
|--------------------------------------------------------------------------
| GitHub
|--------------------------------------------------------------------------
|
| GitHub accounts sign in over plain OAuth 2 with PKCE. The person is
| keyed by GitHub's numeric id, which never changes, while the login
| name can; the login is kept for display only. The address is the
| account's primary one, verified only when GitHub verified it.
|
*/

package providers

import "github.com/dimmihq/dimmi/framework/socialite"

/*
GitHubKey is GitHub's provider key.
*/
const GitHubKey = "github"

/*
GitHub returns the GitHub provider, signing in through the social login
manager.
*/
func GitHub(manager *socialite.Manager) IdentityProvider {
	return social{
		key:      GitHubKey,
		protocol: ProtocolOAuth2,
		display:  Display{Name: "GitHub"},
		manager:  manager,
		identity: func(user socialite.User) ExternalIdentity {
			identity := baseIdentity(user)
			identity.Username = rawString(user, "login")

			return identity
		},
	}
}
