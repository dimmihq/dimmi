package broadcasts_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
trackerType returns the public id of the workspace's tracker type.
*/
func trackerType(t *testing.T, d tests.Desk) string {
	t.Helper()

	var body struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	d.Owner.GetJSON("/api/v1/ticket-types").AssertOK().Decode(&body)

	for _, ty := range body.Data {
		if ty.Name == "Tracker" {
			return ty.ID
		}
	}

	t.Fatal("no tracker type")

	return ""
}

func TestConversationBroadcasts(t *testing.T) {
	t.Parallel()

	t.Run("answers a tracker's customers in their own conversations and holds who is waiting", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		ann := d.Customer(t, "ann", "Export crashes").String("data.id")
		ben := d.Customer(t, "ben", "Export crashes").String("data.id")
		cat := d.Customer(t, "cat", "Export crashes").String("data.id")
		dee := d.Customer(t, "dee", "Unrelated").String("data.id")

		for _, c := range []string{ann, ben} {
			d.Owner.PostJSON("/api/v1/conversations/"+c+"/replies", testkit.JSON{"body": "Looking into it."}).
				AssertCreated()
		}

		tracker := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": trackerType(t, d), "title": "CSV export crashes",
			"link_to": []testkit.JSON{
				{"kind": "tracker", "conversation_id": ann},
				{"kind": "tracker", "conversation_id": ben},
				{"kind": "tracker", "conversation_id": cat},
			},
		}).AssertCreated().String("data.id")

		id := d.Owner.PostJSON("/api/v1/broadcasts", testkit.JSON{
			"name": "Export fix", "channel": "conversation", "ticket_id": tracker,
			"holds": []string{"customer_waiting"},
		}).AssertCreated().
			AssertJSONPath("data.ticket_id", tracker).
			AssertJSONPath("data.holds.0", "customer_waiting").
			String("data.id")

		d.Owner.PatchJSON("/api/v1/broadcasts/"+id, testkit.JSON{
			"body":      "Hi {{ contact.first_name }}, the export works again.",
			"sender_id": models.PublicID(models.PrefixMember, d.TC.Workspace.Owner.ID),
		}).AssertOK()

		d.Owner.GetJSON("/api/v1/broadcasts/"+id+"/estimate").AssertOK().
			AssertJSONPath("data.reach", float64(2)).
			AssertJSONPath("data.skipped.customer_waiting", float64(1))

		d.Owner.PostJSON("/api/v1/broadcasts/"+id+"/send", nil).AssertOK()
		for range 6 {
			d.TC.DrainLater()
		}

		statuses := conversationStatuses(t, d, id)
		assert.Equal(t, map[string]string{ann: "sent/", ben: "sent/", cat: "skipped/customer_waiting"}, statuses)
		assert.NotContains(t, statuses, dee)

		said := d.Owner.GetJSON("/api/v1/conversations/" + ann + "/messages").AssertOK()
		said.AssertJSONPath("data.#(body==\"Hi ann, the export works again.\").author.type", "member")
		said.AssertJSONPath("data.#(body==\"Hi ann, the export works again.\").broadcast.id", id)
		said.AssertJSONPath("data.#(body==\"Hi ann, the export works again.\").broadcast.name", "Export fix")

		annRecipient := recipientIn(d, id, ann)
		require.NotNil(t, annRecipient.ExternalRef)
		message, err := ids.Parse(*annRecipient.ExternalRef)
		require.NoError(t, err)

		require.NoError(t, d.TC.App.Actions.Broadcasts.MessageDelivery(t.Context(), broadcasts.DeliveryReport{
			WorkspaceID: d.TC.Workspace.Workspace.ID, MessageID: message, Status: "failed",
			Reason: "no_sending_address", Final: true,
		}))
		for range 3 {
			d.TC.DrainLater()
		}

		failed := recipientIn(d, id, ann)
		assert.Equal(t, "failed", failed.Status)
		assert.Equal(t, "no_sending_address", deref(failed.Reason))
		d.Owner.GetJSON("/api/v1/broadcasts/"+id+"/report").AssertOK().AssertJSONPath("data.counts.failed", float64(1))
	})

	t.Run("starts from a tracker only on the conversation channel", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		tracker := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": trackerType(t, d), "title": "Sign-in outage",
		}).AssertCreated().String("data.id")

		d.Owner.PostJSON("/api/v1/broadcasts", testkit.JSON{
			"name": "Outage", "channel": "email", "ticket_id": tracker,
		}).AssertStatus(422).AssertJSONPath("code", "broadcast_tracker_invalid")

		d.Owner.PostJSON("/api/v1/broadcasts", testkit.JSON{
			"name": "Outage", "channel": "chat", "holds": []string{"customer_waiting"},
		}).AssertStatus(422).AssertJSONPath("code", "broadcast_holds_invalid")
	})
}

/*
conversationStatuses returns each recipient's status and reason, keyed
by the public id of the conversation they were answered in.
*/
func conversationStatuses(t *testing.T, d tests.Desk, id string) map[string]string {
	t.Helper()

	statuses := map[string]string{}
	for _, r := range (desk{tc: d.TC, owner: d.Owner}).recipients(id) {
		require.NotNil(t, r.ConversationID)
		statuses[models.PublicID(models.PrefixConversation, *r.ConversationID)] = r.Status + "/" + deref(r.Reason)
	}

	return statuses
}

/*
recipientIn returns the broadcast's recipient answered in the
conversation, or an empty one when there is none.
*/
func recipientIn(d tests.Desk, id, conv string) models.BroadcastRecipient {
	for _, r := range (desk{tc: d.TC, owner: d.Owner}).recipients(id) {
		if r.ConversationID != nil && models.PublicID(models.PrefixConversation, *r.ConversationID) == conv {
			return r
		}
	}

	return models.BroadcastRecipient{}
}

/*
deref is the text, or "" for none.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
