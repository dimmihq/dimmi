package contacts_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestDirectoryPreferences(t *testing.T) {
	t.Parallel()

	t.Run("keeps the people list's columns and the pinned attributes with the member", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PatchJSON("/api/v1/inbox-preferences", testkit.JSON{
			"views":             testkit.JSON{"people": testkit.JSON{"columns": []string{"phone", "kind"}}},
			"pinned_attributes": []string{"plan", "seats"},
		}).AssertOK()

		d.Owner.GetJSON("/api/v1/inbox-preferences").AssertOK().
			AssertJSONPath("data.views.people.columns.#", 2).
			AssertJSONPath("data.views.people.columns.0", "phone").
			AssertJSONPath("data.pinned_attributes.1", "seats")
	})
}
