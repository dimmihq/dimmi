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

/*
MagicLinkConfig turns sign-in links by mail on or off.
*/
type MagicLinkConfig struct {
	Enabled bool `default:"true" koanf:"enabled"`
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
