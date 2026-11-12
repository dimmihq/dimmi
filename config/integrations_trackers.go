/*
|--------------------------------------------------------------------------
| Engineering Tool Settings
|--------------------------------------------------------------------------
|
| Linear and Jira are OAuth apps dimmi registered: their client id and
| secret, and the secret their webhooks are signed with (Linear's app
| webhook secret; Jira signs with the client secret, so it needs none).
| GitHub issues reuse the GitHub OAuth app; repositories dimmi links get
| a webhook signed with the GitHub webhook secret. incident.io, Rootly
| and PagerDuty connect with each workspace's own API key and sign with
| each endpoint's own secret, so they need nothing here.
|
| Secrets come from files, such as DIMMI_SERVICES__LINEAR__CLIENT_SECRET_FILE.
|
*/

package config

/*
TrackerServiceConfig is an engineering tool's OAuth app and the secret
its app webhooks are signed with.
*/
type TrackerServiceConfig struct {
	ClientID      string `koanf:"client_id"`
	ClientSecret  string `koanf:"client_secret"`
	WebhookSecret string `koanf:"webhook_secret"`
}

/*
Configured reports whether the app can install.
*/
func (c TrackerServiceConfig) Configured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

/*
TrackersConfig holds what linked issues need beyond the apps: the
secret GitHub repository webhooks are signed with. Fake answers every
tool from canned data instead of calling it, and only a development
build honors it.
*/
type TrackersConfig struct {
	GitHubWebhookSecret string `koanf:"github_webhook_secret"`
	Fake                bool   `koanf:"fake"`
}
