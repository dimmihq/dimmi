/*
|--------------------------------------------------------------------------
| Webhook Settings
|--------------------------------------------------------------------------
|
| Outbound webhooks post signed events to the endpoints a workspace
| registers. A request waits a bounded time for an answer, reads a
| bounded body, and a failed delivery is tried again on a fixed
| schedule over a day. An endpoint failing for days is turned off so it
| stops costing work, and its owners hear about it first.
|
*/

package config

import "time"

/*
WebhooksConfig configures outbound webhooks.

Enabled false stops every event from being delivered, while endpoints
can still be managed. MaxEndpoints caps a workspace's endpoints on a
self-hosted install. RequireHTTPS refuses plain http endpoints; unset,
it holds on the cloud only. Timeout bounds one request, headers and
body. MaxPayloadBytes is the largest body sent before the snapshot is
replaced by a reference. MaxResponseBytes is how much of an answer is
read, StoredResponseBytes how much of it the log keeps. FailingNotifyAfter
is how long an endpoint fails before its owners are told, DisableAfter
how long before it is turned off. RetentionDays is how long the delivery
log is kept. MaxRotationOverlap caps how long a replaced secret still
signs. TestsPerMinute caps test sends per endpoint.
*/
type WebhooksConfig struct {
	Enabled             bool          `default:"true"   koanf:"enabled"`
	MaxEndpoints        int64         `default:"20"     koanf:"max_endpoints"`
	RequireHTTPS        *bool         `koanf:"require_https"`
	Timeout             time.Duration `default:"15s"    koanf:"timeout"`
	MaxPayloadBytes     int           `default:"262144" koanf:"max_payload_bytes"`
	MaxResponseBytes    int64         `default:"65536"  koanf:"max_response_bytes"`
	StoredResponseBytes int           `default:"4096"   koanf:"stored_response_bytes"`
	FailingNotifyAfter  time.Duration `default:"24h"    koanf:"failing_notify_after"`
	DisableAfter        time.Duration `default:"72h"    koanf:"disable_after"`
	RetentionDays       int           `default:"30"     koanf:"delivery_retention_days"`
	MaxRotationOverlap  time.Duration `default:"168h"   koanf:"rotation_max_overlap"`
	TestsPerMinute      int           `default:"10"     koanf:"tests_per_minute"`
}

/*
HTTPSRequired reports whether endpoints must use https: as configured,
or on the cloud only when unset.
*/
func (c WebhooksConfig) HTTPSRequired(edition Edition) bool {
	if c.RequireHTTPS != nil {
		return *c.RequireHTTPS
	}

	return edition == EditionCloud
}
