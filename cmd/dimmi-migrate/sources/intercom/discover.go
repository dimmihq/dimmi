package intercom

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/engine"
	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/format"
)

/*
permissions name the Intercom private-app permission each kind needs.
*/
var permissions = map[format.Kind]string{
	format.KindTeammate:            "Read admins",
	format.KindTeam:                "Read admins",
	format.KindTag:                 "Read tags",
	format.KindAttributeDefinition: "Read data attributes",
	format.KindCompany:             "Read one company / Read and list companies",
	format.KindContact:             "Read and list users and companies",
	format.KindContactNote:         "Read and list users and companies",
	format.KindTicketType:          "Read tickets",
	format.KindHelpCenter:          "Read and list articles",
	format.KindCollection:          "Read and list articles",
	format.KindArticle:             "Read and list articles",
	format.KindSnippet:             "Read content",
	format.KindSavedReply:          "Read conversations",
	format.KindSchedule:            "Read admins",
	format.KindConversation:        "Read conversations",
	format.KindTicket:              "Read tickets",
	format.KindWorkflow:            "Read workflows",
}

/*
notImported is what a migration from Intercom leaves behind, shown in
the plan so nobody is surprised.
*/
var notImported = []string{
	"Segments: they are rules, rebuild them as views or audiences",
	"Outbound messages, series, banners, tours and news items",
	"Fin and bot settings, and workflows you didn't list with --workflow",
	"Reports, statistics and SLA outcomes: dimmi works out reply and close times from the history",
	"Device, browser, UTM and social profile fields, subscription types and SMS consent",
	"Custom objects and relationship attributes",
	"Tickets beyond the 500 most recent parts each, which Intercom doesn't return",
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
Discover counts what the workspace holds, kind by kind, and finds the
permissions the token lacks. It reads counts only, never content.
*/
func (s *Source) Discover(ctx context.Context) (engine.Inventory, error) {
	acct, err := s.Account(ctx)
	if err != nil {
		return engine.Inventory{}, err
	}

	inv := engine.Inventory{Account: acct, NotImported: notImported}
	missing := map[string]bool{}

	add := func(kind format.Kind, count int, err error, estimate bool, notes ...string) error {
		switch {
		case isForbidden(err):
			if p := permissions[kind]; p != "" && !missing[p] {
				missing[p] = true
				inv.Missing = append(inv.Missing, p)
			}

			inv.Kinds = append(inv.Kinds, engine.KindCount{Kind: kind, Notes: []string{"the token can't read these"}})
		case isNotFound(err):
			inv.Kinds = append(inv.Kinds, engine.KindCount{Kind: kind, Notes: []string{"not available on this workspace"}})
		case err != nil:
			return err
		default:
			inv.Kinds = append(inv.Kinds, engine.KindCount{Kind: kind, Count: count, Estimate: estimate, Notes: notes})
		}

		return nil
	}

	counter := func(kind format.Kind, fn func() (int, error), estimate bool, notes ...string) error {
		n, err := fn()

		return add(kind, n, err, estimate, notes...)
	}

	length := func(path string, q url.Values, key string) func() (int, error) {
		return func() (int, error) {
			var out map[string]json.RawMessage
			if err := s.Client.get(ctx, path, q, &out); err != nil {
				return 0, err
			}

			var items []json.RawMessage
			_ = json.Unmarshal(out[key], &items)

			return len(items), nil
		}
	}

	steps := []error{
		counter(format.KindTeammate, length("/admins", nil, "admins"), false,
			"imported disabled: an admin enables each one in dimmi",
			"teammates removed from Intercom are added from the conversations they wrote in"),
		counter(format.KindTeam, length("/teams", nil, "teams"), false),
		counter(format.KindTag, length("/tags", nil, "data"), false),
		counter(format.KindAttributeDefinition, func() (int, error) {
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
		}, false),
		counter(format.KindCompany, func() (int, error) {
			return s.total(ctx, "/companies", url.Values{"per_page": {"1"}})
		}, true, "companies without users are found while importing"),
		counter(format.KindContact, func() (int, error) {
			return s.total(ctx, "/contacts", url.Values{"per_page": {"1"}})
		}, false),
		add(format.KindContactNote, 0, nil, true, "counted while importing"),
		counter(format.KindTicketType, length("/ticket_types", nil, "data"), false),
		counter(format.KindHelpCenter, length("/help_center/help_centers", nil, "data"), false,
			"one help center with every language it's translated into"),
		counter(format.KindCollection, func() (int, error) {
			return s.total(ctx, "/help_center/collections", url.Values{"per_page": {"1"}})
		}, false),
		counter(format.KindArticle, func() (int, error) {
			n, err := s.total(ctx, "/articles", url.Values{"per_page": {"1"}})
			if err != nil {
				return 0, err
			}

			internal, err := s.total(ctx, "/internal_articles", url.Values{"per_page": {"1"}})
			if err == nil {
				n += internal
			}

			return n, nil
		}, false, "with every translation; internal articles stay visible to teammates only"),
		counter(format.KindSnippet, func() (int, error) {
			return s.total(ctx, "/content_snippets", url.Values{"per_page": {"1"}})
		}, false, "become teammate-only articles"),
		counter(format.KindSavedReply, func() (int, error) {
			var out list[macro]
			err := s.Client.get(ctx, "/macros", url.Values{"per_page": {"150"}}, &out)

			return max(out.TotalCount, len(out.Data)), err
		}, true, "macros become saved replies; their placeholders are rewritten"),
		counter(format.KindSchedule, length("/office_hours_schedules", nil, "data"), false),
		counter(format.KindConversation, func() (int, error) {
			return s.searchTotal(ctx, "/conversations/search")
		}, false, "with every part, rating and attachment"),
		counter(format.KindTicket, func() (int, error) {
			return s.searchTotal(ctx, "/tickets/search")
		}, false),
	}

	if len(s.Workflows) > 0 {
		steps = append(steps, add(format.KindWorkflow, len(s.Workflows), nil, false, "imported disabled"))
	}

	for _, err := range steps {
		if err != nil {
			return inv, err
		}
	}

	if len(s.Workflows) == 0 {
		inv.Warnings = append(inv.Warnings,
			"Workflows are only read when you list them: pass --workflow with each workflow's Intercom URL")
	}

	return inv, nil
}
