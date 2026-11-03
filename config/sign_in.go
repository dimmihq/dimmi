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
*/
type FactorsConfig struct {
	Enabled []string `default:"[\"totp\",\"recovery_code\"]" koanf:"enabled"`
}

/*
OAuthProviderConfig configures one sign-in provider. A provider without
a client id is not offered.
*/
type OAuthProviderConfig struct {
	ClientID     string `koanf:"client_id"`
	ClientSecret string `koanf:"client_secret" secret:"true"`
}
