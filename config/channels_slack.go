/*
|--------------------------------------------------------------------------
| Slack Channel Settings
|--------------------------------------------------------------------------
|
| How the Slack channel behaves on this install. Slack retries a
| delivery for about an hour, so a delivery seen within that window is
| known again without a database read. A long-lived socket is held by
| one replica under a lease it renews, and another takes it over once
| the holder stops renewing. A customer's top-level messages within the
| group window join the conversation their first one opened while
| nobody answered it yet, and a short message of thanks never opens one.
|
| Once a workspace's Slack app is uninstalled, its tokens are revoked or
| a teammate disconnects it, what Slack brought in is deleted after a
| grace period, so connecting again within it loses nothing. Slack
| requires the deletion within 14 business days, so the period is at
| most 14 days.
|
*/

package config

import "time"

/*
SlackChannelConfig configures the Slack channel.

APIURL is the Web API base, Slack's own unless a test or a proxy needs
another. MaxPayloadBytes bounds one delivery and DedupeWindow is how
long a delivered event is recognized quickly. GroupWindow joins a
customer's top-level messages into one conversation. PleasantryMaxChars
is the longest message a pleasantry may be. TriggerEmoji opens a
conversation in an emoji room, ClosedMarker marks a closed one and
WorkingMarker shows the agent is writing. LeaseTTL and RenewInterval
time the socket leases. DataDeletionGraceDays is how many days after a
connection ended its Slack data is deleted, 14 at most.
*/
type SlackChannelConfig struct {
	APIURL             string        `default:"https://slack.com/api/" koanf:"api_url"`
	MaxPayloadBytes    int64         `default:"1048576"                koanf:"max_payload_bytes"`
	DedupeWindow       time.Duration `default:"1h"                     koanf:"dedupe_window"`
	GroupWindow        time.Duration `default:"120s"                   koanf:"group_window"`
	PleasantryMaxChars int           `default:"40"                     koanf:"pleasantry_max_chars"`
	TriggerEmoji       string        `default:"ticket"                 koanf:"trigger_emoji"`
	ClosedMarker       string        `default:"white_check_mark"       koanf:"closed_marker"`
	WorkingMarker      string        `default:"eyes"                   koanf:"working_marker"`
	LeaseTTL           time.Duration `default:"15s"                    koanf:"lease_ttl"`
	RenewInterval      time.Duration `default:"5s"                     koanf:"renew_interval"`

	DataDeletionGraceDays int `default:"10" koanf:"data_deletion_grace_days"`
}

/*
MaxSlackDataDeletionGraceDays is the longest grace period Slack's
deletion deadline leaves room for.
*/
const MaxSlackDataDeletionGraceDays = 14
