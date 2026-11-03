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

import "fmt"

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
AuthConfig groups the authentication settings.
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

	if c.Verification.Required == nil {
		c.Verification.Required = &cloud
	}
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
		{c.Passwords.MinLength < 8, "auth.passwords.min_length"},
		{c.Sessions.IdleLifetime <= 0, "auth.sessions.idle_lifetime"},
		{c.Sessions.AbsoluteLifetime < c.Sessions.IdleLifetime, "auth.sessions.absolute_lifetime"},
		{c.Lockout.PerIPEmail <= 0 || c.Lockout.PerEmailSoftLock <= 0 || c.Lockout.PerIPSpray <= 0, "auth.lockout"},
		{c.MagicLink.TTL <= 0, "auth.magic_link.ttl"},
	}

	for _, p := range problems {
		if p.found {
			return fmt.Errorf("%w: %s", ErrInvalid, p.key)
		}
	}

	return nil
}
