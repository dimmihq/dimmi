package conversations_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appevents "github.com/dimmihq/dimmi/app/events"
)

func TestConversationPriority(t *testing.T) {
	t.Parallel()

	const all = "/api/v1/conversations?filter[view]=all"

	t.Run("sets a level and shows it, a repeat changing nothing", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		seen := record[appevents.ConversationPriorityChanged](d.tc)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Please")

		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.priority", "none")
		d.owner.PatchJSON(path, testkit.JSON{"priority": "urgent"}).AssertOK().
			AssertJSONPath("data.priority", "urgent").AssertJSONPath("meta.changed", []any{"priority"})
		d.owner.PatchJSON(path, testkit.JSON{"priority": "urgent"}).AssertOK().
			AssertJSONPath("meta.changed", []any{})

		events := seen.all()
		require.Len(t, events, 1)
		assert.Equal(t, "none", events[0].From)
		assert.Equal(t, "urgent", events[0].To)
	})

	t.Run("refuses a value that is not a level", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Please")

		d.owner.PatchJSON(path, testkit.JSON{"priority": "extreme"}).AssertStatus(422)
		d.owner.PatchJSON(path, testkit.JSON{"priority": true}).AssertStatus(422)
		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.priority", "none")
	})

	t.Run("filters by a comma list of levels and refuses the old flag", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		high, urgent, low := d.customer(t, "a", "A", "a"), d.customer(t, "b", "B", "b"), d.customer(t, "c", "C", "c")
		d.owner.PatchJSON("/api/v1/conversations/"+high, testkit.JSON{"priority": "high"}).AssertOK()
		d.owner.PatchJSON("/api/v1/conversations/"+urgent, testkit.JSON{"priority": "urgent"}).AssertOK()
		d.owner.PatchJSON("/api/v1/conversations/"+low, testkit.JSON{"priority": "low"}).AssertOK()

		assert.ElementsMatch(t, []string{high, urgent}, listed(d.owner.GetJSON(all+"&filter[priority]=high,urgent")))
		d.owner.GetJSON(all + "&filter[priority]=true").AssertStatus(400)
	})

	t.Run("searches by any level, one level and at least a level", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		medium, urgent, none := d.customer(t, "a", "A", "a"), d.customer(t, "b", "B", "b"), d.customer(t, "c", "C", "c")
		d.owner.PatchJSON("/api/v1/conversations/"+medium, testkit.JSON{"priority": "medium"}).AssertOK()
		d.owner.PatchJSON("/api/v1/conversations/"+urgent, testkit.JSON{"priority": "urgent"}).AssertOK()

		assert.ElementsMatch(t, []string{medium, urgent}, listed(d.owner.GetJSON(all+"&q=is:priority")))
		assert.Equal(t, []string{urgent}, listed(d.owner.GetJSON(all+"&q=priority:urgent")))
		assert.ElementsMatch(t, []string{medium, urgent}, listed(d.owner.GetJSON(all+"&q=priority:%3E%3Dmedium")))
		assert.Empty(t, listed(d.owner.GetJSON(all+"&q=priority:nonsense")))
		assert.NotContains(t, listed(d.owner.GetJSON(all+"&q=is:priority")), none)
	})

	t.Run("sorts by priority across pages without repeats or gaps", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		levels := []string{"low", "urgent", "none", "high", "urgent", "medium", "none", "high"}
		byLevel := map[string][]string{}

		for i, level := range levels {
			d.tc.Clock.Travel(time.Minute)
			id := d.customer(t, fmt.Sprintf("c%d", i), "S", "b")
			byLevel[level] = append([]string{id}, byLevel[level]...)
			d.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"priority": level}).AssertOK()
		}

		var want []string
		for _, level := range []string{"urgent", "high", "medium", "low", "none"} {
			want = append(want, byLevel[level]...)
		}

		assert.Equal(t, want, walk(t, d, all+"&sort=-priority&limit=3"))
	})
}

/*
walk follows a list's pages to the end and returns every row, failing
on a page that repeats one.
*/
func walk(t *testing.T, d desk, path string) []string {
	t.Helper()

	var rows []string

	seen := map[string]bool{}
	cursor := ""

	for range 50 {
		page := path
		if cursor != "" {
			page += "&cursor=" + cursor
		}

		res := d.owner.GetJSON(page).AssertOK()
		for _, id := range listed(res) {
			require.False(t, seen[id], "row repeated across pages")
			seen[id] = true
			rows = append(rows, id)
		}

		cursor = res.String("meta.next_cursor")
		if cursor == "" {
			return rows
		}
	}

	require.Fail(t, "list never ended")

	return rows
}
