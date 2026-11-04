/*
|--------------------------------------------------------------------------
| API Limits
|--------------------------------------------------------------------------
|
| The public API's rate limits on a self-hosted install, where the
| operator sets them; the cloud reads them from entitlements. Zero means
| no limit. Repeated bad credentials from one address are always slowed.
|
*/

package config

/*
APILimitsConfig holds requests a minute per token, per signed-in person,
per workspace's tokens together, and per address that keeps sending bad
credentials.
*/
type APILimitsConfig struct {
	TokenPerMinute         int64 `default:"600"  koanf:"token_per_minute"`
	UserPerMinute          int64 `default:"1200" koanf:"user_per_minute"`
	WorkspacePerMinute     int64 `default:"6000" koanf:"workspace_per_minute"`
	BadCredentialPerMinute int   `default:"60"   koanf:"bad_credential_per_minute"`
}
