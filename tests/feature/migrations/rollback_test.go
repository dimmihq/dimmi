package migrations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestRollbackThenRunAgain(t *testing.T) {
	t.Parallel()
	d := open(t)

	run := func(d desk) {
		d.send(t, "teammate", testkit.JSON{"source_id": "adm-1", "email": d.TC.Owner.Email})
		d.send(t, "tag", testkit.JSON{"source_id": "tag-1", "name": "VIP"}).
			AssertJSONPath("data.results.0.status", "created")
		d.send(t, "company", testkit.JSON{"source_id": "co-1", "name": "Acme", "external_id": "acme-1"}).
			AssertJSONPath("data.results.0.status", "created")

		person := contact("c-1", "ada@example.com")
		person["external_id"] = "cus-1"
		person["companies"] = []testkit.JSON{{"ref": "co-1", "default": true}}
		d.send(t, "contact", person).AssertJSONPath("data.results.0.status", "created")

		d.send(t, "conversation", conversation(
			message("p1", 0, "Hello?"),
			event("p2", 1, testkit.JSON{"type": "tag", "added_tag_refs": []string{"tag-1"}}),
		)).AssertJSONPath("data.results.0.status", "created")

		d.send(t, "ticket_type", testkit.JSON{
			"source_id": "tt-1", "kind": "customer", "name": "Bug report",
			"states": []testkit.JSON{{"source_id": "new", "group": "submitted", "name": "Submitted"}},
		})
		d.send(t, "ticket", testkit.JSON{
			"source_id": "tk-1", "ticket_type_ref": "tt-1", "state_ref": "new", "title": "Checkout breaks",
			"contact_ref": "c-1", "created_at": at(10),
		}).AssertJSONPath("data.results.0.status", "created")
	}

	undo := func(d desk) {
		d.client.PostJSON(d.path("/rollbacks"), nil).AssertStatus(202)
		d.TC.Drain()
		d.client.GetJSON(d.path("")).AssertOK().AssertJSONPath("data.status", "rolled_back")
	}

	run(d)
	undo(d)
	assert.Equal(t, 0, d.rows(t, "tags", `name = 'VIP'`), "the migration's tag survived the undo")

	again := d
	again.id = d.client.PostJSON("/api/v1/migrations", testkit.JSON{
		"source": "acme-desk", "source_account": "workspace-1",
	}).AssertCreated().String("data.id")

	run(again)
	assert.Equal(t, 1, d.rows(t, "contacts", `external_id = 'cus-1' AND deleted_at IS NULL`))
	assert.Equal(t, 1, d.rows(t, "companies", `external_id = 'acme-1' AND deleted_at IS NULL`))
	assert.Equal(t, 1, d.rows(t, "tickets", `deleted_at IS NULL`))

	undo(again)
	assert.Equal(t, 0, d.rows(t, "contacts", `external_id = 'cus-1' AND deleted_at IS NULL`))
	assert.Equal(t, 0, d.rows(t, "companies", `external_id = 'acme-1' AND deleted_at IS NULL`))
}
