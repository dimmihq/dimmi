/*
|--------------------------------------------------------------------------
| Entitlement Resources
|--------------------------------------------------------------------------
|
| What a workspace may have, as a client reads it.
|
*/

package resources

/*
WorkspaceEntitlements is what the workspace may have; API holds its
rate limits.
*/
type WorkspaceEntitlements struct {
	Object string    `json:"object"`
	API    APILimits `json:"api"`
}

/*
APILimits are requests a minute per access token, per signed-in person
and per workspace's tokens together; zero means no limit.
*/
type APILimits struct {
	TokenPerMinute     int64 `json:"token_per_minute"`
	UserPerMinute      int64 `json:"user_per_minute"`
	WorkspacePerMinute int64 `json:"workspace_per_minute"`
}
