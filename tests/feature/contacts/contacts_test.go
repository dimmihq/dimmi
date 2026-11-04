package contacts_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

/*
contact adds a contact and returns its public id.
*/
func contact(t *testing.T, d tests.Desk, fields testkit.JSON) string {
	t.Helper()

	return d.Owner.PostJSON("/api/v1/contacts", fields).AssertCreated().String("data.id")
}

func TestContactTags(t *testing.T) {
	t.Parallel()

	t.Run("adds and removes tags, saying so in the update", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		updated := tests.Record[appevents.ContactUpdated](d.TC)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})

		res := d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"vip", "trial"}}).AssertOK().
			AssertJSONPath("data.tags.#", 2).
			AssertJSONPath("data.tags.0.name", "trial")
		id := res.String("data.tags.0.id")

		d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"VIP"}}).AssertOK().AssertJSONPath("data.tags.#", 2)
		d.Owner.PatchJSON(path, testkit.JSON{"remove_tags": []string{id}}).AssertOK().
			AssertJSONPath("data.tags.#", 1).
			AssertJSONPath("data.tags.0.name", "vip")
		d.Owner.PatchJSON(path, testkit.JSON{"remove_tags": []string{id}}).AssertOK()

		all := updated.All()
		require.Len(t, all, 2)
		assert.Equal(t, []string{"tags"}, all[0].Changes)
	})

	t.Run("refuses past fifty tags", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})
		names := make([]string, 50)

		for i := range names {
			names[i] = fmt.Sprintf("tag-%d", i)
		}

		d.Owner.PatchJSON(path, testkit.JSON{"add_tags": names}).AssertOK().AssertJSONPath("data.tags.#", 50)
		d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"overflow"}}).
			AssertStatus(422).AssertJSONPath("code", "tag_limit_reached")
		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.tags.#", 50)
	})

	t.Run("follows the person across their conversations", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		first := d.Customer(t, "bob", "One")
		d.Customer(t, "bob", "Two")
		path := "/api/v1/contacts/" + first.String("data.contact.id")

		d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"vip"}}).AssertOK()

		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.tags.0.name", "vip")
		d.Owner.GetJSON(path+"/conversations").AssertOK().AssertJSONPath("data.#", 2)
	})

	t.Run("filters contacts by every tag named", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ann := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})
		bob := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Bob"})
		contact(t, d, testkit.JSON{"name": "Cal"})

		res := d.Owner.PatchJSON(ann, testkit.JSON{"add_tags": []string{"a", "b"}}).AssertOK()
		a, b := res.String("data.tags.0.id"), res.String("data.tags.1.id")
		d.Owner.PatchJSON(bob, testkit.JSON{"add_tags": []string{a}}).AssertOK()

		d.Owner.GetJSON("/api/v1/contacts?filter[tag]="+a).AssertOK().AssertJSONPath("data.#", 2)
		d.Owner.GetJSON("/api/v1/contacts?filter[tag]="+a+","+b).AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.name", "Ann")
		d.Owner.GetJSON("/api/v1/contacts?filter[tag]=bogus").AssertStatus(400)
	})
}

