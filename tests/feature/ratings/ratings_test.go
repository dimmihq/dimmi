package ratings_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

func TestRatingSettings(t *testing.T) {
	t.Parallel()

	t.Run("start off and keep what the team changes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.GetJSON("/api/v1/conversation-rating-settings").AssertOK().
			AssertJSONPath("data.enabled", false).AssertJSONPath("data.messenger", true).
			AssertJSONPath("data.email", true).AssertJSONPath("data.delay_minutes", float64(0))

		d.Owner.PatchJSON("/api/v1/conversation-rating-settings", testkit.JSON{"enabled": true, "delay_minutes": 60}).
			AssertOK().AssertJSONPath("data.enabled", true).AssertJSONPath("data.delay_minutes", float64(60))
		d.Owner.PatchJSON("/api/v1/conversation-rating-settings", testkit.JSON{"email": false}).AssertOK().
			AssertJSONPath("data.enabled", true).AssertJSONPath("data.email", false)

		d.Owner.PatchJSON("/api/v1/conversation-rating-settings", testkit.JSON{"delay_minutes": 15}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})
}

func TestMessengerRatings(t *testing.T) {
	t.Parallel()

	t.Run("asks nothing while asking is off", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		v := visitor(t, d)
		id := startChat(v)
		closeChat(d, id)

		v.GetJSON("/widget/v1/conversations/"+id+"/rating").AssertOK().AssertJSONPath("data", nil)
		v.PatchJSON("/widget/v1/conversations/"+id+"/rating", testkit.JSON{"score": 5}).AssertNotFound()
	})

	t.Run("the visitor rates, comments, and the team reads it beside the conversation", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		settings(d, testkit.JSON{"enabled": true})
		rated := tests.Record[appevents.ConversationRated](d.TC)
		v := visitor(t, d)
		id := startChat(v)
		closeChat(d, id)

		path := "/widget/v1/conversations/" + id + "/rating"
		v.GetJSON(path).AssertOK().AssertJSONPath("data.score", nil).AssertJSONPath("data.editable", true).
			AssertJSONPath("data.faces.4.label", "Amazing")

		v.PatchJSON(path, testkit.JSON{"score": 6}).AssertStatus(fhttp.StatusUnprocessableEntity)
		v.PatchJSON(path, testkit.JSON{"score": 4}).AssertOK().AssertJSONPath("data.score", float64(4))
		v.PatchJSON(path, testkit.JSON{"comment": "  Quick and kind  "}).AssertOK().
			AssertJSONPath("data.comment", "Quick and kind")

		list := d.Owner.GetJSON("/api/v1/conversations/" + id + "/ratings").AssertOK()
		list.AssertJSONPath("data.0.score", float64(4)).AssertJSONPath("data.0.label", "Good").
			AssertJSONPath("data.0.comment", "Quick and kind").AssertJSONPath("data.0.channel", "widget")
		assert.NotEmpty(t, list.String("data.0.member.id"), "the teammate the conversation was with is rated")

		events := rated.All()
		require.Len(t, events, 2)
		assert.True(t, events[0].First)
		assert.False(t, events[1].First)
		assert.Equal(t, 4, events[1].Score)

		timeline := d.Owner.GetJSON("/api/v1/conversations/" + id + "/messages").AssertOK()
		assert.Contains(t, string(timeline.Body), `"conversation.rated"`)

		other := visitor(t, d)
		other.GetJSON(path).AssertNotFound()
		other.PatchJSON(path, testkit.JSON{"score": 1}).AssertNotFound()
	})

	t.Run("a delayed ask goes out later, and not at all once the customer wrote again", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		settings(d, testkit.JSON{"enabled": true, "delay_minutes": 10})
		v := visitor(t, d)

		later := startChat(v)
		closeChat(d, later)
		v.GetJSON("/widget/v1/conversations/"+later+"/rating").AssertOK().AssertJSONPath("data", nil)
		d.TC.DrainLater()
		v.GetJSON("/widget/v1/conversations/"+later+"/rating").AssertOK().AssertJSONPath("data.editable", true)

		reopened := startChat(visitor(t, d))
		closeChat(d, reopened)
		d.Owner.PatchJSON("/api/v1/conversations/"+reopened, testkit.JSON{"status": "open"}).AssertOK()
		d.TC.DrainLater()
		d.Owner.GetJSON("/api/v1/conversations/"+reopened+"/ratings").AssertOK().AssertJSONPath("data", []any{})
	})

	t.Run("a channel turned off is not asked", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		settings(d, testkit.JSON{"enabled": true, "messenger": false})
		v := visitor(t, d)
		id := startChat(v)
		closeChat(d, id)

		v.GetJSON("/widget/v1/conversations/"+id+"/rating").AssertOK().AssertJSONPath("data", nil)
	})
}

