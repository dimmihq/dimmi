/*
|--------------------------------------------------------------------------
| Series Settings
|--------------------------------------------------------------------------
|
| A person can be in many series at once, so the limits that keep them
| from being flooded hold across all of them. Messages over the daily
| cap wait for the next day; they are never dropped. Quiet hours keep
| email to the day in the person's own zone.
|
*/

package config

/*
SeriesConfig configures series. MaxMessagesPerDay is how many series
messages one contact gets a day across every series. QuietStart and
QuietEnd bound the hours, in the contact's zone, email may leave.
SweepMinutes is how often series with rules look for people who began
to match as time passed. MaxStepsPerEnrollment ends a journey that
takes more steps than any real series needs. AITimeoutSeconds bounds
the wait for an AI path. WebhookTimeoutSeconds bounds a webhook call.
*/
type SeriesConfig struct {
	MaxMessagesPerDay     int    `default:"2"     koanf:"max_messages_per_day"`
	QuietStart            string `default:"08:00" koanf:"quiet_start"`
	QuietEnd              string `default:"20:00" koanf:"quiet_end"`
	SweepMinutes          int    `default:"15"    koanf:"sweep_minutes"`
	MaxStepsPerEnrollment int    `default:"500"   koanf:"max_steps_per_enrollment"`
	AITimeoutSeconds      int    `default:"20"    koanf:"ai_timeout_seconds"`
	WebhookTimeoutSeconds int    `default:"10"    koanf:"webhook_timeout_seconds"`
}
