/*
|--------------------------------------------------------------------------
| Listing a Drill-down
|--------------------------------------------------------------------------
|
| The list reads the facts the number was counted from, joined to the
| conversations for their subject, and counts how many there are up to
| the cap.
|
*/

package reports

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
list reads one page of the drill-down and its total.
*/
func (d drill) list(ctx context.Context, tx *database.Tx, q Query, in DrillInput) (DrillPage, error) {
	limit := min(max(in.Limit, 1), MaxDrillPage)
	offset := min(max(in.Offset, 0), MaxDrill)

	var s statement
	where := s.where(q, d.dated, q.Start, q.End)
	if d.extra != nil {
		where += "\n  AND " + d.extra(&s, q)
	}

	value, order := "NULL::float8", "f.started_at DESC"
	if d.value != "" {
		value = d.value + "::float8"
		order = d.value + " DESC NULLS LAST"
	}

	if d.order != "" {
		order = d.order
	}

	total, err := countDrill(ctx, tx, prepared{sql: where, args: s.args})
	if err != nil {
		return DrillPage{}, err
	}

	listSQL := fmt.Sprintf(`SELECT f.conversation_id, f.number, c.subject, f.status, f.channel, f.started_at,
    f.closed_at, f.assignee_member_id, %s
FROM conversation_facts f
JOIN conversations c ON c.id = f.conversation_id
WHERE %s
ORDER BY %s, f.conversation_id
LIMIT %d OFFSET %d`, value, where, order, limit, offset)

	out := DrillPage{Total: total, Capped: total >= MaxDrill}
	if offset >= MaxDrill {
		return out, nil
	}

	err = rows(ctx, tx, prepared{sql: listSQL, args: s.args}, func(scan func(...any) error) error {
		var r DrillRow
		if err := scan(&r.ConversationID, &r.Number, &r.Subject, &r.Status, &r.Channel, &r.StartedAt, &r.ClosedAt,
			&r.AssigneeMemberID, &r.Value); err != nil {
			return err
		}

		out.Rows = append(out.Rows, r)

		return nil
	})

	return out, err
}

/*
countDrill counts the facts matching the condition, up to MaxDrill.
*/
func countDrill(ctx context.Context, tx *database.Tx, where prepared) (int64, error) {
	var total int64

	countSQL := fmt.Sprintf("SELECT count(*) FROM (SELECT 1 FROM conversation_facts f WHERE %s LIMIT %d) x",
		where.sql, MaxDrill)
	count := prepared{sql: countSQL, args: where.args}
	err := rows(ctx, tx, count, func(scan func(...any) error) error { return scan(&total) })

	return total, err
}
