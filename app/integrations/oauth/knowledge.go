/*
|--------------------------------------------------------------------------
| Knowledge Services
|--------------------------------------------------------------------------
|
| Notion, Intercom, Zendesk and GitHub are connected so knowledge
| sources can read pages, help center articles and repositories with
| the access the person
| granted, never with a key pasted into a form. Each connected account
| is its own installation, so a workspace can read two Notion
| workspaces or two help desks. Notion, Intercom and GitHub name the
| account when asked with the token; a Zendesk account is the
| subdomain the person typed, so its token is checked when a source
| reads it. GitHub asks for repo, the only scope that reaches private
| repositories, and dimmi only ever reads with it.
|
*/

package oauth

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
KnowledgeFeature is the feature knowledge sources connect services for.
*/
const KnowledgeFeature = "knowledge"

/*
RegisterKnowledgeServices registers the services knowledge sources read.
*/
func RegisterKnowledgeServices(r *Registry, client *httpclient.Client) {
	notion := func(ctx context.Context, token string) (Health, error) { return notionHealth(ctx, client, token) }
	intercom := func(ctx context.Context, token string) (Health, error) { return intercomHealth(ctx, client, token) }
	github := func(ctx context.Context, token string) (Health, error) { return githubHealth(ctx, client, token) }

	r.Register(Provider{
		Name: "notion", Label: "Notion", Check: notion, Identify: notion, Separate: true,
		AuthParams: map[string]string{"owner": "user"},
	})
	r.Register(Provider{Name: "intercom", Label: "Intercom", Check: intercom, Identify: intercom, Separate: true})
	r.Register(Provider{Name: "zendesk", Label: "Zendesk", Check: zendeskHealth, Separate: true})
	r.Register(Provider{Name: "github", Label: "GitHub", Check: github, Identify: github, Separate: true})

	for _, name := range []string{"notion", "intercom", "zendesk"} {
		r.RegisterScopes(name, KnowledgeFeature, Scopes{})
	}

	r.RegisterScopes("github", KnowledgeFeature, Scopes{Bot: []string{"repo", "read:user"}})
}

/*
notionHealth asks Notion which workspace the token's bot lives in.
*/
func notionHealth(ctx context.Context, client *httpclient.Client, token string) (Health, error) {
	var me struct {
		Bot struct {
			WorkspaceID   string `json:"workspace_id"`
			WorkspaceName string `json:"workspace_name"`
		} `json:"bot"`
	}

	req := client.New().WithToken(token).WithHeader("Notion-Version", "2022-06-28").Accept("application/json")
	if err := ask(ctx, req, "https://api.notion.com/v1/users/me", &me); err != nil {
		return Health{}, err
	}

	return Health{Account: me.Bot.WorkspaceID, AccountName: me.Bot.WorkspaceName}, nil
}

/*
intercomHealth asks Intercom which workspace the token belongs to.
*/
func intercomHealth(ctx context.Context, client *httpclient.Client, token string) (Health, error) {
	var me struct {
		App struct {
			IDCode string `json:"id_code"`
			Name   string `json:"name"`
		} `json:"app"`
	}

	req := client.New().WithToken(token).WithHeader("Intercom-Version", "2.11").Accept("application/json")
	if err := ask(ctx, req, "https://api.intercom.io/me", &me); err != nil {
		return Health{}, err
	}

	return Health{Account: me.App.IDCode, AccountName: me.App.Name}, nil
}

/*
githubHealth asks GitHub which account the token belongs to.
*/
func githubHealth(ctx context.Context, client *httpclient.Client, token string) (Health, error) {
	var me struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}

	req := client.New().WithToken(token).WithHeader("X-GitHub-Api-Version", "2022-11-28").
		Accept("application/vnd.github+json")
	if err := ask(ctx, req, "https://api.github.com/user", &me); err != nil {
		return Health{}, err
	}

	return Health{Account: strconv.FormatInt(me.ID, 10), AccountName: me.Login}, nil
}

/*
zendeskHealth trusts a Zendesk token: its account lives on a subdomain
the token alone cannot name, so a failing token shows on the source
that reads with it.
*/
func zendeskHealth(context.Context, string) (Health, error) {
	return Health{}, nil
}

/*
ask sends one request and decodes the answer, a refused token being
ErrAuthFailed.
*/
func ask(ctx context.Context, req *httpclient.Request, address string, out any) error {
	resp, err := req.Get(ctx, address)
	if err != nil {
		return fmt.Errorf("ask %s: %w", address, err)
	}

	if resp.Status == 401 || resp.Status == 403 {
		return ErrAuthFailed
	}

	if resp.Status != 200 {
		return fmt.Errorf("ask %s: status %d", address, resp.Status)
	}

	return resp.JSON(out)
}
