/*
|--------------------------------------------------------------------------
| Channel Delivery Retention
|--------------------------------------------------------------------------
|
| Deliveries from chat apps are kept as the provider sent them, with
| what customers wrote, so they go once they have done their job. A
| delivery processed or ignored is needed only briefly; one that failed,
| or waits while its workspace is suspended, is kept longer for a replay.
|
*/

package config

/*
ChannelInboundEventsConfig says how long kept deliveries stay.

ProcessedRetentionDays covers processed and ignored deliveries, and
FailedRetentionDays failed and held ones. A period of 0 keeps them for
ever.
*/
type ChannelInboundEventsConfig struct {
	ProcessedRetentionDays int `default:"7"  koanf:"processed_retention_days"`
	FailedRetentionDays    int `default:"30" koanf:"failed_retention_days"`
}
