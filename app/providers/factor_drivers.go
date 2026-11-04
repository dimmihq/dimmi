/*
|--------------------------------------------------------------------------
| Factor Drivers
|--------------------------------------------------------------------------
|
| The kinds of second factor the install knows. Passkeys are bound to the
| host of the install's address and answer from that origin or a trusted
| one. An install reached by an IP address cannot hold passkeys at all,
| so there the kind is left out of the enabled ones instead of stopping
| the boot; every other kind still works.
|
*/

package providers

import (
	"net/url"
	"slices"

	"github.com/dimmihq/dimmi/framework/auth/webauthn"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/auth/factors"
)

/*
factorRegistry returns the registry of every factor driver with the
configured kinds enabled, passkeys only where the address can hold
them.
*/
func factorRegistry(fa *foundation.Application, a *app.App) (*factors.Registry, error) {
	cfg := a.Config.Auth.Factors
	party := relyingParty(fa, a)

	enabled := slices.Clone(cfg.Enabled)
	if party == nil {
		enabled = slices.DeleteFunc(enabled, func(kind string) bool { return kind == factors.PasskeyType })
	}

	return factors.NewRegistry(enabled,
		&factors.TOTP{Issuer: cfg.TOTP.Issuer, Keys: a.Keys, Cache: fa.Cache, Clock: fa.Clock},
		&factors.Passkey{Party: party},
		&factors.RecoveryCodes{Keys: a.Keys},
	)
}

/*
relyingParty returns the install's passkey relying party, or nil when
its address cannot hold passkeys.
*/
func relyingParty(fa *foundation.Application, a *app.App) *webauthn.RelyingParty {
	u, err := url.Parse(fa.Config.App.URL)
	if err != nil {
		return nil
	}

	party, err := webauthn.New(webauthn.Config{
		ID:      u.Hostname(),
		Name:    a.Config.Auth.Factors.Passkey.Name,
		Origins: append([]string{origin(fa.Config.App.URL)}, a.Config.HTTP.TrustedOrigins...),
		Clock:   fa.Clock,
	})
	if err != nil {
		return nil
	}

	return party
}
