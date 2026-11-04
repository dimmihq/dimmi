package conversations_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
)

func TestSnoozeOptions(t *testing.T) {
	t.Parallel()

	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	t.Run("resolves each kind in the reader's zone, across a clock change", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 31, 22, 30, 0, 0, time.UTC)

		for name, tc := range map[string]struct {
			option conversations.SnoozeOption
			want   time.Time
		}{
			"minutes from now": {conversations.SnoozeOption{Kind: "in", Minutes: 90}, now.Add(90 * time.Minute)},
			"a day ahead at a time": {
				conversations.SnoozeOption{Kind: "day_at", DaysAhead: 1, At: "09:00"},
				time.Date(2026, 11, 1, 9, 0, 0, 0, ny),
			},
			"the next weekday, never today": {
				conversations.SnoozeOption{Kind: "weekday_at", Weekday: 6, At: "08:30"},
				time.Date(2026, 11, 7, 8, 30, 0, 0, ny),
			},
			"a preset": {
				conversations.SnoozeOption{Kind: "preset", Preset: "tomorrow"},
				time.Date(2026, 11, 1, 9, 0, 0, 0, ny),
			},
		} {
			assert.True(t, tc.want.Equal(conversations.ResolveSnoozeOption(tc.option, now, ny)), name)
		}
	})

	t.Run("offers the built-in presets until a list is set", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "ann", "Later", "Hi")

		res := d.owner.GetJSON("/api/v1/snooze-options?conversation=" + id).AssertOK()
		assert.Equal(t, []string{"later_today", "tomorrow", "next_week", "one_week"}, sliceOf(res, "data.#.key"))
		assert.NotEmpty(t, res.String("data.0.until"))
	})

	t.Run("prefers the team's list, then the workspace's, and snoozes by key", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		team := d.owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().String("data.id")
		id := d.customer(t, "ann", "Later", "Hi")
		workspaceList := []testkit.JSON{{"key": "hour", "label": "In an hour", "kind": "in", "minutes": 60}}
		teamList := []testkit.JSON{{"key": "monday", "label": "Monday", "kind": "weekday_at", "weekday": 1, "at": "09:00"}}

		d.owner.PatchJSON("/api/v1/snooze-lists/workspace", testkit.JSON{"options": workspaceList}).AssertOK()
		assert.Equal(t, []string{"hour"}, sliceOf(d.owner.GetJSON("/api/v1/snooze-options?conversation="+id), "data.#.key"))

		d.owner.PatchJSON("/api/v1/snooze-lists/"+team, testkit.JSON{"options": teamList}).AssertOK().
			AssertJSONPath("data.teams."+team+".0.key", "monday")
		d.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"team_id": team}).AssertOK()
		assert.Equal(t, []string{"monday"}, sliceOf(d.owner.GetJSON("/api/v1/snooze-options?conversation="+id), "data.#.key"))

		d.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "snoozed", "snooze_option": "hour"}).
			AssertStatus(422).AssertJSONPath("code", "snooze_option_unknown")
		d.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "snoozed", "snooze_option": "monday"}).
			AssertOK().AssertJSONPath("data.status", "snoozed")

		d.owner.PatchJSON("/api/v1/snooze-lists/"+team, testkit.JSON{"options": nil}).AssertOK()
		assert.Equal(t, []string{"hour"}, sliceOf(d.owner.GetJSON("/api/v1/snooze-options?conversation="+id), "data.#.key"))
	})

	t.Run("refuses an empty list, eleven options and a member", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, _ := d.join(t, enums.RoleMember)
		eleven := make([]testkit.JSON, 11)
		for i := range eleven {
			eleven[i] = testkit.JSON{"key": string(rune('a' + i)), "label": "x", "kind": "in", "minutes": 10}
		}

		d.owner.PatchJSON("/api/v1/snooze-lists/workspace", testkit.JSON{"options": []testkit.JSON{}}).AssertStatus(422)
		d.owner.PatchJSON("/api/v1/snooze-lists/workspace", testkit.JSON{"options": eleven}).AssertStatus(422)
		d.owner.PatchJSON("/api/v1/snooze-lists/workspace", testkit.JSON{"options": []testkit.JSON{
			{"key": "x", "label": "x", "kind": "in", "minutes": 1},
		}}).AssertStatus(422).AssertJSONPath("code", "invalid_snooze_options")
		bob.PatchJSON("/api/v1/snooze-lists/workspace", testkit.JSON{"options": []testkit.JSON{
			{"key": "x", "label": "x", "kind": "in", "minutes": 10},
		}}).AssertForbidden()
	})

	t.Run("takes exactly one way to say when a snooze ends", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Later", "Hi")

		d.owner.PatchJSON(path, testkit.JSON{
			"status": "snoozed", "snooze_option": "tomorrow", "snooze_preset": "tomorrow",
		}).AssertStatus(422)
		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_option": "tomorrow"}).AssertOK()
	})
}
