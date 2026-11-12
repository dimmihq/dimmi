package contacts_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestMergeContacts(t *testing.T) {
	t.Parallel()

	t.Run("moves conversations, tags and identities, filling the target's blanks", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		target := d.Customer(t, "ann", "Billing").String("data.contact.id")
		res := d.Customer(t, "anna", "Login")
		source := res.String("data.contact.id")
		path := "/api/v1/contacts/" + target

		d.Owner.PatchJSON(path, testkit.JSON{
			"name": "Ann", "custom_attributes": testkit.JSON{"plan": "pro"},
		}).AssertOK()
		d.Owner.PatchJSON("/api/v1/contacts/"+source, testkit.JSON{
			"phone": "+14155550100", "add_tags": []string{"vip"},
			"custom_attributes": testkit.JSON{"plan": "free", "seats": 5},
		}).AssertOK()

		d.Owner.PostJSON(path+"/merges", testkit.JSON{"source_id": source}).AssertOK().
			AssertJSONPath("data.id", target).
			AssertJSONPath("data.name", "Ann").
			AssertJSONPath("data.phone", "+14155550100").
			AssertJSONPath("data.custom_attributes.plan", "pro").
			AssertJSONPath("data.custom_attributes.seats", 5).
			AssertJSONPath("data.tags.0.name", "vip")

		d.Owner.GetJSON(path+"/conversations").AssertOK().AssertJSONPath("data.#", 2)
		d.Owner.GetJSON("/api/v1/contacts/"+source).AssertOK().AssertJSONPath("data.id", target)
		d.Owner.GetJSON("/api/v1/conversations/"+res.String("data.id")).AssertOK().
			AssertJSONPath("data.contact.id", target)
		identities := d.Owner.GetJSON(path + "/identities").AssertOK()
		require.Contains(t, string(identities.Body), "anna@example.com")
		d.TC.Table("audit_logs").AssertCount(1, testkit.Row{"event": "contact.merged"})
	})

	t.Run("refuses two identified users, itself and an unknown contact", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		one := contact(t, d, testkit.JSON{"external_id": "u-1", "name": "One"})
		two := contact(t, d, testkit.JSON{"external_id": "u-2", "name": "Two"})
		path := "/api/v1/contacts/" + one + "/merges"

		d.Owner.PostJSON(path, testkit.JSON{"source_id": two}).
			AssertStatus(409).AssertJSONPath("code", "merge_two_users")
		d.Owner.PostJSON(path, testkit.JSON{"source_id": one}).
			AssertStatus(422).AssertJSONPath("code", "merge_into_itself")
		d.Owner.PostJSON(path, testkit.JSON{"source_id": "ctc_bogus"}).AssertNotFound()
	})

	t.Run("hands a lead's merge the user's external id", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		lead := contact(t, d, testkit.JSON{"name": "Lead", "email": "lead@acme.dev"})
		user := contact(t, d, testkit.JSON{"external_id": "u-9", "name": "User"})

		d.Owner.PostJSON("/api/v1/contacts/"+lead+"/merges", testkit.JSON{"source_id": user}).AssertOK().
			AssertJSONPath("data.external_id", "u-9").
			AssertJSONPath("data.name", "Lead")
		d.Owner.PostJSON("/api/v1/contacts/"+lead+"/merges", testkit.JSON{"source_id": user}).AssertNotFound()
	})

	t.Run("keeps merging to those who may write contacts", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		one := contact(t, d, testkit.JSON{"name": "One"})
		two := contact(t, d, testkit.JSON{"name": "Two"})
		reader := d.TC.ActingAsToken(d.TC.Workspace.Owner, "contacts:read")

		reader.PostJSON("/api/v1/contacts/"+one+"/merges", testkit.JSON{"source_id": two}).AssertForbidden()
		d.Owner.GetJSON("/api/v1/contacts/"+two).AssertOK().AssertJSONPath("data.id", two)
		require.NotEqual(t, one, two)
	})
}
