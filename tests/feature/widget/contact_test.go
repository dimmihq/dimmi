package widget_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetContact(t *testing.T) {
	t.Parallel()

	t.Run("a lead says who they are, cleaned", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		res := v.client().PatchJSON("/widget/v1/contact", testkit.JSON{
			"name": "  Ada\u202e Lovelace\u0000 ", "email": "Ada@MÜNCHEN.de", "phone": "+49 30 123456",
		}).AssertOK()

		res.AssertJSONPath("data.name", "Ada Lovelace")
		res.AssertJSONPath("data.email", "ada@xn--mnchen-3ya.de")
		res.AssertJSONPath("data.phone", "+4930123456")
		res.AssertJSONPath("data.email_verified", false)
		tc.Table("contact_identities").AssertCount(1, testkit.Row{"type": "email", "value": "ada@xn--mnchen-3ya.de"})
		v.client().GetJSON("/widget/v1/contact").AssertOK().AssertJSONPath("data.name", "Ada Lovelace")
	})

	t.Run("an address another contact holds is never taken from them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		first, second := startVisitor(t, tc), startVisitor(t, tc)

		first.client().PatchJSON("/widget/v1/contact", testkit.JSON{"email": "kim@example.com"}).AssertOK()
		second.client().PatchJSON("/widget/v1/contact", testkit.JSON{"email": "kim@example.com"}).AssertOK().
			AssertJSONPath("data.email", "kim@example.com")

		tc.Table("contact_identities").AssertCount(1, testkit.Row{"type": "email", "value": "kim@example.com"})
		tc.Table("contacts").AssertCount(2, testkit.Row{"email": "kim@example.com"})
	})

	t.Run("an attribute the widget writes is no longer trusted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		_, err := tc.Operator().Exec(t.Context(),
			"UPDATE contacts SET trusted_attribute_keys = '{plan,stripe_customer_id}' WHERE workspace_id = $1",
			tc.Workspace.Workspace.ID)
		require.NoError(t, err)

		v.client().PatchJSON("/widget/v1/contact", testkit.JSON{
			"custom_attributes": testkit.JSON{"stripe_customer_id": "cus_x"},
		}).AssertOK().AssertJSONPath("data.custom_attributes.stripe_customer_id", "cus_x")

		contact := sessionContact(t, tc, v)
		assert.Equal(t, []string{"plan"}, contact.TrustedAttributeKeys)
	})

	t.Run("a signed-in customer's profile is locked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		_, err := tc.Operator().Exec(t.Context(), "UPDATE contacts SET kind = 'user' WHERE workspace_id = $1",
			tc.Workspace.Workspace.ID)
		require.NoError(t, err)

		v.client().PatchJSON("/widget/v1/contact", testkit.JSON{"name": "Eve"}).
			AssertForbidden().AssertJSONPath("code", "identity_locked")
	})

	for name, body := range map[string]testkit.JSON{
		"name":       {"name": " "},
		"email":      {"email": "Ada <ada@example.com>"},
		"phone":      {"phone": "030 123"},
		"attributes": {"custom_attributes": testkit.JSON{"Bad Key": 1}},
	} {
		t.Run("refuses a bad "+name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)
			v := startVisitor(t, tc)

			v.client().PatchJSON("/widget/v1/contact", body).AssertStatus(fhttp.StatusUnprocessableEntity)
		})
	}
}

/*
sessionContact reads the visitor's contact.
*/
func sessionContact(t *testing.T, tc *tests.TestCase, v *visitor) models.Contact {
	t.Helper()

	var out models.Contact

	session := v.session(t)
	inWorkspace(t, tc, session.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.FindContact(ctx, models.FindContactParams{WorkspaceID: session.WorkspaceID, ID: session.ContactID})

		return err
	})
	require.Equal(t, string(enums.ContactKindLead), out.Kind)

	return out
}
