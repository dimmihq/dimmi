/*
|--------------------------------------------------------------------------
| Session and Sign-In Settings
|--------------------------------------------------------------------------
|
| How long a browser stays signed in, how failed sign-ins are pushed
| back, and how recent a sign-in must be for sensitive steps.
|
*/

package config

import "time"

/*
SessionsConfig bounds browser sessions. A session nobody used for
IdleLifetime ends, and any session ends AbsoluteLifetime after login.
Cookie names the session cookie; served over HTTPS it gets the __Host-
prefix.
*/
type SessionsConfig struct {
	IdleLifetime     time.Duration `default:"336h"          koanf:"idle_lifetime"`
	AbsoluteLifetime time.Duration `default:"720h"          koanf:"absolute_lifetime"`
	Cookie           string        `default:"dimmi_session" koanf:"cookie"`
	DeviceCookie     string        `default:"dimmi_device"  koanf:"device_cookie"`
}

/*
LockoutConfig sets how failed sign-ins are pushed back. PerIPEmail caps
attempts per minute for one address at one email. PerEmailSoftLock is
the failures per hour at one email, from anywhere, that lock it for
unknown devices. PerIPSpray is the failures per hour from one address,
at any email, that block it.
*/
type LockoutConfig struct {
	PerIPEmail       int `default:"10"  koanf:"per_ip_email"`
	PerEmailSoftLock int `default:"30"  koanf:"per_email_soft_lock"`
	PerIPSpray       int `default:"100" koanf:"per_ip_spray"`
}

/*
PasswordConfirmationConfig sets TTL, how long after proving who they are
a user may take sensitive steps without proving it again.
*/
type PasswordConfirmationConfig struct {
	TTL time.Duration `default:"15m" koanf:"ttl"`
}
