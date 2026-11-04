/*
|--------------------------------------------------------------------------
| Second Factor Settings
|--------------------------------------------------------------------------
|
| Which kinds of second factor users may enroll and how each presents
| the install: the name authenticator apps list it under and the name
| devices show when a passkey is made or used.
|
*/

package config

import "time"

/*
FactorsConfig lists the second-factor types users may enroll.
MaxPerUser caps the active factors of one user, the recovery code set
included. PendingTTL is how long an enrollment may stay unfinished
before it counts as gone. TOTP names the install in authenticator apps
and Passkey on passkey prompts.
*/
type FactorsConfig struct {
	Enabled    []string            `default:"[\"totp\",\"passkey\",\"recovery_code\"]" koanf:"enabled"`
	MaxPerUser int                 `default:"10"                                   koanf:"max_per_user"`
	PendingTTL time.Duration       `default:"1h"                                   koanf:"pending_ttl"`
	TOTP       TOTPFactorConfig    `koanf:"totp"`
	Passkey    PasskeyFactorConfig `koanf:"passkey"`
}

/*
TOTPFactorConfig configures authenticator apps. Issuer is the name an
app lists the account under.
*/
type TOTPFactorConfig struct {
	Issuer string `default:"dimmi" koanf:"issuer"`
}

/*
PasskeyFactorConfig configures passkeys. Name is the install's name some
devices show when a passkey is made or used. The domain passkeys are
bound to is the host of app.url, and they answer from that origin or a
trusted one.
*/
type PasskeyFactorConfig struct {
	Name string `default:"dimmi" koanf:"name"`
}
