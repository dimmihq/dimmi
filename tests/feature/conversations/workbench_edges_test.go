package conversations_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	conversationviews "github.com/dimmihq/dimmi/app/actions/conversation_views"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
corrupt writes the SQL straight to the database, as a damaged row.
*/
func corrupt(t *testing.T, d desk, sql string) {
	t.Helper()

	_, err := d.tc.Operator().Exec(t.Context(), sql)
	require.NoError(t, err)
}

func TestWorkbenchEdges(t *testing.T) {
	t.Parallel()

	const prefs = "/api/v1/inbox-preferences"

	t.Run("fails on damaged preferences and counts without the pinned views", func(t *testing.T) {
		t.Parallel()

		for _, column := range []string{"views", "pinned_attributes", "sidebar"} {
			d := open(t)
			d.owner.PatchJSON(prefs, testkit.JSON{"pinned_attributes": []string{"plan"}}).AssertOK()
			corrupt(t, d, "UPDATE member_inbox_preferences SET "+column+" = '\"broken\"'")

			d.owner.GetJSON("/api/v1/conversation-counts").AssertOK()
			if column == "sidebar" {
				continue
			}

			d.owner.GetJSON(prefs).AssertStatus(500)
			d.owner.PatchJSON(prefs, testkit.JSON{"pinned_attributes": []string{"x"}}).AssertStatus(500)
		}
	})

	t.Run("refuses a damaged view when listing it, and lists one closed by default", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		closed := d.customer(t, "ann", "Done", "Hi")
		d.owner.PatchJSON("/api/v1/conversations/"+closed, testkit.JSON{"status": "closed"}).AssertOK()
		id := d.owner.PostJSON("/api/v1/conversation-views", testkit.JSON{
			"name": "Closed", "scope": "personal", "status_filter": "closed",
			"definition": oneCondition("message_count", "gt", 0),
		}).AssertCreated().String("data.id")

		d.owner.GetJSON("/api/v1/conversations?filter[view]="+id).AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.id", closed)

		corrupt(t, d, `UPDATE conversation_views SET definition = '{"op":"xor","groups":[]}'`)
		d.owner.GetJSON("/api/v1/conversations?filter[view]=" + id).AssertStatus(422)
		corrupt(t, d, `UPDATE conversation_views SET definition = '"broken"'`)
		d.owner.GetJSON("/api/v1/conversations?filter[view]=" + id).AssertStatus(422)
	})

	t.Run("searches inside a view that searches too", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		match := d.customer(t, "ann", "Refund for the card", "Refund my card please")
		d.customer(t, "bob", "Refund for the plan", "Refund my plan please")
		id := saveView(t, d.owner, "personal", oneCondition("text", "matches", "refund"))

		d.owner.GetJSON("/api/v1/conversations?filter[view]="+id+"&q=card").AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.id", match)
	})

	t.Run("answers snooze choices and lists for what is not there or is damaged", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.owner.GetJSON("/api/v1/snooze-options?conversation=conv_01jbd0k3a8x9t2m4q5r6s7v8w9").AssertNotFound()
		d.owner.PatchJSON("/api/v1/snooze-lists/team_01jbd0k3a8x9t2m4q5r6s7v8w9", testkit.JSON{
			"options": []testkit.JSON{{"key": "a", "label": "A", "kind": "in", "minutes": 30}},
		}).AssertNotFound()

		d.owner.PatchJSON("/api/v1/snooze-lists/workspace", testkit.JSON{
			"options": []testkit.JSON{{"key": "a", "label": "A", "kind": "in", "minutes": 30}},
		}).AssertOK()
		corrupt(t, d, `UPDATE snooze_lists SET options = '"broken"'`)
		d.owner.GetJSON("/api/v1/snooze-lists").AssertStatus(500)
	})

	t.Run("reads definitions and errors and falls back to UTC for unknown zones", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		_, err := conversations.ReadDefinition(json.RawMessage(`{`))
		require.ErrorIs(t, err, exceptions.ErrInvalidDefinition)

		plain := errors.New("database is down")
		assert.Equal(t, plain, conversations.CompileError(plain))

		assert.Equal(t, time.UTC, conversations.Location(new("Nowhere/Never"), models.Workspace{Timezone: "Also/Not"}))

		m := authz.NewMembership(d.tc.Workspace.Workspace, d.tc.Workspace.Owner, []string{"conversations:write"})
		for _, fields := range []conversationviews.Fields{
			{Name: new("   "), Definition: json.RawMessage(`{"op":"and"}`)},
			{Name: new("View"), Definition: json.RawMessage(`{`)},
		} {
			_, err := d.tc.App.Actions.Inbox.CreateView.Handle(t.Context(), conversationviews.CreateInput{
				Membership: m, Scope: conversationviews.ScopePersonal, Fields: fields,
			})
			assert.ErrorIs(t, err, exceptions.ErrInvalidDefinition)
		}
	})
}