func TestEmailRatings(t *testing.T) {
	t.Parallel()

	t.Run("mails five faces whose signed page keeps the score and comment", func(t *testing.T) {
		t.Parallel()
		d := emailDesk(t)
		settings(d, testkit.JSON{"enabled": true})
		id := emailChat(t, d, "ann@example.com")
		d.Owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()

		token := ratingToken(t, d, 5)
		page := "/api/v1/conversation-ratings/" + token

		d.TC.Client().GetJSON(page).AssertOK().AssertJSONPath("data.workspace", d.TC.Workspace.Workspace.Name).
			AssertJSONPath("data.score", nil)
		d.TC.Client().PatchJSON(page, testkit.JSON{"score": 5}).AssertOK().AssertJSONPath("data.score", float64(5))
		d.TC.Client().PatchJSON(page, testkit.JSON{"comment": "Lovely"}).AssertOK().
			AssertJSONPath("data.comment", "Lovely")

		d.Owner.GetJSON("/api/v1/conversations/"+id+"/ratings").AssertOK().
			AssertJSONPath("data.0.channel", "email").AssertJSONPath("data.0.score", float64(5))
	})

	t.Run("refuses a link it did not sign and a rating past its week", func(t *testing.T) {
		t.Parallel()
		d := emailDesk(t)
		settings(d, testkit.JSON{"enabled": true})
		id := emailChat(t, d, "bo@example.com")
		d.Owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()
		token := ratingToken(t, d, 3)

		d.TC.Client().GetJSON("/api/v1/conversation-ratings/not-a-token").AssertNotFound()
		forged := token[:len(token)-2] + "xx"
		d.TC.Client().PatchJSON("/api/v1/conversation-ratings/"+forged, testkit.JSON{"score": 1}).AssertNotFound()

		_, err := d.TC.Operator().Exec(t.Context(),
			`UPDATE conversation_ratings SET asked_at = asked_at - interval '8 days'`)
		require.NoError(t, err)

		page := "/api/v1/conversation-ratings/" + token
		d.TC.Client().GetJSON(page).AssertOK().AssertJSONPath("data.editable", false)
		d.TC.Client().PatchJSON(page, testkit.JSON{"score": 1}).AssertStatus(fhttp.StatusGone)
	})

	t.Run("sends nothing without an address to send from", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		settings(d, testkit.JSON{"enabled": true})
		id := emailChat(t, d, "di@example.com")

		// A workspace starts with a forwarding address it may send from;
		// with every address removed there is none left.
		_, err := d.TC.Operator().Exec(t.Context(),
			`UPDATE email_addresses SET deleted_at = now() WHERE workspace_id = $1`, d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)

		d.Owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()

		assert.Empty(t, ratingMail(d))
	})

	t.Run("sends nothing with email turned off", func(t *testing.T) {
		t.Parallel()
		d := emailDesk(t)
		settings(d, testkit.JSON{"enabled": true, "email": false})
		id := emailChat(t, d, "cy@example.com")
		d.Owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()

		assert.Empty(t, ratingMail(d))
	})
}

func TestWorkflowRatings(t *testing.T) {
	t.Parallel()

	t.Run("a close step asks for a rating even with asking off, and a rating starts workflows", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		publish(t, d, "Close and ask", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				{"id": "s_close01", "kind": "close", "settings": testkit.JSON{"ask_rating": true}},
			},
		})
		publish(t, d, "On rating", testkit.JSON{
			"schema": 1, "trigger": "conversation.rated",
			"steps": []testkit.JSON{{"id": "s_note001", "kind": "note", "settings": testkit.JSON{"text": "Rated!"}}},
		})

		v := visitor(t, d)
		id := startChat(v)
		d.TC.Drain()

		path := "/widget/v1/conversations/" + id + "/rating"
		v.GetJSON(path).AssertOK().AssertJSONPath("data.editable", true)
		v.PatchJSON(path, testkit.JSON{"score": 2}).AssertOK()
		d.TC.Drain()

		notes := d.Owner.GetJSON("/api/v1/conversations/" + id + "/messages").AssertOK()
		assert.Contains(t, string(notes.Body), "Rated!")
	})
}

/*
publish creates a workflow from the definition and publishes it.
*/
func publish(t *testing.T, d tests.Desk, name string, spec testkit.JSON) {
	t.Helper()

	id := d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": name, "spec": spec}).
		AssertCreated().String("data.id")
	d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).AssertOK()
}
