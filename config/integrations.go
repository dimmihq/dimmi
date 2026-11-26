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
| App credentials are secrets: set them in the environment, such as
| DIMMI_SERVICES__SLACK__CLIENT_SECRET, or name a file that holds the
| value with DIMMI_SERVICES__SLACK__CLIENT_SECRET_FILE.
|
*/

package config

import (
	"fmt"
	"time"
)

/*
ServicesConfig holds the credentials of the apps dimmi has registered at
other services. Slack is the shared Slack app, WhatsApp the Meta app
WhatsApp numbers connect through, Meta the same app for Messenger and
Instagram with each one's login; Notion and GitHub are the OAuth
clients knowledge sources connect through.
*/
type ServicesConfig struct {
	Slack     SlackServiceConfig    `koanf:"slack"`
	WhatsApp  WhatsAppServiceConfig `koanf:"whatsapp"`
	Meta      MetaServiceConfig     `koanf:"meta"`
	Facebook  SocialServiceConfig   `koanf:"facebook"`
	Instagram SocialServiceConfig   `koanf:"instagram"`
	Notion    OAuthServiceConfig    `koanf:"notion"`
	GitHub    OAuthServiceConfig    `koanf:"github"`
	Linear    TrackerServiceConfig  `koanf:"linear"`
	Jira      TrackerServiceConfig  `koanf:"jira"`
	Trackers  TrackersConfig        `koanf:"trackers"`
}

/*
OAuthServiceConfig is an OAuth client registered at a service: its id
and secret. Empty means the service cannot be connected.
*/
type OAuthServiceConfig struct {
	ClientID     string `koanf:"client_id"`
	ClientSecret string `koanf:"client_secret" secret:"true"`
}

/*
SlackServiceConfig is the shared Slack app: its OAuth client id and
secret, the signing secret its events are signed with and its app id.
Empty means the shared app is unavailable.
*/
type SlackServiceConfig struct {
	ClientID      string `koanf:"client_id"`
	ClientSecret  string `koanf:"client_secret"  secret:"true"`
	SigningSecret string `koanf:"signing_secret" secret:"true"`
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
WhatsAppServiceConfig is the install's own Meta app, the one WhatsApp
numbers connect through: its id, the secret Meta signs every delivery
with, the Embedded Signup configuration the connect popup opens, and
the token Meta must echo when it checks the webhook address. Without an
app id and secret the popup is unavailable, and a workspace connects
its number with a token of its own Meta app instead.
*/
type WhatsAppServiceConfig struct {
	AppID       string `koanf:"app_id"`
	AppSecret   string `koanf:"app_secret"   secret:"true"`
	ConfigID    string `koanf:"config_id"`
	VerifyToken string `koanf:"verify_token" secret:"true"`
}

/*
Configured reports whether numbers can connect through the install's
own Meta app.
*/
func (c WhatsAppServiceConfig) Configured() bool {
	return c.AppID != "" && c.AppSecret != ""
}

/*
SignupReady reports whether the Embedded Signup popup can open.
*/
func (c WhatsAppServiceConfig) SignupReady() bool {
	return c.Configured() && c.ConfigID != ""
}

/*
IntegrationsConfig sets how integrations behave. Enabled lists the
providers this install offers. RefreshSkew is how long before expiry a
token is renewed, and HealthInterval how often each installation is
checked.
*/
type IntegrationsConfig struct {
	Enabled        []string      `default:"[\"slack\",\"notion\",\"github\",\"linear\",\"jira\",\"incident_io\",\"rootly\",\"pagerduty\"]" koanf:"enabled"` //nolint:lll // one default list
	RefreshSkew    time.Duration `default:"5m"          koanf:"refresh_skew"`
	HealthInterval time.Duration `default:"24h"         koanf:"health_interval"`
}
