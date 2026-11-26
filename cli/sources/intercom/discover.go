package intercom

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

/*
permission names the Intercom private-app permission a kind needs.
*/
func permission(kind format.Kind) string {
	switch kind {
	case format.KindTeammate, format.KindTeam, format.KindSchedule:
		return "Read admins"
	case format.KindTag:
		return "Read tags"
	case format.KindAttributeDefinition:
		return "Read data attributes"
	case format.KindCompany:
		return "Read one company / Read and list companies"
	case format.KindContact, format.KindContactNote:
		return "Read and list users and companies"
	case format.KindTicketType, format.KindTicket:
		return "Read tickets"
	case format.KindHelpCenter, format.KindCollection, format.KindArticle:
		return "Read and list articles"
	case format.KindSnippet:
		return "Read content"
	case format.KindSavedReply, format.KindConversation:
		return "Read conversations"
	case format.KindWorkflow:
		return "Read workflows"
	}

	return ""
}

/*
notImported is what a migration from Intercom leaves behind, shown in
the plan so nobody is surprised.
*/
func notImported() []string {
	return []string{
		"Segments: they are rules, rebuild them as views or audiences",
		"Outbound messages, series, banners, tours and news items",
		"Fin and bot settings, and workflows you didn't list with --workflow",
		"Reports, statistics and SLA outcomes: dimmi works out reply and close times from the history",
		"Device, browser, UTM and social profile fields, subscription types and SMS consent",
		"Custom objects and relationship attributes",
		"Tickets beyond the 500 most recent parts each, which Intercom doesn't return",
	}
}

/*
total reads a list's total_count from one call.
*/
func (s *Source) total(ctx context.Context, path string, q url.Values) (int, error) {
	var out struct {
		TotalCount int `json:"total_count"`
	}

	err := s.Client.get(ctx, path, q, &out)

	return out.TotalCount, err
}

func (s *Source) searchTotal(ctx context.Context, path string) (int, error) {
	var out struct {
		TotalCount int `json:"total_count"`
	}

	body := map[string]any{
		"query":      map[string]any{"field": "updated_at", "operator": ">", "value": 0},
		"pagination": map[string]any{"per_page": 1},
	}

	err := s.Client.post(ctx, path, body, &out)

	return out.TotalCount, err
}

/*
discovery is a count in progress: the inventory and the permissions
already found missing.
*/
type discovery struct {
	inv     engine.Inventory
	missing map[string]bool
}

/*
add records a kind's count, or why it couldn't be counted. An error
other than a refusal or a missing endpoint stops the count.
*/
func (d *discovery) add(c engine.KindCount, err error) error {
	switch {
	case isForbidden(err):
		if p := permission(c.Kind); p != "" && !d.missing[p] {
			d.missing[p] = true
			d.inv.Missing = append(d.inv.Missing, p)
		}

		d.inv.Kinds = append(d.inv.Kinds, engine.KindCount{Kind: c.Kind, Notes: []string{"the token can't read these"}})
	case isNotFound(err):
		d.inv.Kinds = append(d.inv.Kinds, engine.KindCount{
			Kind: c.Kind, Notes: []string{"not available on this workspace"},
		})
	case err != nil:
		return err
	default:
		d.inv.Kinds = append(d.inv.Kinds, c)
	}

	return nil
}

/*
count records the kind with the count fn reads.
*/
func (d *discovery) count(c engine.KindCount, fn func() (int, error)) error {
	n, err := fn()
	c.Count = n

	return d.add(c, err)
}

/*
length counts the items of a list an endpoint answers with under key.
*/
func (s *Source) length(ctx context.Context, path, key string) func() (int, error) {
	return func() (int, error) {
		var out map[string]json.RawMessage
		if err := s.Client.get(ctx, path, nil, &out); err != nil {
			return 0, err
		}

		var items []json.RawMessage
		_ = json.Unmarshal(out[key], &items)

		return len(items), nil
	}
}

/*
totalOf counts a list by its total_count.
*/
func (s *Source) totalOf(ctx context.Context, path string) func() (int, error) {
	return func() (int, error) {
		return s.total(ctx, path, url.Values{"per_page": {"1"}})
	}
}

/*
countAttributes counts the custom contact and company attributes, and
the conversation attributes when the token can read them.
*/
func (s *Source) countAttributes(ctx context.Context) (int, error) {
	n := 0

	for _, model := range []string{"contact", "company"} {
		var out list[dataAttribute]
		if err := s.Client.get(ctx, "/data_attributes", url.Values{"model": {model}}, &out); err != nil {
			return 0, err
		}

		for _, a := range out.Data {
			if a.Custom && a.DataType != "relationship" {
				n++
			}
		}
	}

	var conv list[conversationAttribute]
	if err := s.Client.get(ctx, "/conversations/attributes", nil, &conv); err == nil {
		n += len(conv.Data)
	}

	return n, nil
}

