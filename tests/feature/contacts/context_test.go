package contacts_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestConversationContext(t *testing.T) {
	t.Parallel()

	t.Run("tells who the customer is, with company, identities and notes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conv := d.Customer(t, "bob", "Hi")
		contactID := conv.String("data.contact.id")
		company := d.Owner.PostJSON("/api/v1/companies", testkit.JSON{
			"name": "Acme", "domain": "acme.dev", "custom_attributes": testkit.JSON{"seats": 12},
		}).AssertCreated().String("data.id")
		d.Owner.PatchJSON("/api/v1/contacts/"+contactID, testkit.JSON{
			"company_id": company, "notes": "Prefers email", "add_tags": []string{"vip"},
		}).AssertOK()

		d.Owner.GetJSON("/api/v1/conversations/"+conv.String("data.id")+"/context").AssertOK().
			AssertJSONPath("data.object", "conversation_context").
			AssertJSONPath("data.contact.id", contactID).
			AssertJSONPath("data.contact.notes", "Prefers email").
			AssertJSONPath("data.contact.tags.0.name", "vip").
			AssertJSONPath("data.company.name", "Acme").
			AssertJSONPath("data.company.custom_attributes.seats", float64(12)).
			AssertJSONPath("data.identities.0.type", "email").
			AssertJSONPath("data.identities.0.value", "bob@example.com").
			AssertJSONPath("data.source", map[string]any{}).
			AssertJSONPath("data.past_conversations.data.#", 0).
			AssertJSONPath("data.past_conversations.total_estimate", 0).
			AssertJSONPath("data.duplicates_count", 0)
	})

	t.Run("keeps the source an object when a channel brings none", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace.ID

		_, err := d.TC.App.Actions.ProcessInboundEmail.Receiver.ReceiveInboundMessage(
			tenancy.With(t.Context(), ws.String()),
			contracts.InboundMessage{
				ConnectionID: d.Inbox.API.ID, DedupeKey: "context-source",
				Sender:  contracts.InboundSender{Name: "Ann", Email: "ann@example.com"},
				Subject: "From email", Body: "Hi",
			},
		)
		require.NoError(t, err)

		_, err = d.TC.Operator().Exec(
			tenancy.With(t.Context(), ws.String()), "UPDATE conversations SET source = 'null'::jsonb",
		)
		require.NoError(t, err)

		d.Owner.GetJSON("/api/v1/conversations/1/context").AssertOK().
			AssertJSONPath("data.source", map[string]any{})
	})

	t.Run("shows the three newest other conversations and counts the rest", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		current := d.Customer(t, "bob", "Current").String("data.id")

		for i := range 5 {
			d.Customer(t, "bob", fmt.Sprintf("Older %d", i))
		}

		spam := d.Customer(t, "bob", "Spam").String("data.id")
		d.Owner.PatchJSON("/api/v1/conversations/"+spam, testkit.JSON{"spam": true}).AssertOK()
		d.Customer(t, "ann", "Someone else")

		res := d.Owner.GetJSON("/api/v1/conversations/"+current+"/context").AssertOK().
			AssertJSONPath("data.past_conversations.data.#", 3).
			AssertJSONPath("data.past_conversations.data.0.subject", "Older 4").
			AssertJSONPath("data.past_conversations.total_estimate", 5)
		assert.NotContains(t, string(res.Body), current)
		assert.NotContains(t, string(res.Body), spam)
	})

	t.Run("counts contacts that look like the same person", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conv := d.Customer(t, "bob", "Hi")
		d.Owner.PatchJSON("/api/v1/contacts/"+conv.String("data.contact.id"), testkit.JSON{"phone": "+15550100"}).
			AssertOK()
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Robert", "phone": "+15550100"}).AssertCreated()

		d.Owner.GetJSON("/api/v1/conversations/"+conv.String("data.id")+"/context").AssertOK().
			AssertJSONPath("data.duplicates_count", 1)
	})

	t.Run("finds the conversation by number and refuses what is not there", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		other := tests.NewDesk(t)
		d.Customer(t, "bob", "Hi")
		theirs := other.Customer(t, "eve", "Theirs").String("data.id")

		d.Owner.GetJSON("/api/v1/conversations/1/context").AssertOK().AssertJSONPath("data.contact.name", "bob")
		d.Owner.GetJSON("/api/v1/conversations/" + theirs + "/context").AssertNotFound()
		d.Owner.GetJSON("/api/v1/conversations/bogus/context").AssertNotFound()
	})

	t.Run("needs both conversation and contact reading", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conv := d.Customer(t, "bob", "Hi").String("data.id")
		path := "/api/v1/conversations/" + conv + "/context"
		member, _ := d.Join(t, enums.RoleMember)

		member.GetJSON(path).AssertOK()
		d.TC.ActingAsToken(d.TC.Workspace.Owner, "conversations:read").GetJSON(path).AssertForbidden()
		d.TC.ActingAsToken(d.TC.Workspace.Owner, "conversations:read", "contacts:read").GetJSON(path).AssertOK()
	})
}
