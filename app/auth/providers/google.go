/*
|--------------------------------------------------------------------------
| Google
|--------------------------------------------------------------------------
|
| Google accounts sign in over OpenID Connect: the person is read only
| from an ID token signed by Google, meant for this client and carrying
| this attempt's nonce. The person is keyed by the token's subject. A
| Workspace account also names its domain, kept as the hosted domain.
|
*/

package providers

import "github.com/dimmihq/dimmi/framework/socialite"

/*
GoogleKey is Google's provider key.
*/
const GoogleKey = "google"

/*
Google returns the Google provider, signing in through the social login
manager.
*/
func Google(manager *socialite.Manager) IdentityProvider {
	return social{
		key:      GoogleKey,
		protocol: ProtocolOIDC,
		display:  Display{Name: "Google"},
		manager:  manager,
		identity: func(user socialite.User) ExternalIdentity {
			identity := baseIdentity(user)
			identity.HostedDomain = rawString(user, "hd")

			return identity
		},
	}
}
