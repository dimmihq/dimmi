/*
|--------------------------------------------------------------------------
| Report SQL
|--------------------------------------------------------------------------
|
| Reports are read from the facts with SQL put together for each
| request: only the filters asked for become clauses, so every mix walks
| the workspace's index on the date it is counted by. Columns and dates
| come from fixed names here, never from the request; every value is a
| bound parameter. The reader's access is one more clause, the same
| visibility rule the inbox applies, left out when it would show
| everything anyway.
|
*/

package reports

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
)

/*
statement collects a statement's bound values.
*/
type statement struct {
	args []any
}

/*
arg binds a value and returns its placeholder.
*/
func (s *statement) arg(v any) string {
	s.args = append(s.args, v)

	return fmt.Sprintf("$%d", len(s.args))
}

/*
where returns the clauses that pick the facts a query counts: the
workspace, the dated column inside the range, the filters and access.
The facts are aliased f; the dated column is named with its alias.
*/
func (s *statement) where(q Query, dated string, start, end time.Time) string {
	clauses := []string{
		"f.workspace_id = " + s.arg(q.Membership.Workspace.ID),
	}

	if dated != "" {
		clauses = append(clauses, fmt.Sprintf("%s >= %s AND %s < %s", dated, s.arg(start), dated, s.arg(end)))
	}

	return strings.Join(append(clauses, s.filters(q)...), "\n  AND ")
}

/*
filters returns the clauses of the filters set and of access.
*/
func (s *statement) filters(q Query) []string {
	f := q.Filters
	var out []string

	lists := []struct {
		column string
		values any
		n      int
		cast   string
	}{
		{"f.inbox_id", f.Inboxes, len(f.Inboxes), "uuid[]"},
		{"f.assignee_team_id", f.Teams, len(f.Teams), "uuid[]"},
		{"f.assignee_member_id", f.Members, len(f.Members), "uuid[]"},
		{"f.channel", f.Channels, len(f.Channels), "text[]"},
		{"f.priority", f.Priorities, len(f.Priorities), "smallint[]"},
	}
	for _, l := range lists {
		if l.n > 0 {
			out = append(out, fmt.Sprintf("%s = ANY (%s::%s)", l.column, s.arg(l.values), l.cast))
		}
	}

	if len(f.Tags) > 0 {
		out = append(out, "EXISTS (SELECT 1 FROM conversation_tag ct WHERE ct.conversation_id = f.conversation_id"+
			" AND ct.tag_id = ANY ("+s.arg(f.Tags)+"::uuid[]))")
	}

	switch f.AI {
	case AIInvolved:
		out = append(out, "f.ai_involved")
	case AINotInvolved:
		out = append(out, "NOT f.ai_involved")
	}

	return append(out, s.access(q)...)
}

/*
access returns the visibility clause, none when the reader sees every
conversation.
*/
func (s *statement) access(q Query) []string {
	p := conversations.VisibleScope(q.Membership).Params()
	if p.Mode == "all" && len(p.HiddenInboxes) == 0 {
		return nil
	}

	return []string{fmt.Sprintf(
		"dimmi_conversation_visible(f.assignee_member_id, f.assignee_team_id, f.inbox_id, %s::text, %s::uuid, "+
			"%s::uuid[], %s::boolean, %s::uuid[])",
		s.arg(p.Mode), s.arg(p.Viewer), s.arg(p.Teams), s.arg(p.Unassigned), s.arg(p.HiddenInboxes),
	)}
}

/*
seconds returns the duration column to read, calendar or office hours.
The names come from the facts' own columns.
*/
func seconds(q Query, column string) string {
	if q.Filters.BusinessHours {
		return strings.TrimSuffix(column, "seconds") + "business_seconds"
	}

	return column
}

/*
localDay returns the expression for the local day of a dated column.
*/
func (s *statement) localDay(q Query, column string) string {
	return fmt.Sprintf("to_char((%s AT TIME ZONE %s)::date, 'YYYY-MM-DD')", column, s.arg(q.Location.String()))
}

/*
prepared is a statement ready to run with the values it binds.
*/
type prepared struct {
	sql  string
	args []any
}

/*
rows runs a statement and hands each row's scan to the callback.
*/
func rows(ctx context.Context, tx *database.Tx, p prepared, scan func(func(...any) error) error) error {
	r, err := tx.Query(ctx, p.sql, p.args...)
	if err != nil {
		return fmt.Errorf("run report query: %w", err)
	}
	defer r.Close()

	for r.Next() {
		if err := scan(r.Scan); err != nil {
			return fmt.Errorf("read report row: %w", err)
		}
	}

	if err := r.Err(); err != nil {
		return fmt.Errorf("read report rows: %w", err)
	}

	return nil
}

/*
durationStats is the part of a select that sums up a duration column:
how many, the median, the 90th percentile and the mean.
*/
func durationStats(column string) string {
	return fmt.Sprintf("count(%[1]s)::bigint, percentile_cont(0.5) WITHIN GROUP (ORDER BY %[1]s)::float8, "+
		"percentile_cont(0.9) WITHIN GROUP (ORDER BY %[1]s)::float8, avg(%[1]s)::float8", column)
}

/*
Duration is a duration column summed up, in seconds; the figures are
nil when nothing was counted.
*/
type Duration struct {
	Count  int64
	Median *float64
	P90    *float64
	Mean   *float64
}