/*
countArticles counts public articles, and internal ones when the
workspace has them.
*/
func (s *Source) countArticles(ctx context.Context) (int, error) {
	n, err := s.total(ctx, "/articles", url.Values{"per_page": {"1"}})
	if err != nil {
		return 0, err
	}

	internal, err := s.total(ctx, "/internal_articles", url.Values{"per_page": {"1"}})
	if err == nil {
		n += internal
	}

	return n, nil
}

func (s *Source) countMacros(ctx context.Context) (int, error) {
	var out list[macro]
	err := s.Client.get(ctx, "/macros", url.Values{"per_page": {"150"}}, &out)

	return max(int(out.TotalCount), len(out.Data)), err
}

/*
countOf is a kind's count with its notes, before it's read.
*/
func countOf(k format.Kind, estimate bool, notes ...string) engine.KindCount {
	return engine.KindCount{Kind: k, Estimate: estimate, Notes: notes}
}

/*
Discover counts what the workspace holds, kind by kind, and finds the
permissions the token lacks. It reads counts only, never content.
*/
func (s *Source) Discover(ctx context.Context) (engine.Inventory, error) {
	acct, err := s.Account(ctx)
	if err != nil {
		return engine.Inventory{}, err
	}

	d := &discovery{inv: engine.Inventory{Account: acct, NotImported: notImported()}, missing: map[string]bool{}}

	for _, err := range s.discoverSteps(ctx, d) {
		if err != nil {
			return d.inv, err
		}
	}

	if len(s.Workflows) == 0 {
		d.inv.Warnings = append(d.inv.Warnings,
			"Workflows are only read when you list them: pass --workflow with each workflow's Intercom URL")
	}

	return d.inv, nil
}

/*
discoverSteps counts every kind, in import order, and returns what each
count failed with.
*/
func (s *Source) discoverSteps(ctx context.Context, d *discovery) []error {
	steps := []error{
		d.count(countOf(format.KindTeammate, false,
			"imported disabled: an admin enables each one in dimmi",
			"teammates removed from Intercom are added from the conversations they wrote in"),
			s.length(ctx, "/admins", "admins")),
		d.count(countOf(format.KindTeam, false), s.length(ctx, "/teams", "teams")),
		d.count(countOf(format.KindTag, false), s.length(ctx, "/tags", "data")),
		d.count(countOf(format.KindAttributeDefinition, false),
			func() (int, error) { return s.countAttributes(ctx) }),
		d.count(countOf(format.KindCompany, true, "companies without users are found while importing"),
			s.totalOf(ctx, "/companies")),
		d.count(countOf(format.KindContact, false), s.totalOf(ctx, "/contacts")),
		d.add(countOf(format.KindContactNote, true, "counted while importing"), nil),
		d.count(countOf(format.KindTicketType, false), s.length(ctx, "/ticket_types", "data")),
		d.count(countOf(format.KindHelpCenter, false, "one help center with every language it's translated into"),
			s.length(ctx, "/help_center/help_centers", "data")),
		d.count(countOf(format.KindCollection, false), s.totalOf(ctx, "/help_center/collections")),
		d.count(countOf(format.KindArticle, false,
			"with every translation; internal articles stay visible to teammates only"),
			func() (int, error) { return s.countArticles(ctx) }),
		d.count(countOf(format.KindSnippet, false, "become teammate-only articles"),
			s.totalOf(ctx, "/content_snippets")),
		d.count(countOf(format.KindSavedReply, true, "macros become saved replies; their placeholders are rewritten"),
			func() (int, error) { return s.countMacros(ctx) }),
		d.count(countOf(format.KindSchedule, false), s.length(ctx, "/office_hours_schedules", "data")),
		d.count(countOf(format.KindConversation, false, "with every part, rating and attachment"),
			func() (int, error) { return s.searchTotal(ctx, "/conversations/search") }),
		d.count(countOf(format.KindTicket, false),
			func() (int, error) { return s.searchTotal(ctx, "/tickets/search") }),
	}

	if len(s.Workflows) > 0 {
		steps = append(steps, d.add(engine.KindCount{
			Kind: format.KindWorkflow, Count: len(s.Workflows), Notes: []string{"imported disabled"},
		}, nil))
	}

	return steps
}
