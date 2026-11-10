/*
|--------------------------------------------------------------------------
| Integration Settings
|--------------------------------------------------------------------------
|
| A workspace connects the tools its team already uses, Slack first, with
| one installation per tool account that every feature shares. The
| install offers only the providers listed as enabled, and only those
| whose app credentials are set: a self-hosted install without a shared
| Slack app still connects Slack from tokens of its own custom app.
|
| Tokens close to expiry are renewed when used, a little ahead of time,
| and every installation is checked against its provider once a day.
|
| App credentials are secrets: set them from files, such as
| DIMMI_SERVICES__SLACK__CLIENT_SECRET_FILE, rather than in dimmi.yaml.
|
*/

package config

import (
	"fmt"
	"time"
)

/*
ServicesConfig holds the credentials of the apps dimmi has registered at
other services. Slack is the shared Slack app.
*/
type ServicesConfig struct {
	Slack SlackServiceConfig `koanf:"slack"`
}

/*
SlackServiceConfig is the shared Slack app: its OAuth client id and
secret, the signing secret its events are signed with and its app id.
Empty means the shared app is unavailable.
*/
type SlackServiceConfig struct {
	ClientID      string `koanf:"client_id"`
	ClientSecret  string `koanf:"client_secret"`
	SigningSecret string `koanf:"signing_secret"`
	AppID         string `koanf:"app_id"`
}

/*
Configured reports whether the shared Slack app can install.
*/
func (c SlackServiceConfig) Configured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

/*
check refuses a shared app that could install but never receive: Slack
signs every event, and without the signing secret each one is turned
away.
*/
func (c SlackServiceConfig) check() error {
	if c.Configured() && c.SigningSecret == "" {
		return fmt.Errorf("%w: services.slack.signing_secret", ErrInvalid)
	}

	return nil
}

/*
IntegrationsConfig sets how integrations behave. Enabled lists the
providers this install offers. RefreshSkew is how long before expiry a
token is renewed, and HealthInterval how often each installation is
checked.
*/
type IntegrationsConfig struct {
	Enabled        []string      `default:"[\"slack\"]" koanf:"enabled"`
	RefreshSkew    time.Duration `default:"5m"          koanf:"refresh_skew"`
	HealthInterval time.Duration `default:"24h"         koanf:"health_interval"`
}
