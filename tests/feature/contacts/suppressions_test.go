package contacts_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestEmailSuppressions(t *testing.T) {
	t.Parallel()

	suppress := func(t *testing.T, d tests.Desk, email, reason string) {
		t.Helper()

		ws := d.TC.Workspace.Workspace.ID
		ctx := tenancy.With(t.Context(), ws.String())
		err := d.TC.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			return models.New(tx).SuppressEmail(ctx, models.SuppressEmailParams{
				WorkspaceID: ws, Email: email, Reason: reason, Scope: "all", Source: "manual",
				Details: []byte(`{}`), Now: d.TC.Clock.Now(),
			})
		})
		require.NoError(t, err)
	}

	t.Run("lists what stops mail to a customer and lifts it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact := d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "ana@example.test"}).
			AssertCreated().String("data.id")
		other := d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "bo@example.test"}).
			AssertCreated().String("data.id")
		suppress(t, d, "ana@example.test", "hard_bounce")

		list := "/api/v1/email-suppressions?filter[contact]=" + contact
		id := d.Owner.GetJSON(list).AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.object", "email_suppression").
			AssertJSONPath("data.0.reason", "hard_bounce").String("data.0.id")
		d.Owner.GetJSON("/api/v1/email-suppressions?filter[contact]="+other).AssertOK().AssertJSONPath("data.#", 0)

		d.Owner.Delete("/api/v1/email-suppressions/" + id).AssertNoContent()
		d.Owner.Delete("/api/v1/email-suppressions/" + id).AssertNotFound()
		d.Owner.GetJSON(list).AssertOK().AssertJSONPath("data.#", 0)
	})

	t.Run("keeps a complaint for owners to lift", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact := d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "cy@example.test"}).
			AssertCreated().String("data.id")
		suppress(t, d, "cy@example.test", "complaint")
		id := d.Owner.GetJSON("/api/v1/email-suppressions?filter[contact]=" + contact).AssertOK().
			String("data.0.id")

		admin, _ := d.Join(t, enums.RoleAdmin)
		admin.Delete("/api/v1/email-suppressions/" + id).AssertForbidden()
		member, _ := d.Join(t, enums.RoleMember)
		member.Delete("/api/v1/email-suppressions/" + id).AssertForbidden()
		d.Owner.Delete("/api/v1/email-suppressions/" + id).AssertNoContent()
	})
}
