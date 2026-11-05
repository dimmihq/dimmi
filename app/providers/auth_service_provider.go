/*
|--------------------------------------------------------------------------
| Auth Service Provider
|--------------------------------------------------------------------------
|
| Who is calling and what they may do. The permission catalog is checked
| first, so a permission without a group or a role row stops the boot
| instead of being granted or refused by accident. Browser sessions stay
| on the framework's store, which keeps how each user signed in, and take
| the product's cookie names and lifetimes.
|
| Guards run in the order they are registered and the first that finds
| its credential decides, so the bearer guard goes first: an explicit
| token wins over a cookie the browser sent along. Routes then name the
| kinds of caller they accept:
|
|   auth:session  the user's browser only
|   auth:user     the user themselves, by browser or by their own app
|   auth:token    an access token bound to one workspace
|   auth          any of them
|
*/

package providers

import (
	"context"
	"errors"
	"fmt"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/session"

	"github.com/dimmihq/dimmi/app"
	appauth "github.com/dimmihq/dimmi/app/auth"
	idp "github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/support/keysets"
)

/*
AuthServiceProvider wires sessions, guards and the sign-in services.
*/
type AuthServiceProvider struct {
	App *app.App
}

/*
Register checks the permission catalog and builds the session manager,
the guards and the sign-in services, mailed secrets, the links to the
account pages, the configured identity providers and the sealing of
attempts at them included, the data keys and the auth factors sealing
with them.
*/
func (p AuthServiceProvider) Register(fa *foundation.Application) error {
	a := p.App

	tokens, tokensErr := appauth.NewPersonalTokens(fa.DB, fa.Clock)
	if err := errors.Join(a.Permissions.Check(), tokensErr); err != nil {
		return err
	}

	a.Tokens = tokens
	a.Devices = appauth.NewDevices()
	a.Cookies = appauth.Cookies{Device: a.Config.Auth.Sessions.DeviceCookie, Secure: a.SecureCookies()}
	a.SecurityLog = &appauth.SecurityLog{DB: fa.DB, Clock: fa.Clock, Devices: a.Devices, Events: fa.Events}
	a.Lockout = &appauth.Lockout{Limiter: fa.Limiter, Cache: fa.Cache, Clock: fa.Clock, Config: a.Config.Auth.Lockout}
	a.Secrets = &appauth.OneTimeTokens{
		DB: fa.DB, Clock: fa.Clock, Codes: appauth.Codes{Key: fa.DerivedKey(appauth.CodeKeyPurpose)},
	}
	a.Links = appauth.Links{Base: fa.Config.App.URL}
	a.Providers = idp.Static{Registry: a.IdentityProviders, Configured: func(key string) bool {
		return a.Config.Auth.OAuth[key].ClientID != ""
	}}

	encrypter, err := encryption.NewEncrypter(fa.DerivedKey(appauth.OAuthKeyPurpose))
	if err != nil {
		return fmt.Errorf("seal provider sign-in attempts: %w", err)
	}

	a.OAuthAttempts = appauth.OAuthAttempts{Encrypter: encrypter}

	if a.Keys, err = fa.Crypt.Keys(&keysets.Store{DB: fa.DB}); err != nil {
		return fmt.Errorf("data keys: %w", err)
	}

	if a.Factors, err = factorRegistry(fa, a); err != nil {
		return err
	}

	fa.Session.Configure(session.Config{
		Cookie:      a.Config.Auth.Sessions.Cookie,
		Insecure:    !a.SecureCookies(),
		IdleTimeout: a.Config.Auth.Sessions.IdleLifetime,
		Lifetime:    a.Config.Auth.Sessions.AbsoluteLifetime,
		Errors:      fa.Errors,
	})

	p.registerGuards(fa)

	return nil
}

/*
registerGuards registers the bearer and session guards and names the
kinds of caller routes accept.
*/
func (p AuthServiceProvider) registerGuards(fa *foundation.Application) {
	tokens := &appauth.TokenGuard{DB: fa.DB, Tokens: p.App.Tokens}

	fa.Guards.Register(appauth.KindToken, fauth.BearerGuard(tokens))
	fa.Guards.Register(appauth.KindSession, &appauth.SessionGuard{DB: fa.DB})
	fa.Guards.Define(appauth.KindSession, exceptions.ErrSessionRequired, appauth.KindSession)
	fa.Guards.Define("user", exceptions.ErrFirstPartyOnly, appauth.KindSession, appauth.KindApp)
}

/*
Boot has nothing to start.
*/
func (p AuthServiceProvider) Boot(context.Context, *foundation.Application) error {
	return nil
}