func TestContactRecord(t *testing.T) {
	t.Parallel()

	t.Run("keeps notes, clears them with null, and caps their length", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})

		d.Owner.PatchJSON(path, testkit.JSON{"notes": "Prefers email."}).AssertOK().
			AssertJSONPath("data.notes", "Prefers email.")
		d.Owner.PatchJSON(path, testkit.JSON{"notes": nil}).AssertOK().AssertJSONPath("data.notes", nil)

		long := make([]byte, 10001)
		for i := range long {
			long[i] = 'x'
		}

		d.Owner.PatchJSON(path, testkit.JSON{"notes": string(long)}).AssertValidationError("notes")
	})

	t.Run("links and unlinks a company of the workspace", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		other := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})
		company := d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "Acme"}).AssertCreated().String("data.id")
		foreign := other.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "Theirs"}).AssertCreated().
			String("data.id")

		d.Owner.PatchJSON(path, testkit.JSON{"company_id": company}).AssertOK().
			AssertJSONPath("data.company.name", "Acme")
		d.Owner.GetJSON("/api/v1/contacts?filter[company]="+company).AssertOK().AssertJSONPath("data.#", 1)

		d.Owner.PatchJSON(path, testkit.JSON{"company_id": foreign}).
			AssertStatus(422).AssertJSONPath("code", "company_not_found")
		d.Owner.PatchJSON(path, testkit.JSON{"company_id": "bogus"}).
			AssertStatus(422).AssertJSONPath("code", "company_not_found")
		d.Owner.PatchJSON(path, testkit.JSON{"company_id": nil}).AssertOK().AssertJSONPath("data.company", nil)
	})

	t.Run("hides blocked contacts unless asked", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})
		contact(t, d, testkit.JSON{"name": "Bob"})

		d.Owner.PatchJSON(path, testkit.JSON{"blocked": true}).AssertOK()

		d.Owner.GetJSON("/api/v1/contacts").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.name", "Bob")
		d.Owner.GetJSON("/api/v1/contacts?filter[blocked]=true").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.name", "Ann")
		d.Owner.GetJSON("/api/v1/contacts?filter[blocked]=maybe").AssertStatus(400)
	})

	t.Run("filters by when they were seen or added and by custom attributes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(t, d, testkit.JSON{"name": "Ann", "custom_attributes": testkit.JSON{"plan": "pro", "seats": 3}})
		contact(t, d, testkit.JSON{"name": "Bob", "custom_attributes": testkit.JSON{"plan": "free"}})

		list := "/api/v1/contacts?"
		d.Owner.GetJSON(list+"filter[custom.plan]=pro").AssertOK().AssertJSONPath("data.0.name", "Ann").
			AssertJSONPath("data.#", 1)
		d.Owner.GetJSON(list+"filter[custom.seats]=3").AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.GetJSON(list + "filter[custom.a]=1&filter[custom.b]=1&filter[custom.c]=1&filter[custom.d]=1").
			AssertStatus(400)

		d.Owner.GetJSON(list+"filter[created_after]=2000-01-01T00:00:00Z").AssertOK().AssertJSONPath("data.#", 2)
		d.Owner.GetJSON(list+"filter[created_before]=2000-01-01T00:00:00Z").AssertOK().AssertJSONPath("data.#", 0)
		d.Owner.GetJSON(list+"filter[last_seen_after]=2100-01-01T00:00:00Z").AssertOK().AssertJSONPath("data.#", 0)
		d.Owner.GetJSON(list+"filter[last_seen_before]=2100-01-01T00:00:00Z").AssertOK().AssertJSONPath("data.#", 2)
		d.Owner.GetJSON(list + "filter[created_after]=soon").AssertStatus(400)
		d.Owner.GetJSON(list + "filter[company]=bogus").AssertStatus(400)
		d.Owner.GetJSON(list + "filter[nope]=1").AssertStatus(400)
	})
}

