package conversations_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestWorkbenchRules(t *testing.T) {
	t.Parallel()

	const views = "/api/v1/conversation-views"

	t.Run("refuses view icons, orders, statuses and names no list has", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		open := oneCondition("status", "in", []string{"open"})

		for path, body := range map[string]testkit.JSON{
			"icon":          {"icon": "unicorn"},
			"sort":          {"sort": "name"},
			"status_filter": {"status_filter": "pending"},
		} {
			body["scope"], body["definition"] = "personal", open
			if body["name"] == nil {
				body["name"] = "View"
			}

			d.owner.PostJSON(views, body).AssertStatus(422).AssertJSONPath("code", "invalid_definition").
				AssertJSONPath("path", path)
		}

		id := saveView(t, d.owner, "personal", open)
		d.owner.PatchJSON(views+"/"+id, testkit.JSON{"sort": "name"}).AssertStatus(422).AssertJSONPath("path", "sort")
	})

	t.Run("refuses a view search that is too long or holds no words", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		for _, text := range []string{strings.Repeat("refund ", 50), "&&& |||"} {
			d.owner.PostJSON(views, testkit.JSON{
				"name": "Search", "scope": "personal", "definition": oneCondition("text", "matches", text),
			}).AssertStatus(422).AssertJSONPath("path", "groups.0.conditions.0.value")
		}
	})

	t.Run("lists a view with its own status and order unless the request says", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		closed := d.customer(t, "ann", "Closed one", "Hi")
		d.owner.PatchJSON("/api/v1/conversations/"+closed, testkit.JSON{"status": "closed"}).AssertOK()
		d.tc.Clock.Travel(time.Minute)
		newest := d.customer(t, "bob", "Newest", "Hi")

		id := d.owner.PostJSON(views, testkit.JSON{
			"name": "Everything", "scope": "personal", "status_filter": "any", "sort": "created_at",
			"definition": oneCondition("message_count", "gt", 0),
		}).AssertCreated().String("data.id")

		d.owner.GetJSON("/api/v1/conversations?filter[view]="+id).AssertJSONPath("data.#", 2).
			AssertJSONPath("data.0.id", closed)
		d.owner.GetJSON("/api/v1/conversations?filter[view]="+id+"&filter[status]=open&sort=-created_at").
			AssertJSONPath("data.#", 1).AssertJSONPath("data.0.id", newest)
	})

	t.Run("refuses snooze options their kind cannot resolve", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		for _, option := range []testkit.JSON{
			{"key": "a", "label": "A", "kind": "preset", "preset": "someday"},
			{"key": "a", "label": "A", "kind": "day_at", "days_ahead": 1, "at": "25:00"},
			{"key": "a", "label": "A", "kind": "day_at", "days_ahead": 0, "at": "09:00"},
			{"key": "a", "label": "A", "kind": "weekday_at", "weekday": 8, "at": "09:00"},
			{"key": "a", "label": "A", "kind": "fortnight"},
			{"key": "", "label": "A", "kind": "in", "minutes": 30},
		} {
			d.owner.PatchJSON("/api/v1/snooze-lists/workspace", testkit.JSON{"options": []testkit.JSON{option}}).
				AssertStatus(422).AssertJSONPath("code", "invalid_snooze_options")
		}

		twice := testkit.JSON{"key": "a", "label": "A", "kind": "in", "minutes": 30}
		d.owner.PatchJSON("/api/v1/snooze-lists/workspace", testkit.JSON{"options": []testkit.JSON{twice, twice}}).
			AssertStatus(422)
	})

	t.Run("resolves day and weekday options in the reader's time zone, never today", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		conv := d.customer(t, "ann", "Later", "Hi")
		now := d.tc.Clock.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}

		d.owner.PatchJSON("/api/v1/snooze-lists/workspace", testkit.JSON{"options": []testkit.JSON{
			{"key": "tomorrow_nine", "label": "Tomorrow 9", "kind": "day_at", "days_ahead": 1, "at": "09:00"},
			{"key": "same_day", "label": "Next same day", "kind": "weekday_at", "weekday": weekday, "at": "08:30"},
		}}).AssertOK()

		res := d.owner.GetJSON("/api/v1/snooze-options?conversation=" + conv).AssertOK()
		tomorrow, _ := time.Parse(time.RFC3339, res.String("data.0.until"))
		nextWeek, _ := time.Parse(time.RFC3339, res.String("data.1.until"))

		assert.True(t, tomorrow.After(now) && tomorrow.Before(now.Add(48*time.Hour)))
		assert.True(t, nextWeek.After(now.Add(6*24*time.Hour)) && nextWeek.Before(now.Add(8*24*time.Hour)))
	})

	t.Run("changes each part of a view and refuses what it cannot hold", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := saveView(t, d.owner, "personal", oneCondition("status", "in", []string{"open"}))
		taken := saveView(t, d.owner, "personal", oneCondition("status", "in", []string{"closed"}))
		path := views + "/" + id

		d.owner.PatchJSON(path, testkit.JSON{
			"icon": "star", "sort": "created_at", "status_filter": "any",
			"definition": oneCondition("status", "in", []string{"snoozed"}),
		}).AssertOK().AssertJSONPath("data.icon", "star").AssertJSONPath("data.sort", "created_at").
			AssertJSONPath("data.status_filter", "any")
		d.owner.PatchJSON(path, testkit.JSON{"icon": "", "sort": "", "status_filter": ""}).AssertOK().
			AssertJSONPath("data.icon", nil).AssertJSONPath("data.sort", nil)

		name := d.owner.GetJSON(views + "/" + taken).String("data.name")
		d.owner.PatchJSON(path, testkit.JSON{"name": name}).AssertStatus(422).AssertJSONPath("code", "view_name_taken")
		d.owner.PatchJSON(path, testkit.JSON{"name": "   "}).AssertStatus(422)
		d.owner.PatchJSON(path, testkit.JSON{"definition": testkit.JSON{"op": "xor"}}).AssertStatus(422).
			AssertJSONPath("path", "op")
	})

	t.Run("keeps others from changing views that are not theirs to change", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, _ := d.join(t, enums.RoleMember)
		personal := saveView(t, d.owner, "personal", oneCondition("status", "in", []string{"open"}))
		shared := saveView(t, d.owner, "shared", oneCondition("status", "in", []string{"open"}))

		bob.Delete(views + "/" + personal).AssertNotFound()
		bob.Delete(views + "/" + shared).AssertForbidden()
		d.owner.PatchJSON(views, testkit.JSON{"order": []string{personal}}).AssertNotFound()
		d.owner.PatchJSON(views, testkit.JSON{"order": []string{shared}}).AssertOK().
			AssertJSONPath("data.0.id", shared)
	})
}
