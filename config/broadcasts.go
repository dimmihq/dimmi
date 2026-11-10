/*
|--------------------------------------------------------------------------
| Broadcast Settings
|--------------------------------------------------------------------------
|
| How fast broadcasts leave and when they stop themselves. Mailbox
| providers punish sudden volume from an address, so email is paced per
| address; Slack allows about one post a second per channel. A send that
| bounces or draws complaints past the limits pauses before it harms the
| address's reputation.
|
*/

package config

/*
BroadcastsConfig configures broadcasts. EmailPerMinute is the most
emails one address sends a minute, MessengerPerMinute and SlackPerMinute
the same for Messenger conversations and Slack posts. MinGapHours is the
least time between two broadcasts to one person, unless a broadcast
opts out. PauseAfter is how many sent emails the bounce and complaint
rates wait for, BounceRate and ComplaintRate the shares that pause a
send. LeaseSeconds is how long a recipient handed to a sender stays
out of the queue before it is tried again, and BatchSize how many
recipients one dispatch hands out. DispatchSeconds is how often a
sending broadcast hands out recipients.
*/
type BroadcastsConfig struct {
	EmailPerMinute     int     `default:"600"   koanf:"email_per_minute"`
	MessengerPerMinute int     `default:"1200"  koanf:"messenger_per_minute"`
	SlackPerMinute     int     `default:"50"    koanf:"slack_per_minute"`
	MinGapHours        int     `default:"72"    koanf:"min_gap_hours"`
	PauseAfter         int     `default:"100"   koanf:"pause_after"`
	BounceRate         float64 `default:"0.05"  koanf:"bounce_rate"`
	ComplaintRate      float64 `default:"0.003" koanf:"complaint_rate"`
	LeaseSeconds       int     `default:"600"   koanf:"lease_seconds"`
	BatchSize          int     `default:"200"   koanf:"batch_size"`
	DispatchSeconds    int     `default:"10"    koanf:"dispatch_seconds"`
}
