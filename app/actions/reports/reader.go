/*
|--------------------------------------------------------------------------
| Reading Facts
|--------------------------------------------------------------------------
|
| The shapes every conversation report reads facts in: one row of
| figures over the range, one row per local day, or one row per group
| such as channel or teammate. The facts are aliased f; reply and member
| facts join them so every filter applies to them too.
|
*/

package reports

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
The tables figures are read from.
*/
const (
	fromFacts   = "conversation_facts f"
	fromReplies = "conversation_reply_facts r JOIN conversation_facts f ON f.conversation_id = r.conversation_id"
	fromMembers = "conversation_member_facts mf JOIN conversation_facts f ON f.conversation_id = mf.conversation_id"
)

/*
reader reads figures for one query.
*/
type reader struct {
	tx  *database.Tx
	q   Query
	pre []any
}

/*
figures describes what to read: from where, dated by which column,
which columns, and a condition beyond the filters. Columns that bind
values bind them first, to the reader's pre.
*/
type figures struct {
	from  string
	dated string
	cols  string
	extra string
}

/*
sql returns the statement and its values, grouped by the expression
when one is given; the group is the first column read.
*/
func (r reader) sql(f figures, group string) (string, []any) {
	s := statement{args: append([]any(nil), r.pre...)}

	where := s.where(r.q, f.dated, r.q.Start, r.q.End)
	if f.extra != "" {
		where += "\n  AND " + f.extra
	}

	if group == "" {
		return fmt.Sprintf("SELECT %s\nFROM %s\nWHERE %s", f.cols, f.from, where), s.args
	}

	switch group {
	case groupDay:
		group = s.localDay(r.q, f.dated)
	case groupHour:
		group = fmt.Sprintf("to_char(%s AT TIME ZONE %s, 'ID-HH24')", f.dated, s.arg(r.q.Location.String()))
	}

	return fmt.Sprintf(
		"SELECT COALESCE((%s)::text, '') AS g, %s\nFROM %s\nWHERE %s\nGROUP BY 1\nORDER BY 1",
		group, f.cols, f.from, where,
	), s.args
}

/*
values is one row of figures, each a number or nil, after the group's
key when grouped.
*/
type values struct {
	key  string
	nums []*float64
}

/*
at returns the figure at the index, nil past the end.
*/
func (v values) at(i int) *float64 {
	if i >= len(v.nums) {
		return nil
	}

	return v.nums[i]
}

/*
count returns the figure at the index as a whole number, 0 for nil.
*/
func (v values) count(i int) int64 {
	if p := v.at(i); p != nil {
		return int64(*p)
	}

	return 0
}

/*
read runs the statement for n figures per row.
*/
func (r reader) read(ctx context.Context, f figures, group string, n int) ([]values, error) {
	sql, args := r.sql(f, group)

	var out []values
	err := rows(ctx, r.tx, prepared{sql: sql, args: args}, func(scan func(...any) error) error {
		v := values{nums: make([]*float64, n)}
		targets := make([]any, 0, n+1)
		if group != "" {
			targets = append(targets, &v.key)
		}

		for i := range v.nums {
			targets = append(targets, &v.nums[i])
		}

		if err := scan(targets...); err != nil {
			return err
		}

		out = append(out, v)

		return nil
	})

	return out, err
}

/*
one reads a single row of n figures.
*/
func (r reader) one(ctx context.Context, f figures, n int) (values, error) {
	out, err := r.read(ctx, f, "", n)
	if err != nil || len(out) == 0 {
		return values{nums: make([]*float64, n)}, err
	}

	return out[0], nil
}
