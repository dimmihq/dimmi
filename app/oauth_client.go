/*
|--------------------------------------------------------------------------
| Integration Apps
|--------------------------------------------------------------------------
|
| The apps dimmi registered at other services become providers of the
| framework's account connections, under the provider's own name. Each
| sends the browser back to the one integrations callback, so the
| redirect URL registered with the service never changes. Options only a
| driver knows stay under oauth_client.providers and win over these.
|
*/

package app

import (
	"maps"
	"strings"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/foundation"
)

/*
configureOAuthClient hands the framework's account connections the
shared Slack app, when its credentials are set.
*/
func (a *App) configureOAuthClient(cfg *foundation.Config) {
	if cfg.OAuthClient.Providers == nil {
		cfg.OAuthClient.Providers = map[string]config.Section{}
	}

	slack := a.Config.Services.Slack
	if !slack.Configured() {
		return
	}

	section := config.Section{
		"driver":        "slack",
		"client_id":     slack.ClientID,
		"client_secret": slack.ClientSecret,
		"redirect_url":  IntegrationCallbackURL(cfg.App.URL, "slack"),
	}
	maps.Copy(section, cfg.OAuthClient.Providers["slack"])
	cfg.OAuthClient.Providers["slack"] = section
}

/*
IntegrationCallbackURL is where a provider sends the browser back after
an install or a member's grant.
*/
func IntegrationCallbackURL(appURL, provider string) string {
	return strings.TrimRight(appURL, "/") + "/integrations/" + provider + "/callback"
}