func TestUpsertContact(t *testing.T) {
	t.Parallel()

	const upsert = "/api/v1/contacts?upsert=true"

	t.Run("creates by external id as a user, then updates the same one", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		created := tests.Record[appevents.ContactCreated](d.TC)
		updated := tests.Record[appevents.ContactUpdated](d.TC)

		res := d.Owner.PostJSON(upsert, testkit.JSON{
			"external_id": "user_42", "email": "Bob@Acme.dev", "name": "Bob",
			"custom_attributes": testkit.JSON{"plan": "pro"},
		}).AssertCreated().
			AssertJSONPath("meta.upserted", "created").
			AssertJSONPath("data.kind", "user").
			AssertJSONPath("data.email", "bob@acme.dev")
		id := res.String("data.id")

		d.Owner.PostJSON(upsert, testkit.JSON{
			"external_id": "user_42", "name": "Robert", "custom_attributes": testkit.JSON{"plan": nil, "seats": 2},
		}).AssertOK().
			AssertJSONPath("meta.upserted", "updated").
			AssertJSONPath("data.id", id).
			AssertJSONPath("data.name", "Robert").
			AssertJSONPath("data.email", "bob@acme.dev").
			AssertJSONPath("data.custom_attributes.seats", 2).
			AssertJSONMissing("data.custom_attributes.plan")

		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "user_42", "name": "Robert"}).AssertOK()
		assert.Len(t, created.All(), 1)
		assert.Len(t, updated.All(), 1)
	})

	t.Run("matches a proven address and sets the external id once", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := contact(t, d, testkit.JSON{"email": "ann@acme.dev"})

		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "u1", "email": "ANN@acme.dev"}).AssertOK().
			AssertJSONPath("data.id", id).
			AssertJSONPath("data.kind", "user").
			AssertJSONPath("data.external_id", "u1").
			AssertJSONPath("data.email_verified", true)
		d.Owner.PostJSON(upsert, testkit.JSON{"email": "ann@acme.dev", "name": "Ann"}).AssertCreated().
			AssertJSONPath("data.kind", "lead")
	})

	t.Run("makes a second contact when the address belongs to another external id", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		first := d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "u1", "email": "shared@acme.dev"}).
			AssertCreated()
		second := d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "u2", "email": "shared@acme.dev"}).
			AssertCreated()

		assert.NotEqual(t, first.String("data.id"), second.String("data.id"))
		d.Owner.GetJSON("/api/v1/contacts/"+first.String("data.id")+"/identities").AssertOK().
			AssertJSONPath("data.0.value", "shared@acme.dev")
		d.Owner.GetJSON("/api/v1/contacts/"+second.String("data.id")+"/identities").AssertOK().
			AssertJSONPath("data.#", 0)
	})

	t.Run("never chooses a lead who only typed the address, naming them instead", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace.ID
		_, err := d.TC.App.DB.Exec(t.Context(), `
			INSERT INTO contacts (workspace_id, kind, email, email_verified, name)
			VALUES ($1, 'lead', 'typed@acme.dev', false, 'Typist')`, ws)
		require.NoError(t, err)

		d.Owner.PostJSON(upsert, testkit.JSON{"email": "typed@acme.dev", "name": "Real"}).AssertCreated().
			AssertJSONPath("data.name", "Real").
			AssertJSONPath("data.email_verified", true).
			AssertJSONPath("meta.possible_duplicates.#", 1).
			AssertJSONPath("meta.possible_duplicates.0.name", "Typist")
	})

	t.Run("links the company it names", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		company := testkit.JSON{"external_id": "org_7", "name": "Acme", "domain": "WWW.Acme.dev"}

		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "u1", "company": company}).AssertCreated().
			AssertJSONPath("data.company.name", "Acme").
			AssertJSONPath("data.company.domain", "acme.dev")
		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "u2", "company": testkit.JSON{"domain": "acme.dev"}}).
			AssertCreated().AssertJSONPath("data.company.name", "Acme")
		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "u3", "company": testkit.JSON{"name": "x"}}).
			AssertStatus(422).AssertJSONPath("code", "upsert_key_required")
		d.Owner.GetJSON("/api/v1/companies").AssertOK().AssertJSONPath("data.#", 1)
	})

	t.Run("refuses what it cannot match on or read", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON(upsert, testkit.JSON{"name": "Nobody"}).
			AssertStatus(422).AssertJSONPath("code", "upsert_key_required")
		d.Owner.PostJSON("/api/v1/contacts?upsert=yes", testkit.JSON{"name": "x"}).
			AssertStatus(400).AssertJSONPath("code", "invalid_parameter")
		d.Owner.PostJSON("/api/v1/contacts?upsert=false", testkit.JSON{"name": "x"}).AssertCreated()
		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "u9", "custom_attributes": testkit.JSON{"Bad": 1}}).
			AssertStatus(422)

		reader := d.TC.ActingAsToken(d.TC.Workspace.Owner, "contacts:read")
		reader.PostJSON(upsert, testkit.JSON{"external_id": "u1"}).AssertForbidden()
	})
}

func TestContactIdentities(t *testing.T) {
	t.Parallel()

	t.Run("lists and detaches an identity", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"email": "ann@acme.dev"})

		res := d.Owner.GetJSON(path+"/identities").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.type", "email").
			AssertJSONPath("data.0.verified", true)
		id := res.String("data.0.id")

		d.Owner.Delete(path + "/identities/" + id).AssertNoContent()
		d.Owner.Delete(path + "/identities/" + id).AssertNotFound()
		d.Owner.Delete(path + "/identities/bogus").AssertNotFound()
		d.Owner.Delete("/api/v1/contacts/bogus/identities/" + id).AssertNotFound()
		d.Owner.GetJSON("/api/v1/contacts/bogus/identities").AssertNotFound()
		d.Owner.GetJSON(path+"/identities").AssertOK().AssertJSONPath("data.#", 0)

		d.TC.ActingAsToken(d.TC.Workspace.Owner, "contacts:read").Delete(path + "/identities/" + id).AssertForbidden()
	})

	t.Run("another workspace's contact is not found", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		other := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, other, testkit.JSON{"email": "theirs@acme.dev"})

		d.Owner.GetJSON(path + "/identities").AssertNotFound()
		d.Owner.GetJSON(path + "/duplicates").AssertNotFound()
		d.Owner.GetJSON(path + "/conversations").AssertNotFound()
		d.Owner.Delete(path).AssertNotFound()
	})
}

