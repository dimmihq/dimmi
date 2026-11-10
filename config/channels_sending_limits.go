/*
|--------------------------------------------------------------------------
| Email Sending Limits and Checks
|--------------------------------------------------------------------------
|
| The caps and pause rates that keep one workspace from hurting the
| reputation every other workspace sends on, the automatic DNS setup some
| hosts offer, and the scheduled proof that mail really goes both ways.
|
*/

package config

/*
EmailLimitsConfig holds sending caps. The daily and hourly ones are what a
self-hosted install uses for the matching entitlements, zero meaning none.
The two per-address ones protect recipients on every install, so zero
there means the default. ProviderPerMinute and ProviderPerDay are the
sending quota the install's mail provider grants, such as an SES
account's, zero meaning none; every broadcast of every workspace
shares BroadcastSharePercent of it, so replies always keep room.
*/
type EmailLimitsConfig struct {
	VerifiedDaily                int `koanf:"verified_daily"`
	UnverifiedDaily              int `koanf:"unverified_daily"`
	NewWorkspaceHourly           int `koanf:"new_workspace_hourly"`
	UnverifiedRelayWorkspaceDay  int `koanf:"unverified_relay_workspace_daily"`
	UnverifiedRelayPerAddressDay int `koanf:"unverified_relay_per_address_daily"`
	AutomatedPerRecipientHourly  int `koanf:"automated_per_recipient_hourly"`
	ProviderPerMinute            int `koanf:"provider_per_minute"`
	ProviderPerDay               int `koanf:"provider_per_day"`
	BroadcastSharePercent        int `default:"80"                              koanf:"broadcast_share_percent"`
}

/*
EmailReputation holds the rates that pause a workspace's sending. Zero turns
a rate off.
*/
type EmailReputation struct {
	BouncePauseRate    float64 `koanf:"bounce_pause_rate"`
	ComplaintPauseRate float64 `koanf:"complaint_pause_rate"`
}

/*
DomainConnectConfig configures automatic DNS setup at hosts that support it:
the identifiers of our published template, the key that signs a request and
the TXT host that publishes its public half.
*/
type DomainConnectConfig struct {
	Enabled    bool   `koanf:"enabled"`
	ProviderID string `koanf:"provider_id"`
	ServiceID  string `koanf:"service_id"`
	PrivateKey string `koanf:"private_key" secret:"true"`
	KeyHost    string `koanf:"key_host"`
}

/*
RoundTripConfig configures the scheduled proof that mail really goes both
ways. Workspace is the slug of the workspace it runs in, Simulator whether it
also mails SES's simulator. The alert goes to the webhook and each success
pings the heartbeat.
*/
type RoundTripConfig struct {
	Enabled         bool   `koanf:"enabled"`
	Workspace       string `koanf:"workspace"`
	Simulator       bool   `koanf:"simulator"`
	AlertWebhookURL string `koanf:"alert_webhook_url" secret:"true"`
	HeartbeatURL    string `koanf:"heartbeat_url"     secret:"true"`
}
