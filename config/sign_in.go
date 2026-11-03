/*
|--------------------------------------------------------------------------
| Sign-In Method Settings
|--------------------------------------------------------------------------
|
| Which ways of signing in beyond a password the install offers: links
| by mail, second factors, and sign-in providers, each offered only once
| it is configured.
|
*/

package config

import "time"

/*
MagicLinkConfig turns sign-in links by mail on or off. TTL is how long a
link and its code work after they were sent.
*/
type MagicLinkConfig struct {
	Enabled bool          `default:"true" koanf:"enabled"`
	TTL     time.Duration `default:"15m"  koanf:"ttl"`
}

/*
VerificationConfig says whether a user must prove their email before
the steps that reach other people, such as inviting them or minting
tokens. Required is unset until the edition settles it: the cloud
requires it, a self-hosted install does not.
*/
type VerificationConfig struct {
	Required *bool `koanf:"required"`
}

/*
FactorsConfig lists the second-factor types users may enroll.
MaxPerUser caps the active factors of one user, the recovery code set
included. PendingTTL is how long an enrollment may stay unfinished
before it counts as gone. TOTP names the install in authenticator apps.
*/
type FactorsConfig struct {
	Enabled    []string         `default:"[\"totp\",\"recovery_code\"]" koanf:"enabled"`
	MaxPerUser int              `default:"10"                         koanf:"max_per_user"`
	PendingTTL time.Duration    `default:"1h"                         koanf:"pending_ttl"`
	TOTP       TOTPFactorConfig `koanf:"totp"`
}

/*
TOTPFactorConfig configures authenticator apps. Issuer is the name an
app lists the account under.
*/
type TOTPFactorConfig struct {
	Issuer string `default:"dimmi" koanf:"issuer"`
}

/*
OAuthProviderConfig configures one sign-in provider. A provider without
a client id is not offered.
*/
type OAuthProviderConfig struct {
	ClientID     string `koanf:"client_id"`
	ClientSecret string `koanf:"client_secret" secret:"true"`
}
