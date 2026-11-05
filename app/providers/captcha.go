/*
|--------------------------------------------------------------------------
| Captcha Provider
|--------------------------------------------------------------------------
|
| The captcha the server asks strangers to solve, chosen by
| configuration: Turnstile with its secret key, or nothing at all.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/support/turnstile"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
captchaFor returns the captcha the configuration names.
*/
func captchaFor(a *app.App) appauth.Captcha {
	c := a.Config.Auth.Captcha
	if c.Driver != appconfig.CaptchaTurnstile {
		return appauth.NoCaptcha{}
	}

	return &turnstile.Client{HTTP: a.HTTP, Secret: c.SecretKey, VerifyURL: c.VerifyURL, Timeout: c.Timeout}
}
