/*
|--------------------------------------------------------------------------
| Signup Guards
|--------------------------------------------------------------------------
|
| What stands between strangers and a new account: the captcha the
| server asks them to solve, Turnstile or nothing, and the judgement of
| a signup's risk, built on the embedded list of throwaway mailboxes.
|
*/

package providers

import (
	"fmt"

	"github.com/dimmihq/dimmi/app"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/support/turnstile"
	appconfig "github.com/dimmihq/dimmi/config"
	"github.com/dimmihq/dimmi/resources/data"
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

/*
signupRisk returns the judge of signup risk, with the throwaway domains
loaded once.
*/
func signupRisk(a *app.App) (*appauth.SignupRisk, error) {
	domains, err := data.DisposableDomains()
	if err != nil {
		return nil, fmt.Errorf("load disposable domains: %w", err)
	}

	return &appauth.SignupRisk{Disposable: domains, Mode: a.Config.Auth.DisposableEmail, Limiter: a.Limiter}, nil
}
