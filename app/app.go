/*
|--------------------------------------------------------------------------
| Application
|--------------------------------------------------------------------------
|
| The product's application is the framework's, plus what only this
| product has: its typed configuration, the permission catalog, the
| sign-in policy and lockout, and the business actions, each built once
| with exactly the services it uses. Nothing here is looked up by name:
| providers fill the fields while the application boots, and everything
| after boot receives what it needs through its constructor.
|
| The application is created before the framework boots it, so tests can
| swap a field such as the login policy before any action is built.
|
*/

package app

import (
	"cmp"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/config"
)

/*
App is the booted product.

Version is the release the binary was built as. Config is the
application's own configuration; the framework's stays on
Application.Config. Permissions is the permission catalog and
LoginPolicy decides which sign-in methods an address may use. AppGrants
ends the user's first-party app grants. Tokens, Devices, Cookies,
Lockout, SecurityLog, Secrets and Links are the sign-in services
providers build at boot, and Actions every business action.
*/
type App struct {
	*foundation.Application

	Version     string
	Config      *config.Config
	Permissions *auth.Catalog
	LoginPolicy auth.LoginPolicyResolver
	AppGrants   auth.AppGrants
	Tokens      *auth.PersonalTokens
	Devices     *auth.Devices
	Cookies     auth.Cookies
	Lockout     *auth.Lockout
	SecurityLog *auth.SecurityLog
	Secrets     *auth.OneTimeTokens
	Links       auth.Links
	Actions     *Actions
}

/*
New returns the application before boot, with its configuration empty
until the framework loads it, the full permission catalog, the login
policy that allows every method, and no first-party app grants.
*/
func New(version string) *App {
	return &App{
		Version:     version,
		Config:      &config.Config{},
		Permissions: auth.NewCatalog(),
		LoginPolicy: auth.AllowAll{},
		AppGrants:   auth.NoAppGrants{},
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
*/
func (a *App) ConfigureFramework(cfg *foundation.Config) {
	if cfg.Mail.From.Email == "" {
		cfg.Mail.From = mail.Address{Name: cmp.Or(cfg.Mail.From.Name, "Dimmi"), Email: "no-reply@" + host(cfg.App.URL)}
	}

	mailer := cfg.Mail.Mailers[cfg.Mail.Default]
	if mailer.String("driver") != "log" {
		return
	}

	if a.Config.App.Edition == config.EditionCloud {
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
