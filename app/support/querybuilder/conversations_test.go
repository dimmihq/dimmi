package querybuilder_test

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
placeholders returns the highest $n placeholder of the query.
*/
func placeholders(sql string) int {
	most := 0
	for _, match := range regexp.MustCompile(`\$(\d+)`).FindAllStringSubmatch(sql, -1) {
		n, _ := strconv.Atoi(match[1])
		most = max(most, n)
	}

	return most
}

func TestConversations(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	id, other := ids.New(), ids.New()

	t.Run("narrows by every filter, each value a placeholder", func(t *testing.T) {
		t.Parallel()
		minimum, number, query := int16(2), int64(7), "refund"
		list := querybuilder.Conversations{
			WorkspaceID: id, MemberID: other, Statuses: []string{"open", "snoozed"}, Unassigned: true, NoTeam: true,
			AssigneeID: &id, TeamID: &id, InboxID: &id, ContactID: &id, CompanyID: &id,
			ConnectionIDs: []ids.UUID{id}, ChannelTypes: []string{"email"}, Priorities: []int16{3},
			PriorityMin: &minimum, Number: &number, Unread: true, Waiting: true, Mentioned: true,
			TagsAll: []ids.UUID{id, other}, TagsAny: []ids.UUID{id}, CreatedAfter: &at, CreatedBefore: &at,
			TSQuery: &query, Limit: 25,
			Extra: []querybuilder.Fragment{func(next int) (string, []any) {
				return "c.subject = $" + strconv.Itoa(next), []any{"x"}
			}},
		}

		sql, args := list.IDs()
		for _, part := range []string{
			"c.status = ANY", "c.assignee_member_id IS NULL", "c.assignee_team_id IS NULL", "c.inbox_id =",
			"c.contact_id =", "ct.company_id =", "c.channel_connection_id = ANY", "cc.type = ANY",
			"c.priority = ANY", "c.priority >=", "c.number =", "c.waiting_since IS NOT NULL", "c.created_at >=",
			"c.created_at <", "ct.tag_id = ANY", "m.undone_at IS NULL", "mm.workspace_member_id =",
			"to_tsquery('dimmi_search'", "c.subject = $", "LIMIT $", "c.spam = false",
		} {
			assert.Contains(t, sql, part)
		}
		assert.Equal(t, len(args), placeholders(sql))

		count, countArgs := list.Count(1000)
		assert.Contains(t, count, "SELECT count(*) FROM (SELECT 1 FROM conversations c WHERE")
		assert.Equal(t, int64(1000), countArgs[len(countArgs)-1])
	})

	t.Run("lists one status by equality and the trash without the spam flag", func(t *testing.T) {
		t.Parallel()

		sql, _ := querybuilder.Conversations{WorkspaceID: id, Statuses: []string{"closed"}, Deleted: true}.IDs()

		assert.Contains(t, sql, "c.status = $2")
		assert.Contains(t, sql, "c.deleted_at IS NOT NULL")
		assert.NotContains(t, sql, "c.spam")
	})

	t.Run("orders and pages by each sort, forwards and back, past a null too", func(t *testing.T) {
		t.Parallel()

		for _, sort := range append(querybuilder.Sorts(), "") {
			for _, after := range []querybuilder.Keyset{{Priority: 2, At: &at, ID: id}, {Priority: 2, ID: id}} {
				for _, reverse := range []bool{false, true} {
					sql, args := querybuilder.Conversations{
						WorkspaceID: id, Sort: sort, After: &after, Reverse: reverse,
					}.IDs()

					assert.Contains(t, sql, " ORDER BY ")
					assert.Contains(t, sql, "c.id")
					assert.Equal(t, len(args), placeholders(sql), string(sort))
				}
			}
		}
	})

	t.Run("walks a mixed order key by key and a null cursor past the nulls", func(t *testing.T) {
		t.Parallel()

		sql, _ := querybuilder.Conversations{
			WorkspaceID: id, Sort: querybuilder.SortPriorityWaiting,
			After: &querybuilder.Keyset{Priority: 2, At: &at, ID: id},
		}.IDs()
		assert.Contains(t, sql,
			"c.priority < $4 OR (c.priority = $4 AND (c.waiting_since > $3 OR c.waiting_since IS NULL")

		sql, _ = querybuilder.Conversations{
			WorkspaceID: id, Sort: querybuilder.SortLatest, After: &querybuilder.Keyset{ID: id},
		}.IDs()
		assert.Contains(t, sql, "c.last_message_at IS NOT NULL OR (c.last_message_at IS NULL AND (c.id < $2))")

		sql, _ = querybuilder.Conversations{
			WorkspaceID: id, Sort: querybuilder.SortWaiting, After: &querybuilder.Keyset{ID: id},
		}.IDs()
		assert.Contains(t, sql, "c.waiting_since IS NULL AND (c.id > $2)")

		sql, _ = querybuilder.Conversations{
			WorkspaceID: id, Sort: querybuilder.SortOldest, After: &querybuilder.Keyset{At: &at, ID: id},
		}.IDs()
		assert.Contains(t, sql, "(c.last_message_at, c.id) > ($2, $3) OR c.last_message_at IS NULL")
	})

	t.Run("reads the time each sort orders by", func(t *testing.T) {
		t.Parallel()
		last, waiting, created, wakes := at, at.Add(time.Hour), at.Add(2*time.Hour), at.Add(3*time.Hour)

		for sort, want := range map[querybuilder.Sort]time.Time{
			querybuilder.SortLatest: last, querybuilder.SortOldest: last, querybuilder.SortPriority: last,
			querybuilder.SortWaiting: waiting, querybuilder.SortPriorityWaiting: waiting,
			querybuilder.SortNewestStarted: created, querybuilder.SortOldestStarted: created,
			querybuilder.SortSnoozeEnds: wakes, "": last,
		} {
			assert.Equal(t, want, *sort.TimeOf(&last, &waiting, &created, &wakes), string(sort))
		}
	})
}
