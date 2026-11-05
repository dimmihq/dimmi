package widget_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetContactLifecycle(t *testing.T) {
	t.Parallel()

	merge := func(t *testing.T, tc *tests.TestCase, from, into *visitor, intoUser bool) {
		t.Helper()

		ws := tc.Workspace.Workspace.ID
		in := widget.MergeInput{
			WorkspaceID: ws, From: from.session(t).ContactID, Into: into.session(t).ContactID, IntoUser: intoUser,
		}
		scoped := tenancy.With(t.Context(), ws.String())

		err := tc.App.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			_, err := tc.App.Actions.Widget.Contacts.Repoint(ctx, tx, in)

			return err
		})
		require.NoError(t, err)
	}

	t.Run("a lead merged into a lead keeps its session on the survivor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		from, into := startVisitor(t, tc), startVisitor(t, tc)

		merge(t, tc, from, into, false)
		merge(t, tc, from, into, false)

		assert.Equal(t, into.session(t).ContactID, from.session(t).ContactID)
		from.client().GetJSON("/widget/v1/session").AssertOK().AssertJSONPath("data.contact.id", into.contact)
	})

	t.Run("a lead merged into a signed-in customer loses its session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		from, into := startVisitor(t, tc), startVisitor(t, tc)

		merge(t, tc, from, into, true)

		from.client().GetJSON("/widget/v1/session").AssertUnauthorized().AssertJSONPath("code", "session_revoked")
		assert.Equal(t, "contact_merged", *from.session(t).RevokeReason)
	})

	t.Run("erasing a contact ends its sessions and forgets their pages, twice is fine", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		v.client().PostJSON("/widget/v1/pings", map[string]any{"page": map[string]any{"url": "https://acme.com/x"}})
		row := v.session(t)

		for range 2 {
			require.NoError(t, tc.App.Actions.Widget.Contacts.EraseContact(
				tenancy.With(t.Context(), row.WorkspaceID.String()), tc.App.DB, row.WorkspaceID, row.ContactID))
		}

		after := v.session(t)
		assert.Equal(t, "contact_erased", *after.RevokeReason)
		assert.Nil(t, after.CurrentPage)
		assert.JSONEq(t, "[]", string(after.RecentPages))
		assert.Nil(t, after.UserAgent)
	})

	t.Run("a merge or an erasure whose sessions cannot be written fails", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		from, into := startVisitor(t, tc), startVisitor(t, tc)
		ws := tc.Workspace.Workspace.ID
		in := widget.MergeInput{
			WorkspaceID: ws, From: from.session(t).ContactID, Into: into.session(t).ContactID,
		}
		tc.FailWrites("widget_sessions", "UPDATE")
		scoped := tenancy.With(t.Context(), ws.String())

		for _, intoUser := range []bool{false, true} {
			in.IntoUser = intoUser
			err := tc.App.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
				_, err := tc.App.Actions.Widget.Contacts.Repoint(ctx, tx, in)

				return err
			})
			require.Error(t, err)
		}

		require.Error(t, tc.App.Actions.Widget.Contacts.EraseContact(scoped, tc.App.DB, ws, in.From))
	})
}
