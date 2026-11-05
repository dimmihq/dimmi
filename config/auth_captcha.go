/*
|--------------------------------------------------------------------------
| Captcha Settings
|--------------------------------------------------------------------------
|
| A public server can ask people signing up or mailing themselves a
| sign-in link to prove they are not a script. Turnstile is the one
| provider; with no driver nothing is asked. Its site key goes to the
| browser, its secret only to the provider. A provider that does not
| answer within Timeout counts as down.
|
*/

package config

import (
	"fmt"
	"time"
)

/*
The captcha drivers.
*/
const (
	CaptchaNone      = "none"
	CaptchaTurnstile = "turnstile"
)

/*
CaptchaConfig chooses the captcha provider. Driver is none or turnstile;
SiteKey and SecretKey are the provider's keys; VerifyURL is where
answers are checked; Timeout bounds each check.
*/
type CaptchaConfig struct {
	Driver    string        `default:"none"                                                      koanf:"driver"`
	SiteKey   string        `koanf:"site_key"`
	SecretKey string        `koanf:"secret_key" secret:"true"`
	VerifyURL string        `default:"https://challenges.cloudflare.com/turnstile/v0/siteverify" koanf:"verify_url"`
	Timeout   time.Duration `default:"3s"                                                        koanf:"timeout"`
}

/*
check reports a captcha setting that cannot work: an unknown driver, or
Turnstile without its keys.
*/
func (c CaptchaConfig) check() error {
	known := c.Driver == CaptchaNone || c.Driver == CaptchaTurnstile
	keyed := c.Driver != CaptchaTurnstile || (c.SiteKey != "" && c.SecretKey != "")

	if !known || !keyed || c.Timeout <= 0 {
		return fmt.Errorf("%w: auth.captcha", ErrInvalid)
	}

	return nil
}
