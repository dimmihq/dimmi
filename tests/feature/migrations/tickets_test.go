package migrations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
ticketSetup sends a teammate, a customer and two ticket types, one for
customers and one for back-office work.
*/
func ticketSetup(t *testing.T, d desk) {
	t.Helper()

	d.send(t, "teammate", testkit.JSON{"source_id": "adm-1", "email": d.TC.Owner.Email})
	d.send(t, "contact", contact("c-1", "ada@example.com"))

	for id, kind := range map[string]string{"tt-c": "customer", "tt-b": "back_office"} {
		d.send(t, "ticket_type", testkit.JSON{
			"source_id": id, "kind": kind, "name": "Type " + id,
			"states": []testkit.JSON{
				{"source_id": "new", "group": "submitted", "name": "Submitted"},
				{"source_id": "done", "group": "resolved", "name": "Done"},
			},
		}).AssertJSONPath("data.results.0.status", "created")
	}
}

/*
written finds the messages and notes in the tickets' conversations,
leaving out the timeline's own events.
*/
const written = `type <> 'event' AND conversation_id IN (SELECT conversation_id FROM tickets)`

/*
ticket is the ticket tk-1 of the type, by the customer c-1, with its
entries.
*/
func ticket(typeRef string, entries ...testkit.JSON) testkit.JSON {
	return testkit.JSON{
		"source_id": "tk-1", "ticket_type_ref": typeRef, "state_ref": "new", "title": "Export fails",
		"contact_ref": "c-1", "created_at": at(10), "entries": entries,
	}
}

func note(id string, minute int, body string) testkit.JSON {
	return testkit.JSON{
		"source_id": id, "type": "note", "created_at": at(minute), "body": body,
		"author": testkit.JSON{"type": "teammate", "ref": "adm-1"},
	}
}

func TestTicketEntries(t *testing.T) {
	t.Parallel()

	t.Run("a customer ticket keeps what was written on it in its own conversation", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		ticketSetup(t, d)

		d.send(t, "ticket", ticket("tt-c",
			message("e1", 10, "The export fails."),
			reply("e2", 11, "Looking into it.", "adm-1"),
			note("e3", 12, "Repro on staging."),
			event("e4", 13, testkit.JSON{"type": "status", "status": "closed"}),
		)).AssertJSONPath("data.results.0.status", "created")

		where := written
		assert.Equal(t, 1, d.rows(t, "tickets", `conversation_id IS NOT NULL`))
		assert.Equal(t, 3, d.rows(t, "messages", where), "the message, the reply and the note")
		assert.Equal(t, 1, d.rows(t, "messages", where+` AND type = 'note'`))
		assert.Equal(t, 1, d.rows(t, "messages", where+` AND author_type = 'member' AND type = 'message'`))
	})

	t.Run("a sync writes only the entries new since", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		ticketSetup(t, d)

		first := ticket("tt-c", message("e1", 10, "The export fails."))
		d.send(t, "ticket", first)

		again := ticket("tt-c", message("e1", 10, "The export fails."), reply("e2", 30, "Fixed now.", "adm-1"))
		again["updated_at"] = at(30)
		d.send(t, "ticket", again).AssertJSONPath("data.results.0.status", "updated")
		d.send(t, "ticket", again).AssertJSONPath("data.results.0.status", "unchanged")

		assert.Equal(t, 2, d.rows(t, "messages", written))
	})

	t.Run("a ticket linked to a conversation adds its entries there", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		ticketSetup(t, d)

		d.send(t, "conversation", conversation(message("p1", 0, "Hello?")))

		linked := ticket("tt-c", note("e1", 10, "Converted to a ticket."))
		linked["conversation_ref"] = "cv-1"
		d.send(t, "ticket", linked).AssertJSONPath("data.results.0.status", "created")

		assert.Equal(t, 1, d.rows(t, "conversations", "true"))
		assert.Equal(t, 2, d.rows(t, "messages", written))
	})

	t.Run("a back-office ticket keeps its entries as notes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		ticketSetup(t, d)

		d.send(t, "ticket", ticket("tt-b",
			note("e1", 10, "Needs a refund."), reply("e2", 11, "Refunded.", "adm-1"),
		)).AssertJSONPath("data.results.0.status", "created")

		assert.Equal(t, 0, d.rows(t, "conversations", "true"))
		assert.Equal(t, 2, d.rows(t, "ticket_events", `kind = 'note'`))
	})

	t.Run("an undo removes the conversation a ticket opened", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		ticketSetup(t, d)

		d.send(t, "ticket", ticket("tt-c", message("e1", 10, "The export fails.")))
		assert.Equal(t, 1, d.rows(t, "conversations", `deleted_at IS NULL`))

		d.client.PostJSON(d.path("/rollbacks"), nil).AssertStatus(202)
		d.TC.Drain()

		assert.Equal(t, 0, d.rows(t, "conversations", `deleted_at IS NULL`))
		assert.Equal(t, 0, d.rows(t, "tickets", `deleted_at IS NULL`))
	})
}
