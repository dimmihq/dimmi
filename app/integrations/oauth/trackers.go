/*
|--------------------------------------------------------------------------
| Engineering Tools
|--------------------------------------------------------------------------
|
| The tools tickets link to. Linear and Jira Cloud connect over OAuth:
| Linear's install acts as the app, so issues and comments it writes are
| the app's, and Atlassian is asked for its API audience and a refresh
| token. GitHub issues reuse the GitHub connection knowledge sources
| made, asking for the right to add a repository webhook. incident.io,
| Rootly and PagerDuty connect with an API key the workspace pastes,
| checked against the tool before it is kept.
|
| A Linear install is the Linear organization; a Jira one is the site,
| named by the cloud id Atlassian's accessible resources list.
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
The features of the engineering tools.
*/
const (
	FeatureIssues    = "issues"
	FeatureIncidents = "incidents"
)

/*
RegisterTrackers adds the issue trackers and incident tools.
*/
func RegisterTrackers(r *Registry, client *httpclient.Client) {
	linear := func(ctx context.Context, token string) (Health, error) { return linearHealth(ctx, client, token) }
	jira := func(ctx context.Context, token string) (Health, error) { return jiraHealth(ctx, client, token) }

	r.Register(Provider{
		Name: "linear", Label: "Linear", Check: linear, Identify: linear, Separate: true,
		AuthParams: map[string]string{"actor": "app", "prompt": "consent"},
	})
	r.Register(Provider{
		Name: "jira", Label: "Jira", Check: jira, Identify: jira, Separate: true,
		AuthParams: map[string]string{"audience": "api.atlassian.com", "prompt": "consent"},
	})
	r.RegisterScopes("linear", FeatureIssues, Scopes{Bot: []string{"read", "write"}})
	r.RegisterScopes("jira", FeatureIssues, Scopes{Bot: []string{
		"read:jira-work", "write:jira-work", "manage:jira-webhook", "read:jira-user", "offline_access",
	}})
	r.RegisterScopes("github", FeatureIssues, Scopes{Bot: []string{"repo", "admin:repo_hook", "read:user"}})

	keyed := map[string]Provider{
		"incident_io": {Label: "incident.io", Check: keyCheck(client, "https://api.incident.io/v1/identity", "")},
		"rootly":      {Label: "Rootly", Check: keyCheck(client, "https://api.rootly.com/v1/users/me", "")},
		"pagerduty":   {Label: "PagerDuty", Check: keyCheck(client, "https://api.pagerduty.com/abilities", "pd")},
	}

	for _, name := range []string{"incident_io", "rootly", "pagerduty"} {
		p := keyed[name]
		p.Name, p.Key, p.Separate = name, true, true
		r.Register(p)
		r.RegisterScopes(name, FeatureIncidents, Scopes{})
	}
}

/*
linearHealth asks Linear which organization the token belongs to.
*/
func linearHealth(ctx context.Context, client *httpclient.Client, token string) (Health, error) {
	var out struct {
		Data struct {
			Organization struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				URLKey string `json:"url_key"`
			} `json:"organization"`
		} `json:"data"`
	}

	resp, err := client.New().WithToken(token).WithHeader("Content-Type", "application/json").
		Post(ctx, "https://api.linear.app/graphql", []byte(`{"query":"{ organization { id name url_key: urlKey } }"}`))
	if err != nil {
		return Health{}, fmt.Errorf("linear organization: %w", err)
	}

	if resp.Status == 401 || resp.Status == 403 {
		return Health{}, ErrAuthFailed
	}

	if resp.Status != 200 || resp.JSON(&out) != nil || out.Data.Organization.ID == "" {
		return Health{}, fmt.Errorf("linear organization: status %d", resp.Status)
	}

	org := out.Data.Organization

	return Health{Account: org.ID, AccountName: org.Name, Metadata: map[string]any{"url_key": org.URLKey}}, nil
}

/*
jiraHealth asks Atlassian which Jira site the token reaches, the first
when it reaches several.
*/
func jiraHealth(ctx context.Context, client *httpclient.Client, token string) (Health, error) {
	var sites []struct {
		ID   string `json:"id"`
		URL  string `json:"url"`
		Name string `json:"name"`
	}

	req := client.New().WithToken(token).Accept("application/json")
	if err := ask(ctx, req, "https://api.atlassian.com/oauth/token/accessible-resources", &sites); err != nil {
		return Health{}, err
	}

	if len(sites) == 0 {
		return Health{}, ErrAuthFailed
	}

	site := sites[0]

	return Health{
		Account: site.ID, AccountName: site.Name,
		Metadata: map[string]any{"cloud_id": site.ID, "site_url": site.URL},
	}, nil
}

/*
keyCheck checks a pasted API key against the address. PagerDuty wants
its own token header; the others a bearer token. The account is the key
holder's name the tool reports, or the address's host.
*/
func keyCheck(client *httpclient.Client, address, style string) func(context.Context, string) (Health, error) {
	return func(ctx context.Context, token string) (Health, error) {
		req := client.New().Accept("application/json")
		if style == "pd" {
			req.WithHeader("Authorization", "Token token="+token).
				WithHeader("Accept", "application/vnd.pagerduty+json;version=2")
		} else {
			req.WithToken(token)
		}

		var out map[string]any
		if err := ask(ctx, req, address, &out); err != nil {
			return Health{}, err
		}

		return Health{Account: keyAccount(out, token), AccountName: keyName(out)}, nil
	}
}

/*
keyAccount names the account a key belongs to: the id the tool reports
or, when it reports none, a stable fingerprint of the key's length and
ends, so one key is one installation.
*/
func keyAccount(out map[string]any, token string) string {
	for _, path := range [][]string{{"identity", "dashboard_url"}, {"data", "id"}} {
		if node, ok := out[path[0]].(map[string]any); ok {
			if s, ok := node[path[1]].(string); ok && s != "" {
				return s
			}
		}
	}

	if len(token) < 8 {
		return "key"
	}

	return "key-" + strconv.Itoa(len(token)) + "-" + token[len(token)-4:]
}

/*
keyName names the key's holder as the tool reports it.
*/
func keyName(out map[string]any) string {
	if identity, ok := out["identity"].(map[string]any); ok {
		if name, ok := identity["name"].(string); ok {
			return name
		}
	}

	if data, ok := out["data"].(map[string]any); ok {
		if attrs, ok := data["attributes"].(map[string]any); ok {
			if name, ok := attrs["name"].(string); ok {
				return name
			}
		}
	}

	return "API key"
}
