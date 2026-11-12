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

	appconfig "github.com/dimmihq/dimmi/config"
)

/*
configureOAuthClient hands the framework's account connections the
shared Slack app and the knowledge services' clients, when their
credentials are set.
*/
func (a *App) configureOAuthClient(cfg *foundation.Config) {
	if cfg.OAuthClient.Providers == nil {
		cfg.OAuthClient.Providers = map[string]config.Section{}
	}

	for name, section := range knowledgeClients(a.Config.Services) {
		section["redirect_url"] = IntegrationCallbackURL(cfg.App.URL, name)
		maps.Copy(section, cfg.OAuthClient.Providers[name])
		cfg.OAuthClient.Providers[name] = section
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
knowledgeClients are the OAuth clients of the services knowledge
sources read, those with credentials set: Notion, Intercom and GitHub
follow the standard, Zendesk lives on each customer's own subdomain.
*/
func knowledgeClients(s appconfig.ServicesConfig) map[string]config.Section {
	out := map[string]config.Section{}

	if c := s.Notion; c.ClientID != "" && c.ClientSecret != "" {
		out["notion"] = oauthSection("generic", c, endpoints{
			authorize: "https://api.notion.com/v1/oauth/authorize",
			exchange:  "https://api.notion.com/v1/oauth/token",
		})
		out["notion"]["header_auth"] = true
		out["notion"]["pkce"] = false
	}

	if c := s.Intercom; c.ClientID != "" && c.ClientSecret != "" {
		out["intercom"] = oauthSection("generic", c, endpoints{
			authorize: "https://app.intercom.com/oauth",
			exchange:  "https://api.intercom.io/auth/eagle/token",
		})
		out["intercom"]["pkce"] = false
	}

	if c := s.GitHub; c.ClientID != "" && c.ClientSecret != "" {
		out["github"] = oauthSection("generic", c, endpoints{
			authorize: "https://github.com/login/oauth/authorize",
			exchange:  "https://github.com/login/oauth/access_token",
		})
	}

	trackerClients(s, out)

	if c := s.Zendesk; c.ClientID != "" {
		out["zendesk"] = oauthSection("hosted", c, endpoints{
			authorize: "https://{account}.zendesk.com/oauth/authorizations/new",
			exchange:  "https://{account}.zendesk.com/oauth/tokens",
		})
		out["zendesk"]["scopes"] = []string{"read"}
	}

	return out
}

/*
trackerClients adds the OAuth clients of Linear and Jira Cloud, those
with credentials set. Linear joins scopes with commas and acts as the
app; Atlassian needs its API audience and a consent prompt so a refresh
token comes back.
*/
func trackerClients(s appconfig.ServicesConfig, out map[string]config.Section) {
	if c := s.Linear; c.Configured() {
		out["linear"] = oauthSection("generic", appconfig.OAuthServiceConfig{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
		}, endpoints{
			authorize: "https://linear.app/oauth/authorize", exchange: "https://api.linear.app/oauth/token",
		})
		out["linear"]["revoke_url"] = "https://api.linear.app/oauth/revoke"
		out["linear"]["scope_separator"] = ","
	}

	if c := s.Jira; c.Configured() {
		out["jira"] = oauthSection("generic", appconfig.OAuthServiceConfig{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
		}, endpoints{
			authorize: "https://auth.atlassian.com/authorize", exchange: "https://auth.atlassian.com/oauth/token",
		})
		out["jira"]["pkce"] = false
	}
}

/*
endpoints are where a service asks the person to allow access and where
the code it hands back is exchanged.
*/
type endpoints struct {
	authorize string
	exchange  string
}

/*
oauthSection is the provider config of a service's OAuth client.
*/
func oauthSection(driver string, c appconfig.OAuthServiceConfig, e endpoints) config.Section {
	return config.Section{
		"driver": driver, "client_id": c.ClientID, "client_secret": c.ClientSecret,
		"auth_url": e.authorize, "token_url": e.exchange,
	}
}

/*
IntegrationCallbackURL is where a provider sends the browser back after
an install or a member's grant.
*/
func IntegrationCallbackURL(appURL, provider string) string {
	return strings.TrimRight(appURL, "/") + "/integrations/" + provider + "/callback"
}
