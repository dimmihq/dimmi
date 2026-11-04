/*
|--------------------------------------------------------------------------
| Application
|--------------------------------------------------------------------------
|
| The product's application is the framework's, plus what only this
| product has: its typed configuration, the permission catalog, the
| sign-in policy and lockout, the identity providers, and the business
| actions, each built once with exactly the services it uses. Nothing here is looked up by name:
| providers fill the fields while the application boots, and everything
| after boot receives what it needs through its constructor.
|
| The application is created before the framework boots it, so tests can
| swap a field such as the login policy, or register an identity
| provider of its own, before any action is built.
|
*/

package app

import (
	"cmp"
	"io/fs"
	"maps"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/view/vite"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/auth/providers"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/channels"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/support/helpsample"
	"github.com/dimmihq/dimmi/app/support/retention"
	"github.com/dimmihq/dimmi/app/support/widgetassets"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
App is the booted product.

Version is the release the binary was built as. Config is the
application's own configuration; the framework's stays on
Application.Config. Permissions is the permission catalog,
LoginPolicy decides which sign-in methods an address may use, and
AccessPolicy what a workspace requires of the callers working in it.
IdentityProviders holds the registered identity provider drivers, and
Providers finds the ones the install configured. Factors holds the auth
factor drivers and knows which the install enables, sealing their
secrets with Keys, the install's data keys. AppGrants
ends the user's first-party app grants. Tokens, Devices, Cookies,
Lockout, SecurityLog, Secrets, Links and OAuthAttempts are the sign-in
services providers build at boot, and Actions every business action.
Channels holds the channel drivers replies go out through.
WorkspaceInitializers are the setup steps every new workspace goes
through, registered by the parts of the product that need defaults.
Retention is asked before any purge destroys data. Entitlements answers
what a workspace may have and BillingStatus whether it ever paid.
EmailChannel is the
email channel driver, which also receives inbound mail.
Frontend is the browser app's production build and Vite links it, or
the dev server while one runs, into pages. Widget is the messenger
widget's build and WidgetAssets serves it. HelpSample is the sample help
center and changelog the public pages show until their content lives in
the database. Realtime authenticates realtime connections.
*/
type App struct {
	*foundation.Application

	Version      string
	Config       *appconfig.Config
	Permissions  *auth.Catalog
	LoginPolicy  auth.LoginPolicyResolver
	AccessPolicy auth.AccessPolicy
	AppGrants    auth.AppGrants

	IdentityProviders *providers.Registry
	Providers         providers.ProviderResolver
	Factors           *factors.Registry
	Keys              *encryption.Keys

	Tokens      *auth.PersonalTokens
	Devices     *auth.Devices
	Cookies     auth.Cookies
	Lockout     *auth.Lockout
	SecurityLog *auth.SecurityLog
	Secrets     *auth.OneTimeTokens
	Links       auth.Links
	Actions     *Actions
	Channels    *channels.Manager

	WorkspaceInitializers *workspaces.Initializers
	Retention             contracts.RetentionGuard
	Entitlements          contracts.Entitlements
	BillingStatus         contracts.BillingStatus
	EmailChannel          *email.Channel

	Realtime *appbroadcasting.Connections

	OAuthAttempts auth.OAuthAttempts
	Frontend      fs.FS
	Vite          *vite.Bundle
	Widget        fs.FS
	WidgetAssets  *widgetassets.Assets
	HelpSample    *helpsample.Library
}

/*
New returns the application before boot, with its configuration empty
until the framework loads it, the full permission catalog, the login
policy that allows every method, the access policy of every rule a
workspace may set, no first-party app grants, no identity providers or
workspace initializers yet, and no retention holds.
*/
func New(version string) *App {
	return &App{
		Version:      version,
		Config:       &appconfig.Config{},
		Permissions:  auth.NewCatalog(),
		LoginPolicy:  auth.AllowAll{},
		AccessPolicy: auth.DefaultAccessPolicy(),
		AppGrants:    auth.NoAppGrants{},

		IdentityProviders: providers.NewRegistry(),

		WorkspaceInitializers: workspaces.NewInitializers(),
		Retention:             retention.NoHolds{},
	}
}

/*
ConfigureFramework adjusts the framework's configuration to the kind of
install, once both are loaded.

Mail goes out from the product's name at the install's own host unless
the operator named a sender, so a fresh install's mail is never refused
for having none. Mail that is only written to the log must still reach
people on a self-hosted install without a mail server: there each line
is a warning carrying every link, so the operator can pass sign-in and
invitation links on by hand. On the cloud a line never carries a link,
since there the log is no place for anyone's secrets.

Each identity provider with a client id becomes a social login provider
of the same name, see configureSocialLogin.
*/
func (a *App) ConfigureFramework(cfg *foundation.Config) {
	a.configureSocialLogin(cfg)

	if cfg.Mail.From.Email == "" {
		cfg.Mail.From = mail.Address{Name: cmp.Or(cfg.Mail.From.Name, "Dimmi"), Email: "no-reply@" + host(cfg.App.URL)}
	}

	mailer := cfg.Mail.Mailers[cfg.Mail.Default]
	if mailer.String("driver") != "log" {
		return
	}

	if a.Config.App.Edition == appconfig.EditionCloud {
		mailer["content"] = "envelope"

		return
	}

	mailer["level"] = cmp.Or(mailer.String("level"), "warn")
	mailer["content"] = cmp.Or(mailer.String("content"), "links")
}

/*
host returns the host name of the application's URL. The framework
refuses to boot with a URL that does not parse, so the error is never
set here.
*/
func host(appURL string) string {
	u, _ := url.Parse(appURL)

	return u.Hostname()
}

/*
MailConfigured reports whether mail leaves the server, rather than only
being written to the log.
*/
func (a *App) MailConfigured() bool {
	cfg := a.Application.Config.Mail

	return cfg.Mailers[cfg.Default].String("driver") != "log"
}

/*
SecureCookies reports whether the application is served over HTTPS,
where cookies may carry the Secure flag and the __Host- prefix.
*/
func (a *App) SecureCookies() bool {
	return strings.HasPrefix(a.Application.Config.App.URL, "https://")
}

/*
configureSocialLogin hands the framework's social login every identity
provider configured under auth.oauth: its driver is its key, its
credentials come from auth.oauth, and the provider sends people back to
its callback route. Options only a driver knows, such as an issuer,
stay under socialite.providers and win over these defaults.
*/
func (a *App) configureSocialLogin(cfg *foundation.Config) {
	if cfg.Socialite.Providers == nil {
		cfg.Socialite.Providers = map[string]config.Section{}
	}

	for key, provider := range a.Config.Auth.OAuth {
		if provider.ClientID == "" {
			continue
		}

		section := config.Section{
			"driver":        key,
			"client_id":     provider.ClientID,
			"client_secret": provider.ClientSecret,
			"redirect_url":  strings.TrimRight(cfg.App.URL, "/") + "/auth/" + key + "/callback",
		}
		maps.Copy(section, cfg.Socialite.Providers[key])
		cfg.Socialite.Providers[key] = section
	}
}
