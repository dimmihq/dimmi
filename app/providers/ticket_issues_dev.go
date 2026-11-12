//go:build dev

/*
|--------------------------------------------------------------------------
| Canned Engineering Tools
|--------------------------------------------------------------------------
|
| A development build started with services.trackers.fake answers every
| engineering tool from canned data, so the linking dialog, the pickers
| and a tool's settings page can be tried and screenshotted without a
| Linear, Jira or PagerDuty account. Keys and tokens are accepted as
| they are pasted. Nothing reaches the real tools.
|
*/

package providers

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/issues"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
)

/*
fakeTrackers swaps every tool for a canned one when the setting asks.
*/
func fakeTrackers(a *app.App, s *issues.Issues) {
	if !a.Config.Services.Trackers.Fake {
		return
	}

	for name := range s.Trackers {
		incidents := s.Trackers[name].Incidents()
		s.Trackers[name] = &cannedTool{provider: name, incidents: incidents}

		if p, ok := s.Registry.Provider(name); ok {
			p.Check = func(context.Context, string) (oauth.Health, error) {
				return oauth.Health{
					Account: "demo-" + name, AccountName: "Acme " + issues.Label(name),
					Scopes: s.Registry.BotScopes(name, nil),
				}, nil
			}
			s.Registry.Register(p)
		}
	}
}

/*
cannedTool answers like a tool from canned data.
*/
type cannedTool struct {
	provider  string
	incidents bool
	made      int
}

/*
Provider names the tool.
*/
func (c *cannedTool) Provider() string { return c.provider }

/*
Incidents says whether it holds incidents.
*/
func (c *cannedTool) Incidents() bool { return c.incidents }

/*
Options returns canned lists.
*/
func (c *cannedTool) Options(_ context.Context, _ trackers.Call, list, _ string) ([]trackers.Option, error) {
	switch list {
	case trackers.ListTeams:
		return []trackers.Option{
			{ID: "eng", Name: "Engineering", Detail: "ENG"},
			{ID: "web", Name: "Web app", Detail: "WEB"},
			{ID: "bil", Name: "Billing", Detail: "BIL"},
		}, nil
	case trackers.ListProjects:
		return []trackers.Option{{ID: "rel", Name: "Q4 reliability"}, {ID: "exp", Name: "Exports v2"}}, nil
	case trackers.ListStates:
		return c.states(), nil
	case trackers.ListSeverities:
		return []trackers.Option{
			{ID: "minor", Name: "Minor", Rank: 1},
			{ID: "major", Name: "Major", Rank: 2},
			{ID: "critical", Name: "Critical", Rank: 3},
		}, nil
	case trackers.ListServices:
		return []trackers.Option{{ID: "checkout", Name: "Checkout"}, {ID: "api", Name: "Public API"}}, nil
	case trackers.ListUsers:
		return []trackers.Option{{ID: "sam@acme.test", Name: "Sam Rivera", Detail: "sam@acme.test"}}, nil
	case trackers.ListFields:
		return []trackers.Option{{ID: "customers", Name: "Customers affected"}}, nil
	case trackers.ListEnvironments:
		return []trackers.Option{{ID: "production", Name: "production"}, {ID: "staging", Name: "staging"}}, nil
	}

	return nil, trackers.ErrUnsupported
}

/*
states are the tool's canned workflow.
*/
func (c *cannedTool) states() []trackers.Option {
	if c.incidents {
		return []trackers.Option{
			{ID: "triage", Name: "Triage", Group: "backlog"},
			{ID: "investigating", Name: "Investigating", Group: "started"},
			{ID: "fixing", Name: "Fixing", Group: "started"},
			{ID: "monitoring", Name: "Monitoring", Group: "started"},
			{ID: "closed", Name: "Closed", Group: "done"},
		}
	}

	return []trackers.Option{
		{ID: "backlog", Name: "Backlog", Group: "backlog"},
		{ID: "todo", Name: "Todo", Group: "unstarted"},
		{ID: "progress", Name: "In Progress", Group: "started"},
		{ID: "review", Name: "In Review", Group: "started"},
		{ID: "done", Name: "Done", Group: "done"},
		{ID: "canceled", Name: "Canceled", Group: "canceled"},
	}
}

/*
canned are the issues a search finds.
*/
func (c *cannedTool) canned() []trackers.Issue {
	titles := []string{
		"CSV export crashes on files over 10 MB", "Export times out for large workspaces",
		"CSV export drops accented characters",
	}
	statuses := []string{"In Progress", "Backlog", "Done"}
	groups := []string{"started", "backlog", "done"}

	out := make([]trackers.Issue, len(titles))
	for i, title := range titles {
		out[i] = c.issue(410+i, title)
		out[i].Status, out[i].StatusGroup = statuses[i], groups[i]
	}

	return out
}

/*
issue is a canned issue of the number.
*/
func (c *cannedTool) issue(n int, title string) trackers.Issue {
	key := "ENG-" + strconv.Itoa(n)
	if c.incidents {
		key = "INC-" + strconv.Itoa(n)
	}

	return trackers.Issue{
		ExternalID: c.provider + "-" + strconv.Itoa(n), Key: key, Title: title, Status: "Backlog",
		StatusGroup: "backlog", TeamID: "eng", TeamName: "Engineering",
		URL: "https://example.test/" + c.provider + "/" + strings.ToLower(key),
	}
}

/*
Search finds canned issues by their words.
*/
func (c *cannedTool) Search(_ context.Context, _ trackers.Call, q string) ([]trackers.Issue, error) {
	var out []trackers.Issue

	for _, is := range c.canned() {
		if strings.Contains(strings.ToLower(is.Title+" "+is.Key), strings.ToLower(q)) {
			out = append(out, is)
		}
	}

	return out, nil
}

/*
Read returns the canned issue a reference names.
*/
func (c *cannedTool) Read(_ context.Context, _ trackers.Call, ref string) (trackers.Issue, error) {
	for _, is := range c.canned() {
		if strings.Contains(ref, strings.ToLower(is.Key)) || strings.Contains(ref, is.Key) || ref == is.ExternalID {
			return is, nil
		}
	}

	return trackers.Issue{}, trackers.ErrNotFound
}

/*
Create makes a canned issue.
*/
func (c *cannedTool) Create(_ context.Context, _ trackers.Call, in trackers.CreateInput) (trackers.Issue, error) {
	c.made++

	is := c.issue(500+c.made, in.Title)
	if c.incidents {
		is.Status, is.StatusGroup, is.Severity = "Investigating", "started", in.Severity
	}

	return is, nil
}

/*
Comment is accepted and dropped.
*/
func (*cannedTool) Comment(context.Context, trackers.Call, trackers.Issue, string) error { return nil }

/*
Impact is accepted and dropped.
*/
func (*cannedTool) Impact(context.Context, trackers.Call, trackers.Issue, trackers.Impact) error {
	return nil
}

/*
Verify refuses every delivery: nothing delivers to a canned tool.
*/
func (*cannedTool) Verify(trackers.Signed, string, time.Time) error { return trackers.ErrBadSignature }

/*
Parse reads nothing.
*/
func (*cannedTool) Parse([]byte) ([]trackers.Event, error) { return nil, nil }
