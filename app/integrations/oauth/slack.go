/*
|--------------------------------------------------------------------------
| Slack
|--------------------------------------------------------------------------
|
| One Slack app serves every feature: customer channels, notifications
| and Operator. Its install asks for the bot scopes of all three by
| default, so turning on a feature later needs no second consent, and a
| teammate's own grant asks only for the user scopes that let replies
| and reactions go out as them.
|
| A token is checked with auth.test, which names the Slack workspace,
| the bot user and, in a response header, the scopes the token carries;
| bots.info then names the app. An Enterprise Grid organization install
| is refused: the product connects one Slack workspace at a time.
|
*/

package oauth

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/oauthclient"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
The features that use Slack.
*/
const (
	FeatureChannels      = "channels"
	FeatureNotifications = "notifications"
	FeatureOperator      = "operator"
)

/*
SlackAPI is the base URL of Slack's Web API.
*/
const SlackAPI = "https://slack.com/api"

/*
RegisterSlack adds Slack and the scopes of its features, calling its
API through the client.
*/
func RegisterSlack(r *Registry, client *httpclient.Client) {
	name := string(enums.IntegrationProviderSlack)
	check := slackCheck{client: client}

	r.Register(Provider{
		Name: name, Label: "Slack", Check: check.check, Metadata: slackMetadata, Validate: slackValidate,
		Hint: func(account string) map[string]string { return map[string]string{"team": account} },
	})

	r.RegisterScopes(name, FeatureChannels, Scopes{
		Bot: []string{
			"channels:history", "groups:history", "im:history", "channels:read", "groups:read", "im:read",
			"im:write", "chat:write", "chat:write.customize", "chat:write.public", "files:read", "files:write",
			"users:read", "users:read.email", "reactions:read", "reactions:write", "team:read", "channels:join",
			"app_mentions:read", "commands", "assistant:write",
		},
		User: []string{"chat:write", "reactions:write"},
	})
	r.RegisterScopes(name, FeatureNotifications, Scopes{
		Bot: []string{"chat:write", "chat:write.public", "im:write", "users:read", "users:read.email"},
	})
	r.RegisterScopes(name, FeatureOperator, Scopes{
		Bot: []string{"app_mentions:read", "im:history", "assistant:write"},
	})
}

/*
slackMetadata keeps the bot user, the app's bot and the Grid
organization of a fresh install.
*/
func slackMetadata(grant oauthclient.Grant) map[string]any {
	out := map[string]any{}

	if bot, ok := grant.Raw["bot_user_id"].(string); ok && bot != "" {
		out["bot_user_id"] = bot
	}

	if enterprise, ok := grant.Raw["enterprise"].(map[string]any); ok {
		out["enterprise_id"] = enterprise["id"]
	}

	return out
}

/*
slackValidate refuses an organization-wide install.
*/
func slackValidate(grant oauthclient.Grant) error {
	if grant.Raw["is_enterprise_install"] == true {
		return exceptions.ErrEnterpriseInstallUnsupported
	}

	return nil
}

/*
slackCheck asks Slack about a bot token.
*/
type slackCheck struct {
	client *httpclient.Client
}

/*
slackAnswer is what auth.test and bots.info answer.
*/
type slackAnswer struct {
	OK           bool   `json:"ok"`
	Error        string `json:"error"`
	URL          string `json:"url"`
	Team         string `json:"team"`
	TeamID       string `json:"team_id"`
	UserID       string `json:"user_id"`
	BotID        string `json:"bot_id"`
	EnterpriseID string `json:"enterprise_id"`
	Bot          struct {
		AppID string `json:"app_id"`
	} `json:"bot"`
}

/*
check runs auth.test, then bots.info for the app the bot belongs to.
*/
func (c slackCheck) check(ctx context.Context, token string) (Health, error) {
	who, header, err := c.call(ctx, token, "auth.test", nil)
	if err != nil {
		return Health{}, err
	}

	health := Health{
		Account: who.TeamID, AccountName: who.Team, Scopes: splitScopes(header.Get("X-OAuth-Scopes")),
		Metadata: map[string]any{"bot_user_id": who.UserID, "bot_id": who.BotID, "team_domain": teamDomain(who.URL)},
	}

	if who.EnterpriseID != "" {
		health.Metadata["enterprise_id"] = who.EnterpriseID
	}

	if who.BotID == "" {
		return health, nil
	}

	bot, _, err := c.call(ctx, token, "bots.info", url.Values{"bot": {who.BotID}})
	if err != nil {
		return Health{}, err
	}

	health.App = bot.Bot.AppID

	return health, nil
}

/*
call posts to the Web API method with the token. Slack refusing the
token is ErrAuthFailed; any other failure is the provider's.
*/
func (c slackCheck) call(ctx context.Context, token, method string, form url.Values,
) (slackAnswer, fhttp.Header, error) {
	resp, err := c.client.New().WithToken(token).
		WithHeader("Content-Type", "application/x-www-form-urlencoded").
		Post(ctx, SlackAPI+"/"+method, form.Encode())
	if err != nil {
		return slackAnswer{}, nil, fmt.Errorf("slack %s: %w", method, err)
	}

	var answer slackAnswer
	if resp.Status != fhttp.StatusOK || resp.JSON(&answer) != nil {
		return slackAnswer{}, nil, fmt.Errorf("slack %s: %w: answered %d", method, oauthclient.ErrProvider, resp.Status)
	}

	switch {
	case answer.OK:
		return answer, resp.Header, nil
	case answer.Error == "invalid_auth" || answer.Error == "token_revoked" || answer.Error == "account_inactive" ||
		answer.Error == "not_authed":
		return slackAnswer{}, nil, fmt.Errorf("slack %s: %w: %s", method, ErrAuthFailed, answer.Error)
	default:
		return slackAnswer{}, nil, fmt.Errorf("slack %s: %w: %s", method, oauthclient.ErrProvider, answer.Error)
	}
}

/*
teamDomain reads the Slack workspace's subdomain from its URL.
*/
func teamDomain(raw string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
	domain, _, _ := strings.Cut(host, ".")

	return domain
}

/*
splitScopes reads a comma-joined scope header.
*/
func splitScopes(header string) []string {
	var out []string

	for scope := range strings.SplitSeq(header, ",") {
		if scope = strings.TrimSpace(scope); scope != "" {
			out = append(out, scope)
		}
	}

	return out
}
