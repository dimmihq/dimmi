/*
|--------------------------------------------------------------------------
| Google Provider
|--------------------------------------------------------------------------
|
| Google is an OpenID Connect issuer, so signing in with Google is the
| generic OpenID Connect flow with Google's issuer filled in. Google is
| always asked to show its account chooser, because people often hold
| several Google accounts and must pick the one they mean. A managed
| account's domain arrives as the hd claim in the user's raw claims.
|
|   providers:
|     google:
|       driver: google
|       client_id: ...
|       client_secret: ...
|
*/

package google

import (
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/drivers/oidc"
)

/*
Options configure the Google provider: the OAuth client registered in
Google Cloud, and the issuer, which only tests change.
*/
type Options struct {
	ClientID     string `koanf:"client_id"`
	ClientSecret string `koanf:"client_secret"`
	Issuer       string `default:"https://accounts.google.com" koanf:"issuer"`
}

/*
Factory builds Google providers for the socialite manager.
*/
func Factory(client *httpclient.Client, clk clock.Clock) manager.Factory[socialite.Provider] {
	return func(name string, section config.Section) (socialite.Provider, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(name, client, clk, opts)
	}
}

/*
New returns the Google provider configured under the name.
*/
func New(name string, client *httpclient.Client, clk clock.Clock, opts Options) (*oidc.Provider, error) {
	return oidc.New(name, client, clk, oidc.Options{
		Issuer:       opts.Issuer,
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		Scopes:       []string{"openid", "email", "profile"},
		Prompt:       "select_account",
	})
}
