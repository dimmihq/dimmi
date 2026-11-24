/*
|--------------------------------------------------------------------------
| Authentication Settings
|--------------------------------------------------------------------------
|
| How people sign up and sign in: who may create an account, how
| passwords are judged, how long a session lasts and how hard repeated
| failures are pushed back. A self-hosted install starts closed, since
| its operator invites the team; the cloud is open to anyone.
|
*/

package config

import (
	"fmt"
	"slices"
	"strings"
)

/*
The signup modes. In open mode anyone may create an account; in
invite_only mode only someone holding an invitation for their own
address may.
*/
const (
	SignupOpen       = "open"
	SignupInviteOnly = "invite_only"
)

/*
The ways to treat a signup from a throwaway mailbox: let it through,
let it through marked, or refuse it.
*/
const (
	DisposableOff   = "off"
	DisposableFlag  = "flag"
	DisposableBlock = "block"
)

/*
AuthConfig groups the authentication settings. DisposableEmail is off,
flag or block: how a signup from a throwaway mailbox is treated. Terms
is the legal terms a person accepts when creating an account.
*/
type AuthConfig struct {
	Signup               SignupConfig                   `koanf:"signup"`
	Passwords            PasswordsConfig                `koanf:"passwords"`
	Sessions             SessionsConfig                 `koanf:"sessions"`
	Lockout              LockoutConfig                  `koanf:"lockout"`
	MagicLink            MagicLinkConfig                `koanf:"magic_link"`
	Verification         VerificationConfig             `koanf:"verification"`
	PasswordConfirmation PasswordConfirmationConfig     `koanf:"password_confirmation"`
	Factors              FactorsConfig                  `koanf:"factors"`
	OAuth                map[string]OAuthProviderConfig `koanf:"oauth"`
	Captcha              CaptchaConfig                  `koanf:"captcha"`
	DisposableEmail      string                         `koanf:"disposable_email"`
	Terms                TermsConfig                    `koanf:"terms"`
}

/*
cloudLegalSite is the site that publishes dimmi cloud's legal pages.
*/
const cloudLegalSite = "https://trydimmi.com"

/*
TermsConfig describes the legal terms a person accepts when creating an
account. SiteURL is the site that publishes them, at /terms and
/privacy; with none, nothing is asked. Version is the version in force,
recorded on each account that accepts it.
*/
type TermsConfig struct {
	SiteURL string `koanf:"site_url"`
	Version string `default:"1.0" koanf:"version"`
}

/*
SignupConfig says who may create an account. Mode is open or
invite_only. MaxPerHourPerIP caps the accounts one address creates in an
hour, to slow down scripted signups.
*/
type SignupConfig struct {
	Mode            string `koanf:"mode"`
	MaxPerHourPerIP int    `koanf:"max_per_hour_per_ip"`
}

/*
PasswordsConfig judges new passwords. MinLength counts characters, not
bytes.
*/
type PasswordsConfig struct {
	MinLength int `default:"10" koanf:"min_length"`
}

/*
settle fills the defaults that depend on the edition.
*/
func (c *AuthConfig) settle(edition Edition) {
	cloud := edition == EditionCloud

	if c.Signup.Mode == "" {
		c.Signup.Mode = map[bool]string{true: SignupOpen, false: SignupInviteOnly}[cloud]
	}

	if c.Signup.MaxPerHourPerIP == 0 {
		c.Signup.MaxPerHourPerIP = map[bool]int{true: 5, false: 20}[cloud]
	}

	if c.DisposableEmail == "" {
		c.DisposableEmail = map[bool]string{true: DisposableBlock, false: DisposableOff}[cloud]
	}

	if c.Verification.Required == nil {
		c.Verification.Required = &cloud
	}

	if cloud && c.Terms.SiteURL == "" {
		c.Terms.SiteURL = cloudLegalSite
	}

	c.Terms.SiteURL = strings.TrimRight(strings.TrimSpace(c.Terms.SiteURL), "/")
}

/*
webAddress reports whether the address opens in a browser.
*/
func webAddress(address string) bool {
	return strings.HasPrefix(address, "https://") || strings.HasPrefix(address, "http://")
}

/*
check reports the first authentication setting that cannot work.
*/
func (c *AuthConfig) check() error {
	problems := []struct {
		found bool
		key   string
	}{
		{c.Signup.Mode != SignupOpen && c.Signup.Mode != SignupInviteOnly, "auth.signup.mode"},
		{c.Signup.MaxPerHourPerIP < 0, "auth.signup.max_per_hour_per_ip"},
		{
			!slices.Contains([]string{DisposableOff, DisposableFlag, DisposableBlock}, c.DisposableEmail),
			"auth.disposable_email",
		},
		{c.Passwords.MinLength < 8, "auth.passwords.min_length"},
		{c.Sessions.IdleLifetime <= 0, "auth.sessions.idle_lifetime"},
		{c.Sessions.AbsoluteLifetime < c.Sessions.IdleLifetime, "auth.sessions.absolute_lifetime"},
		{c.Lockout.PerIPEmail <= 0 || c.Lockout.PerEmailSoftLock <= 0 || c.Lockout.PerIPSpray <= 0, "auth.lockout"},
		{c.MagicLink.TTL <= 0, "auth.magic_link.ttl"},
		{c.Factors.MaxPerUser < 2, "auth.factors.max_per_user"},
		{c.Factors.PendingTTL <= 0, "auth.factors.pending_ttl"},
		{c.Terms.Version == "" || len(c.Terms.Version) > 40, "auth.terms.version"},
		{c.Terms.SiteURL != "" && !webAddress(c.Terms.SiteURL), "auth.terms.site_url"},
	}

	for _, p := range problems {
		if p.found {
			return fmt.Errorf("%w: %s", ErrInvalid, p.key)
		}
	}

	return c.Captcha.check()
}
