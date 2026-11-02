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
	"strings"

	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/config"
)

/*
App is the booted product.

Version is the release the binary was built as. Config is the
application's own configuration; the framework's stays on
Application.Config. Permissions is the permission catalog and
LoginPolicy decides which sign-in methods an address may use. Tokens,
Devices, Cookies and Lockout are the sign-in services providers build at
boot, and Actions every business action.
*/
type App struct {
	*foundation.Application

	Version     string
	Config      *config.Config
	Permissions *auth.Catalog
	LoginPolicy auth.LoginPolicyResolver
	Tokens      *auth.PersonalTokens
	Devices     *auth.Devices
	Cookies     auth.Cookies
	Lockout     *auth.Lockout
	SecurityLog *auth.SecurityLog
	Actions     *Actions
}

/*
New returns the application before boot, with its configuration empty
until the framework loads it, the full permission catalog and the login
policy that allows every method.
*/
func New(version string) *App {
	return &App{
		Version:     version,
		Config:      &config.Config{},
		Permissions: auth.NewCatalog(),
		LoginPolicy: auth.AllowAll{},
	}
}

/*
MailConfigured reports whether mail leaves the server, rather than only
being written to the log.
*/
func (a *App) MailConfigured() bool {
	mail := a.Application.Config.Mail

	return mail.Mailers[mail.Default].String("driver") != "log"
}

/*
SecureCookies reports whether the application is served over HTTPS,
where cookies may carry the Secure flag and the __Host- prefix.
*/
func (a *App) SecureCookies() bool {
	return strings.HasPrefix(a.Application.Config.App.URL, "https://")
}
