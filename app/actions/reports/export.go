/*
|--------------------------------------------------------------------------
| Facts Export
|--------------------------------------------------------------------------
|
| Every conversation's facts can leave as a spreadsheet, one row per
| conversation started in the range, with the same filters and access as
| the reports, so a team can check any number against its rows or carry
| them into its own tools. Ids are public ids, times are UTC and
| durations are seconds.
|
*/

package reports

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
factColumns are the export's columns and the facts they read.
*/
func factColumns() [][2]string {
	return [][2]string{
		{"conversation_id", "f.conversation_id"},
		{"number", "f.number"},
		{"inbox_id", "f.inbox_id"},
		{"channel", "f.channel"},
		{"status", "f.status"},
		{"priority", "f.priority"},
		{"assignee_id", "f.assignee_member_id"},
		{"team_id", "f.assignee_team_id"},
		{"imported", "f.imported"},
		{"started_at", "f.started_at"},
		{"customer_messages", "f.customer_messages"},
		{"first_response_at", "f.first_response_at"},
		{"first_response_seconds", "f.first_response_seconds"},
		{"first_response_business_seconds", "f.first_response_business_seconds"},
		{"first_responder_id", "f.first_responder_id"},
		{"first_answer_seconds", "f.first_answer_seconds"},
		{"human_replies", "f.human_replies"},
		{"first_closed_at", "f.first_closed_at"},
		{"closed_at", "f.closed_at"},
		{"resolution_seconds", "f.resolution_seconds"},
		{"resolution_business_seconds", "f.resolution_business_seconds"},
		{"closes", "f.closes"},
		{"reopens", "f.reopens"},
		{"first_contact_resolved", "f.first_contact_resolved"},
		{"closed_by_id", "f.closed_by_member_id"},
		{"rating", "f.rating"},
		{"rated_at", "f.rated_at"},
		{"ai_involved", "f.ai_involved"},
		{"ai_answers", "f.ai_answers"},
		{"ai_handed_off_at", "f.ai_handed_off_at"},
		{"ai_handoff_reason", "f.ai_handoff_reason"},
		{"ai_confirmed", "f.ai_confirmed"},
		{"ai_cost_micros", "f.ai_cost_micros"},
	}
}

/*
idPrefixes says which columns hold ids and how they read publicly.
*/
func idPrefixes() map[string]models.Prefix {
	return map[string]models.Prefix{
		"conversation_id": models.PrefixConversation, "inbox_id": models.PrefixInbox,
		"assignee_id": models.PrefixMember, "team_id": models.PrefixTeam, "first_responder_id": models.PrefixMember,
		"closed_by_id": models.PrefixMember,
	}
}

/*
FactScope is what a facts export reads: as whom, with which filters, at
which moment, and how many rows at most.
*/
type FactScope struct {
	Membership authz.Membership
	Filters    Filters
	Now        time.Time
	Limit      int
}

/*
CountFacts returns how many conversations the export would cover, up to
the scope's limit.
*/
func CountFacts(ctx context.Context, tx *database.Tx, scope FactScope) (int, error) {
	q, err := NewQuery(scope.Membership, scope.Filters, scope.Now)
	if err != nil {
		return 0, err
	}

	var s statement

	sql := fmt.Sprintf("SELECT count(*) FROM (SELECT 1 FROM conversation_facts f WHERE %s LIMIT %d) x",
		s.where(q, "f.started_at", q.Start, q.End), scope.Limit)

	var n int
	err = rows(ctx, tx, prepared{sql: sql, args: s.args}, func(scan func(...any) error) error { return scan(&n) })

	return n, err
}

/*
FactRows returns the export's header and rows, up to the scope's limit,
oldest first.
*/
func FactRows(ctx context.Context, tx *database.Tx, scope FactScope) ([][]string, error) {
	q, err := NewQuery(scope.Membership, scope.Filters, scope.Now)
	if err != nil {
		return nil, err
	}

	cols := factColumns()
	header := make([]string, len(cols))

	var selects strings.Builder
	for i, c := range cols {
		header[i] = c[0]
		if i > 0 {
			selects.WriteString(", ")
		}

		selects.WriteString(c[1] + "::text")
	}

	var s statement

	sql := fmt.Sprintf(
		"SELECT %s FROM conversation_facts f WHERE %s ORDER BY f.started_at, f.conversation_id LIMIT %d",
		selects.String(), s.where(q, "f.started_at", q.Start, q.End), scope.Limit,
	)

	out := [][]string{header}
	err = rows(ctx, tx, prepared{sql: sql, args: s.args}, func(scan func(...any) error) error {
		cells := make([]*string, len(cols))
		targets := make([]any, len(cols))
		for i := range cells {
			targets[i] = &cells[i]
		}

		if err := scan(targets...); err != nil {
			return err
		}

		out = append(out, factLine(header, cells))

		return nil
	})

	return out, err
}

/*
factLine writes one row: ids as public ids, times as RFC 3339 in UTC,
nothing as an empty cell.
*/
func factLine(header []string, cells []*string) []string {
	line := make([]string, len(cells))
	for i, c := range cells {
		if c == nil {
			continue
		}

		line[i] = *c
		if prefix, ok := idPrefixes()[header[i]]; ok {
			if id, err := ids.Parse(*c); err == nil {
				line[i] = models.PublicID(prefix, id)
			}
		}

		if t, err := time.Parse("2006-01-02 15:04:05.999999-07", *c); err == nil {
			line[i] = t.UTC().Format(time.RFC3339)
		}
	}

	return line
}
