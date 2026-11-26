package migrations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestMigrations(t *testing.T) {
	t.Parallel()

	t.Run("starting again for the same account resumes the migration", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		again := d.client.PostJSON("/api/v1/migrations", testkit.JSON{
			"source": "acme-desk", "source_account": "workspace-1",
		}).AssertOK()
		assert.Equal(t, d.id, again.String("data.id"))

		other := d.client.PostJSON("/api/v1/migrations", testkit.JSON{
			"source": "acme-desk", "source_account": "workspace-2",
		}).AssertCreated()
		assert.NotEqual(t, d.id, other.String("data.id"))

		d.client.GetJSON("/api/v1/migrations").AssertOK().AssertJSONPath("data.#", 2)
		d.client.GetJSON(d.path("")).AssertOK().AssertJSONPath("data.status", "active").
			AssertJSONPath("data.format_version", 1)
	})

	t.Run("a source must be a short lowercase name", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.client.PostJSON("/api/v1/migrations", testkit.JSON{"source": "Acme Desk!", "source_account": "x"}).
			AssertStatus(422)
	})

	t.Run("every kind of record is applied", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		owner := d.TC.Owner.Email

		res := d.send(t, "teammate",
			testkit.JSON{"source_id": "adm-1", "email": owner, "name": "Owner"},
			testkit.JSON{"source_id": "adm-2", "email": "gone@example.com", "name": "Grace"})
		res.AssertJSONPath("data.results.0.status", "created").AssertJSONPath("data.results.1.status", "unsupported")

		d.send(t, "team", testkit.JSON{"source_id": "team-1", "name": "Billing", "member_refs": []string{"adm-1"}}).
			AssertJSONPath("data.results.0.status", "created")
		d.send(t, "tag", testkit.JSON{"source_id": "tag-1", "name": "vip"}).
			AssertJSONPath("data.results.0.status", "created")
		d.send(t, "attribute_definition",
			testkit.JSON{"source_id": "plan", "object": "contact", "key": "plan", "label": "Plan", "type": "string"},
			testkit.JSON{"source_id": "issue", "object": "conversation", "key": "issue", "label": "Issue", "type": "string"},
		).AssertJSONPath("data.results.0.status", "created").AssertJSONPath("data.results.1.status", "unsupported")
		d.send(t, "company", testkit.JSON{
			"source_id": "co-1", "external_id": "acme", "name": "Acme", "tag_refs": []string{"tag-1"},
			"created_at": at(-600),
		}).AssertJSONPath("data.results.0.status", "created")

		c := contact("c-1", "ada@example.com")
		c["companies"] = []testkit.JSON{{"ref": "co-1", "default": true}}
		c["tag_refs"] = []string{"tag-1"}
		c["first_seen_at"], c["last_seen_at"], c["locale"] = at(-500), at(-1), "fr"
		c["custom_attributes"] = testkit.JSON{"plan": "pro"}
		cid := d.send(t, "contact", c).AssertJSONPath("data.results.0.status", "created").String("data.results.0.id")

		contactRes := d.Owner.GetJSON("/api/v1/contacts/" + cid).AssertOK()
		contactRes.AssertJSONPath("data.locale", "fr").AssertJSONPath("data.custom_attributes.plan", "pro")

		d.send(t, "contact_note", testkit.JSON{
			"source_id": "n-1", "contact_ref": "c-1", "body": "Prefers email.", "created_at": at(-100),
			"author": testkit.JSON{"type": "teammate", "ref": "adm-1"},
		}).AssertJSONPath("data.results.0.status", "created")

		d.send(t, "ticket_type", testkit.JSON{
			"source_id": "tt-1", "kind": "customer", "name": "Bug report",
			"states": []testkit.JSON{
				{"source_id": "new", "group": "submitted", "name": "Submitted"},
				{"source_id": "done", "group": "resolved", "name": "Fixed"},
			},
			"fields": []testkit.JSON{{"source_id": "f1", "key": "browser", "label": "Browser", "type": "string"}},
		}).AssertJSONPath("data.results.0.status", "created")

		d.send(t, "help_center", testkit.JSON{"source_id": "hc-1", "name": "Help", "default_locale": "en"}).
			AssertJSONPath("data.results.0.status", "created")
		d.send(t, "collection", testkit.JSON{"source_id": "col-1", "help_center_ref": "hc-1", "name": "Billing"}).
			AssertJSONPath("data.results.0.status", "created")

		picture := d.upload(t, "chart.png", "img-1", png())
		d.send(t, "article", testkit.JSON{
			"source_id": "a-1", "help_center_ref": "hc-1", "collection_ref": "col-1", "title": "Refunds",
			"body": "How refunds work.\n\n![chart](dimmi-file:" + picture + ")", "state": "published",
			"author": testkit.JSON{"type": "teammate", "ref": "adm-1"}, "created_at": at(-300),
			"redirects": []string{"https://help.acme.test/en/articles/1-refunds"},
		}).AssertJSONPath("data.results.0.status", "created")
		d.send(t, "snippet", testkit.JSON{"source_id": "s-1", "title": "Refund window", "body": "30 days."}).
			AssertJSONPath("data.results.0.status", "created")
		d.send(t, "saved_reply", testkit.JSON{
			"source_id": "r-1", "title": "Thanks", "body": "Thanks {{ contact.name }}!",
			"actions": []testkit.JSON{{"type": "tag", "tag_ref": "tag-1"}, {"type": "status", "status": "closed"}},
		}).AssertJSONPath("data.results.0.status", "created")
		d.send(t, "schedule", testkit.JSON{
			"source_id": "sch-1", "name": "Office hours", "timezone": "Europe/Paris",
			"weekly":   []testkit.JSON{{"day": "mon", "start": "09:00", "end": "17:00"}},
			"holidays": []testkit.JSON{{"date": "2024-12-25", "name": "Christmas", "yearly": true}},
		}).AssertJSONPath("data.results.0.status", "created")

		file := d.upload(t, "invoice.pdf", "att-1", []byte("%PDF-1.4 invoice"))
		conv := conversation("cv-1", "c-1",
			message("p1", 0, "Where is my refund?", "c-1"),
			event("p2", 1, "adm-1", testkit.JSON{"type": "assignment", "assignee_ref": "adm-1"}),
			event("p3", 2, "adm-1", testkit.JSON{"type": "priority", "priority": "high"}),
			event("p4", 3, "adm-1", testkit.JSON{"type": "tag", "added_tag_refs": []string{"tag-1"}}),
			testkit.JSON{
				"source_id": "p5", "type": "reply", "created_at": at(4), "body": "Here is the invoice.",
				"author": testkit.JSON{"type": "teammate", "ref": "adm-1"}, "attachments": []string{file},
				"email": testkit.JSON{"message_id": "<p5@acme.test>", "references": []string{"<p1@acme.test>"}},
			},
			testkit.JSON{
				"source_id": "p6", "type": "reply", "created_at": at(5), "body": "Anything else?",
				"author": testkit.JSON{"type": "ai_agent", "label": "Fin (imported)"},
			},
			event("p7", 6, "adm-1", testkit.JSON{"type": "custom", "raw_type": "sla_applied", "text": "SLA applied"}),
			event("p8", 7, "adm-1", testkit.JSON{"type": "status", "status": "closed"}),
		)
		conv["rating"] = testkit.JSON{"score": 5, "remark": "Great", "created_at": at(8)}
		convID := d.send(t, "conversation", conv).AssertJSONPath("data.results.0.status", "created").
			String("data.results.0.id")

		shown := d.Owner.GetJSON("/api/v1/conversations/" + convID).AssertOK()
		shown.AssertJSONPath("data.status", "closed").AssertJSONPath("data.priority", "high").
			AssertJSONPath("data.message_count", 3)
		assert.Equal(t, 1, d.count(t, `SELECT count(*) FROM attachments WHERE workspace_id = $1 AND message_id IS NOT NULL`))
		assert.Equal(t, 1, d.count(t, `SELECT count(*) FROM conversation_ratings WHERE workspace_id = $1 AND score = 5`))
		assert.Equal(t, 2, d.count(t, `SELECT count(*) FROM email_message_refs WHERE workspace_id = $1`))
		assert.Equal(t, 1, d.count(t, `SELECT count(*) FROM messages WHERE workspace_id = $1 AND author_type = 'agent'`))
		assert.Equal(t, 1, d.count(t, `SELECT count(*) FROM conversation_tag WHERE workspace_id = $1`))

		d.send(t, "ticket", testkit.JSON{
			"source_id": "tk-1", "ticket_type_ref": "tt-1", "state_ref": "done", "title": "Checkout breaks",
			"contact_ref": "c-1", "created_at": at(10), "updated_at": at(20),
			"state_history": []testkit.JSON{{"state_ref": "new", "at": at(10)}, {"state_ref": "done", "at": at(20)}},
		}).AssertJSONPath("data.results.0.status", "created")

		progress := d.client.GetJSON(d.path("")).AssertOK()
		progress.AssertJSONPath(`data.counts.#(kind=="conversation").applied`, 1).
			AssertJSONPath(`data.counts.#(kind=="teammate").unsupported`, 1)
	})

	t.Run("sending the same records again changes nothing", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.send(t, "contact", contact("c-1", "ada@example.com"))
		conv := conversation("cv-1", "c-1", message("p1", 0, "Hello?", "c-1"))
		d.send(t, "conversation", conv)

		d.send(t, "contact", contact("c-1", "ada@example.com")).AssertJSONPath("data.results.0.status", "unchanged")
		d.send(t, "conversation", conv).AssertJSONPath("data.results.0.status", "unchanged")

		assert.Equal(t, 1, d.count(t, `SELECT count(*) FROM conversations WHERE workspace_id = $1`))
		assert.Equal(t, 1, d.count(t, `SELECT count(*) FROM contacts WHERE workspace_id = $1 AND email = 'ada@example.com'`))
	})

	t.Run("a sync appends new entries in order", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.send(t, "teammate", testkit.JSON{"source_id": "adm-1", "email": d.TC.Owner.Email})
		d.send(t, "contact", contact("c-1", "ada@example.com"))
		first := conversation("cv-1", "c-1", message("p1", 0, "Hello?", "c-1"))
		id := d.send(t, "conversation", first).String("data.results.0.id")

		later := conversation("cv-1", "c-1", message("p1", 0, "Hello?", "c-1"), reply("p2", 5, "Hi!", "adm-1"),
			message("p3", 9, "Thanks", "c-1"))
		d.send(t, "conversation", later).AssertJSONPath("data.results.0.status", "updated")

		d.Owner.GetJSON("/api/v1/conversations/"+id).AssertOK().AssertJSONPath("data.message_count", 3)
		assert.Equal(t, 3, d.count(t, `SELECT count(*) FROM messages WHERE workspace_id = $1 AND type = 'message'`))
	})

	t.Run("a field a teammate changed is left alone and reported", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.send(t, "contact", contact("c-1", "ada@example.com"))
		first := conversation("cv-1", "c-1", message("p1", 0, "Hello?", "c-1"))
		first["status"] = "open"
		id := d.send(t, "conversation", first).String("data.results.0.id")

		d.Owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()

		first["status"] = "snoozed"
		first["snoozed_until"] = "2099-01-01T00:00:00Z"
		res := d.send(t, "conversation", first)
		res.AssertJSONPath("data.results.0.status", "updated").
			AssertJSONPath("data.results.0.conflicts.0.field", "status")

		d.Owner.GetJSON("/api/v1/conversations/"+id).AssertOK().AssertJSONPath("data.status", "closed")
		d.client.GetJSON(d.path("")).AssertOK().AssertJSONPath(`data.counts.#(kind=="conversation").conflicted`, 1)
	})

	t.Run("a record waits for what it names and is applied once that arrives", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		note := testkit.JSON{
			"source_id": "n-1", "contact_ref": "c-9", "body": "Call back.", "created_at": at(0),
			"author": testkit.JSON{"type": "system"},
		}
		d.send(t, "contact_note", note).AssertJSONPath("data.results.0.status", "deferred").
			AssertJSONPath("data.results.0.waiting_for.0.kind", "contact").
			AssertJSONPath("data.results.0.waiting_for.0.source_id", "c-9")

		res := d.send(t, "contact", contact("c-9", "late@example.com"))
		res.AssertJSONPath("data.resolved.0.source_id", "n-1").AssertJSONPath("data.resolved.0.status", "created")
		assert.Equal(t, 1, d.count(t, `SELECT count(*) FROM customer_notes WHERE workspace_id = $1`))
	})

	t.Run("a bad record fails alone", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		res := d.send(t, "contact", contact("c-1", "ok@example.com"), testkit.JSON{"source_id": "c-2", "kind": "robot"})
		res.AssertJSONPath("data.results.0.status", "created").AssertJSONPath("data.results.1.status", "failed").
			AssertJSONPath("data.results.1.error.field", "kind")
		d.client.GetJSON(d.path("")).AssertOK().AssertJSONPath("data.problems.0.source_id", "c-2")
	})

	t.Run("an undo removes what the migration created and nothing else", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.Desk.Customer(t, "kept", "Live one")
		d.send(t, "tag", testkit.JSON{"source_id": "tag-1", "name": "migrated"})
		d.send(t, "contact", contact("c-1", "ada@example.com"))
		d.send(t, "conversation", conversation("cv-1", "c-1", message("p1", 0, "Hello?", "c-1")))

		d.client.PostJSON(d.path("/rollbacks"), nil).AssertStatus(202).AssertJSONPath("data.status", "rolling_back")
		d.TC.Drain()

		d.client.GetJSON(d.path("")).AssertOK().AssertJSONPath("data.status", "rolled_back")
		assert.Equal(t, 1, d.count(t, `SELECT count(*) FROM conversations WHERE workspace_id = $1 AND deleted_at IS NULL`))
		assert.Equal(t, 0, d.count(t, `SELECT count(*) FROM tags WHERE workspace_id = $1 AND name = 'migrated'`))
		assert.Equal(t, 0, d.count(t, `SELECT count(*) FROM contacts WHERE workspace_id = $1 AND email = 'ada@example.com'
			AND deleted_at IS NULL`))

		d.client.PostJSON(d.path("/records"), testkit.JSON{"kind": "tag", "records": []testkit.JSON{{
			"source_id": "t", "name": "x",
		}}}).AssertStatus(409)
		d.client.PostJSON("/api/v1/migrations", testkit.JSON{"source": "acme-desk", "source_account": "workspace-1"}).
			AssertCreated()
	})

	t.Run("nothing a migration writes sets anything off", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		seen := tests.Record[any](d.TC)

		d.send(t, "teammate", testkit.JSON{"source_id": "adm-1", "email": d.TC.Owner.Email})
		d.send(t, "team", testkit.JSON{"source_id": "team-1", "name": "Support", "member_refs": []string{"adm-1"}})
		d.send(t, "tag", testkit.JSON{"source_id": "tag-1", "name": "vip"})
		d.send(t, "contact", contact("c-1", "ada@example.com"))
		d.send(t, "conversation", conversation("cv-1", "c-1",
			message("p1", 0, "Hello?", "c-1"),
			event("p2", 1, "adm-1", testkit.JSON{"type": "assignment", "assignee_ref": "adm-1"}),
			reply("p3", 2, "Hi!", "adm-1"),
			event("p4", 3, "adm-1", testkit.JSON{"type": "status", "status": "closed"}),
		))
		d.TC.Drain()

		assert.Empty(t, seen.All(), "a migration dispatched events")
	})

	t.Run("another workspace cannot reach the migration", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		other := open(t)

		other.client.GetJSON(d.path("")).AssertNotFound()
		other.client.PostJSON(d.path("/records"), testkit.JSON{"kind": "tag", "records": []testkit.JSON{{
			"source_id": "t", "name": "x",
		}}}).AssertNotFound()
	})

	t.Run("only callers who may run imports reach migrations", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		member, _ := d.Desk.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/migrations").AssertForbidden()
		d.TC.ActingAsToken(d.TC.Workspace.Owner, "contacts:write").GetJSON("/api/v1/migrations").AssertForbidden()
	})

	t.Run("an uploaded file sent again is the first copy", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		first := d.upload(t, "a.txt", "f-1", []byte("hello"))
		again := d.upload(t, "a.txt", "f-1", []byte("hello"))
		require.Equal(t, first, again)
	})
}

/*
png returns a tiny PNG picture.
*/
func png() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
}
