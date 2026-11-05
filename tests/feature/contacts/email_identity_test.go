package contacts_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestContactEmailIdentity(t *testing.T) {
	t.Parallel()

	t.Run("moves the email identity to a changed address", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"email": "old@acme.dev"})

		d.Owner.PatchJSON(path, testkit.JSON{"email": "new@acme.dev"}).AssertOK()

		d.Owner.GetJSON(path+"/identities").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.value", "new@acme.dev")
	})

	t.Run("drops the email identity when the address is cleared", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"email": "gone@acme.dev"})

		d.Owner.PatchJSON(path, testkit.JSON{"email": nil}).AssertOK()

		d.Owner.GetJSON(path+"/identities").AssertOK().AssertJSONPath("data.#", 0)
		contact(t, d, testkit.JSON{"email": "gone@acme.dev"})
	})

	t.Run("refuses an address another contact receives mail at", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(t, d, testkit.JSON{"email": "ann@acme.dev"})
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"email": "bob@acme.dev"})

		d.Owner.PatchJSON(path, testkit.JSON{"email": "Ann@acme.dev"}).AssertStatus(409).
			AssertJSONPath("code", "contact_email_exists")
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "ann@acme.dev", "external_id": "u-1"}).
			AssertStatus(409).AssertJSONPath("code", "contact_email_exists")
		d.Owner.GetJSON(path+"/identities").AssertOK().AssertJSONPath("data.0.value", "bob@acme.dev")
	})
}