func TestContactDuplicates(t *testing.T) {
	t.Parallel()

	t.Run("suggests same address, phone, messaging number, and name at company with reasons", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace.ID
		company := d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "Acme"}).AssertCreated().String("data.id")
		id := contact(t, d, testkit.JSON{"name": "Bob", "email": "bob@acme.dev", "phone": "+14155550100"})
		path := "/api/v1/contacts/" + id
		d.Owner.PatchJSON(path, testkit.JSON{"company_id": company}).AssertOK()

		var chat string

		err := d.TC.App.DB.QueryRow(t.Context(), `
			INSERT INTO contacts (workspace_id, kind, name) VALUES ($1, 'lead', 'Chat') RETURNING id::text`, ws).
			Scan(&chat)
		require.NoError(t, err)
		_, err = d.TC.App.DB.Exec(t.Context(), `
			INSERT INTO contact_identities (workspace_id, contact_id, type, value, verified)
			VALUES ($1, $2::uuid, 'whatsapp', '14155550100', true)`, ws, chat)
		require.NoError(t, err)

		_, err = d.TC.App.DB.Exec(t.Context(), `
			INSERT INTO contacts (workspace_id, kind, email, name) VALUES ($1, 'lead', 'BOB@acme.dev', 'Typed')`, ws)
		require.NoError(t, err)

		contact(t, d, testkit.JSON{"name": "Phone twin", "phone": "+1 415 555 0100"})
		namesake := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Bob"})
		d.Owner.PatchJSON(namesake, testkit.JSON{"company_id": company}).AssertOK()
		contact(t, d, testkit.JSON{"name": "Stranger", "email": "stranger@acme.dev"})

		res := d.Owner.GetJSON(path+"/duplicates").AssertOK().AssertJSONPath("data.#", 4).
			AssertJSONPath("data.0.contact.name", "Typed").
			AssertJSONPath("data.0.reasons.0", "same_email").
			AssertJSONPath("data.1.reasons.0", "same_phone")
		assert.Contains(t, string(res.Body), "same_whatsapp")
		assert.Contains(t, string(res.Body), "same_name_company")

		reader := d.TC.ActingAsToken(d.TC.Workspace.Owner, "conversations:read")
		reader.GetJSON(path + "/duplicates").AssertForbidden()
		reader.GetJSON(path + "/identities").AssertForbidden()
	})
}

func TestDeleteContact(t *testing.T) {
	t.Parallel()

	t.Run("removes a contact without conversations and frees the address", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"email": "ann@acme.dev"})

		d.Owner.Delete(path).AssertNoContent()
		d.Owner.GetJSON(path).AssertNotFound()
		d.Owner.Delete(path).AssertNotFound()
		d.Owner.Delete("/api/v1/contacts/bogus").AssertNotFound()
		contact(t, d, testkit.JSON{"email": "ann@acme.dev"})
	})

	t.Run("refuses a contact with conversations", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		res := d.Customer(t, "bob", "Hi")
		path := "/api/v1/contacts/" + res.String("data.contact.id")

		d.Owner.Delete(path).AssertStatus(409).AssertJSONPath("code", "contact_has_conversations")
		d.TC.ActingAsToken(d.TC.Workspace.Owner, "contacts:read").Delete(path).AssertForbidden()
	})

	t.Run("lets members remove but not read beyond their abilities", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)

		member.Delete("/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})).AssertNoContent()
	})
}

func TestContactConversations(t *testing.T) {
	t.Parallel()

	t.Run("pages the contact's conversations newest first, whatever their status", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		first := d.Customer(t, "bob", "First")
		d.TC.Clock.Travel(60_000_000_000)
		second := d.Customer(t, "bob", "Second")
		d.Customer(t, "ann", "Not theirs")
		d.Owner.PatchJSON("/api/v1/conversations/"+first.String("data.id"), testkit.JSON{"status": "closed"}).AssertOK()

		path := "/api/v1/contacts/" + first.String("data.contact.id") + "/conversations"
		page := d.Owner.GetJSON(path+"?limit=1").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.id", second.String("data.id"))

		d.Owner.GetJSON(path+"?limit=1&cursor="+page.String("meta.next_cursor")).AssertOK().
			AssertJSONPath("data.0.status", "closed")
		d.Owner.GetJSON(path + "?cursor=forged").AssertStatus(400)
		d.Owner.GetJSON("/api/v1/contacts/bogus/conversations").AssertNotFound()
		d.TC.ActingAsToken(d.TC.Workspace.Owner, "contacts:read").GetJSON(path).AssertForbidden()
	})
}
