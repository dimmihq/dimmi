/*
|--------------------------------------------------------------------------
| Sign-In Options
|--------------------------------------------------------------------------
|
| Apps ask the server how people may sign in before anyone has: which
| second factors may be enrolled and which ways of signing in by mail it
| serves; the identity providers come from their own registry. A factor
| is offered only when the install enables it, and a capability only
| when it works, so an app never shows a button that cannot work. Each
| entry is an object, so new fields can be added without breaking older
| apps.
|
*/

package auth

import (
	"slices"

	"github.com/dimmihq/dimmi/config"
)

/*
Factor is a second-factor type as apps show it.
*/
type Factor struct {
	Type string
	Name string
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

/*
The capabilities a server may announce, so an app newer than the server
hides what the server cannot do.
*/
const (
	CapabilityMagicLink = "magic_link"
	CapabilityEmailCode = "email_code"
)

/*
Capabilities returns what the install serves of the capabilities, in
the order apps list them: signing in by mailed link and by mailed code
while magic links are on.
*/
func Capabilities(cfg config.AuthConfig) []string {
	if !cfg.MagicLink.Enabled {
		return []string{}
	}

	return []string{CapabilityMagicLink, CapabilityEmailCode}
}
