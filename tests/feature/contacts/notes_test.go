package contacts_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestContactNotes(t *testing.T) {
	t.Parallel()

	t.Run("keeps separate notes with who wrote each, newest first, the pinned one on top", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"email": "ann@acme.dev"}) + "/notes"

		first := d.Owner.PostJSON(path, testkit.JSON{"body": "Prefers email."}).AssertCreated().
			AssertJSONPath("data.pinned", false).String("data.id")
		d.Owner.PostJSON(path, testkit.JSON{"body": "Renews in March."}).AssertCreated()

		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.#", 2).
			AssertJSONPath("data.0.body", "Renews in March.").AssertJSONPath("data.1.body", "Prefers email.")

		d.Owner.PatchJSON("/api/v1/contact-notes/"+first, testkit.JSON{"pinned": true}).AssertOK().
			AssertJSONPath("data.pinned", true)
		list := d.Owner.GetJSON(path).AssertOK()
		list.AssertJSONPath("data.0.body", "Prefers email.").AssertJSONPath("data.0.pinned", true).
			AssertJSONPath("data.1.pinned", false)

		if list.String("data.0.author.name") == "" {
			t.Fatal("the note does not name who wrote it")
		}
	})

	t.Run("lets only the teammate who wrote a note change or delete it, and anyone pin it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"email": "ann@acme.dev"}) + "/notes"
		note := "/api/v1/contact-notes/" + d.Owner.PostJSON(path, testkit.JSON{"body": "VIP."}).AssertCreated().
			String("data.id")
		ben, _ := d.Join(t, enums.RoleMember)

		ben.PatchJSON(note, testkit.JSON{"body": "Not a VIP."}).AssertForbidden().
			AssertJSONPath("code", "contact_note_not_yours")
		ben.Delete(note).AssertForbidden()
		ben.PatchJSON(note, testkit.JSON{"pinned": true}).AssertOK().AssertJSONPath("data.body", "VIP.")

		d.Owner.PatchJSON(note, testkit.JSON{"body": "VIP, call first."}).AssertOK().
			AssertJSONPath("data.body", "VIP, call first.")
		d.Owner.Delete(note).AssertNoContent()
		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.#", 0)
	})

	t.Run("keeps a company's notes apart from its people's", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		person := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"email": "ann@acme.dev"}) + "/notes"
		company := "/api/v1/companies/" + d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "Acme"}).
			AssertCreated().String("data.id") + "/notes"

		d.Owner.PostJSON(person, testkit.JSON{"body": "Prefers email."}).AssertCreated()
		note := d.Owner.PostJSON(company, testkit.JSON{"body": "Renews in March."}).AssertCreated().
			AssertJSONPath("data.contact_id", nil).String("data.id")

		d.Owner.GetJSON(company).AssertOK().AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.body", "Renews in March.")
		d.Owner.GetJSON(person).AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.PatchJSON("/api/v1/contact-notes/"+note, testkit.JSON{"pinned": true}).AssertOK().
			AssertJSONPath("data.pinned", true)
		d.Owner.GetJSON("/api/v1/companies/cmp_01jbd0k3a8x9t2m4q5r6s7v8w9/notes").AssertNotFound()
	})

	t.Run("refuses an empty note and a caller who cannot write contacts", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"email": "ann@acme.dev"}) + "/notes"

		d.Owner.PostJSON(path, testkit.JSON{"body": "   "}).AssertStatus(422)
		d.TC.ActingAsToken(d.TC.Workspace.Owner, "contacts:read").PostJSON(path, testkit.JSON{"body": "Hi"}).
			AssertForbidden()
		d.TC.ActingAsToken(d.TC.Workspace.Owner, "contacts:read").GetJSON(path).AssertOK()
	})
}
