/*
|--------------------------------------------------------------------------
| OAuth Server Settings
|--------------------------------------------------------------------------
|
| dimmi signs agents and their apps in with OAuth, so a person connects
| Claude, ChatGPT or Cursor from a browser instead of pasting a token.
| Access tokens live an hour and are renewed with refresh tokens; a
| connection nobody uses for ninety days must sign in again. MCP clients
| register themselves before they ask, so registration is open, and
| every app that registers this way is shown to the person as one dimmi
| cannot vouch for.
|
*/

package config

import "time"

/*
OAuthServerConfig sets how agents sign in. AccessTokenTTL and
RefreshTokenTTL are how long the two tokens last, and Registration
whether apps may register themselves, which MCP clients do.
*/
type OAuthServerConfig struct {
	AccessTokenTTL  time.Duration `default:"1h"    koanf:"access_token_ttl"`
	RefreshTokenTTL time.Duration `default:"2160h" koanf:"refresh_token_ttl"`
	Registration    bool          `default:"true"  koanf:"registration"`
}
