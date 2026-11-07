/*
|--------------------------------------------------------------------------
| Identity Verification Settings
|--------------------------------------------------------------------------
|
| How long a signed-in visitor's proof lasts and how forgiving token
| checks are. A token must expire within MaxTokenLifetime, so a leaked
| one is never a standing credential; a signed-in session lives
| SessionTTL past its last fresh token, so an unattended browser signs
| itself out.
|
*/

package config

import "time"

/*
IdentityConfig configures visitor identity verification. SessionTTL is
how long a signed-in session lives after its last verified token,
ClockLeeway the clock skew forgiven between servers, MaxTokenLifetime the
longest a token may be valid, MaxActiveSecrets how many secrets a
workspace keeps at once, RetireDefault how long a rotated secret keeps
working and RetireMax the furthest a retirement may be set.
*/
type IdentityConfig struct {
	SessionTTL       time.Duration `default:"24h"  koanf:"session_ttl"`
	ClockLeeway      time.Duration `default:"60s"  koanf:"clock_leeway"`
	MaxTokenLifetime time.Duration `default:"168h" koanf:"max_token_lifetime"`
	MaxActiveSecrets int64         `default:"5"    koanf:"max_active_secrets"`
	RetireDefault    time.Duration `default:"168h" koanf:"retire_default"`
	RetireMax        time.Duration `default:"720h" koanf:"retire_max"`
}
