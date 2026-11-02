/*
|--------------------------------------------------------------------------
| Sign-In Options
|--------------------------------------------------------------------------
|
| Apps ask the server how people may sign in before anyone has: which
| providers are set up and which second factors may be enrolled. A
| provider is offered only once its client id is configured, and a
| factor only when the install enables it, so an app never shows a
| button that cannot work. Each entry is an object, so new fields can be
| added without breaking older apps.
|
*/

package auth

import (
	"slices"

	"github.com/dimmihq/dimmi/config"
)

/*
Provider is a sign-in provider as apps show it. Protocol is oauth2 or
oidc.
*/
type Provider struct {
	Key      string
	Name     string
	Protocol string
}

/*
Factor is a second-factor type as apps show it.
*/
type Factor struct {
	Type string
	Name string
}

/*
OfferedProviders returns the providers configured with a client id, in
the order apps list them.
*/
func OfferedProviders(cfg config.AuthConfig) []Provider {
	known := []Provider{
		{Key: MethodGitHub, Name: "GitHub", Protocol: "oauth2"},
		{Key: MethodGoogle, Name: "Google", Protocol: "oidc"},
	}

	return slices.DeleteFunc(known, func(p Provider) bool { return cfg.OAuth[p.Key].ClientID == "" })
}

/*
OfferedFactors returns the second-factor types the install enables, in
the order apps list them.
*/
func OfferedFactors(cfg config.AuthConfig) []Factor {
	known := []Factor{
		{Type: MethodTOTP, Name: "Authenticator app"},
		{Type: MethodRecoveryCode, Name: "Recovery codes"},
	}

	return slices.DeleteFunc(known, func(f Factor) bool { return !slices.Contains(cfg.Factors.Enabled, f.Type) })
}
