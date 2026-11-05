package privacy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/retention"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
closed opens a conversation from the named customer and closes it,
returning its path.
*/
func closed(t *testing.T, d tests.Desk, name string) string {
	t.Helper()

	path := "/api/v1/conversations/" + d.Customer(t, name, "Old question").String("data.id")
	d.Owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK()

	return path
}

/*
age moves the closing and last activity of every closed conversation
back 40 days, as if they had sat untouched that long.
*/
func age(t *testing.T, d tests.Desk) {
	t.Helper()

	back := d.TC.Clock.Now().Add(-40 * 24 * time.Hour)
	_, err := d.TC.Operator().Exec(t.Context(),
		`UPDATE conversations SET closed_at = $1, last_activity_at = $1 WHERE status = 'closed'`, back)
	require.NoError(t, err)
}

func TestRetention(t *testing.T) {
	t.Parallel()

	t.Run("keeps closed conversations for ever until a period is set", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.GetJSON("/api/v1/workspace").AssertOK().
			AssertJSONPath("data.retention.closed_conversation_days", nil).
			AssertJSONPath("data.retention.spam_days", 30).
			AssertJSONPath("data.retention.trash_days", 30).
			AssertJSONPath("data.retention.raw_email_days", 30).
			AssertJSONPath("data.retention.export_days", 7)
	})

	t.Run("saves periods within their ranges and keeps what was not sent", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		retention := func(body testkit.JSON) *testkit.Response {
			return d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"retention": body})
		}

		retention(testkit.JSON{"closed_conversation_days": 365, "raw_email_days": 0, "export_days": 14}).AssertOK().
			AssertJSONPath("data.retention.closed_conversation_days", 365).
			AssertJSONPath("data.retention.raw_email_days", 0).
			AssertJSONPath("data.retention.export_days", 14)
		retention(testkit.JSON{"spam_days": 7, "trash_days": 365}).AssertOK().
			AssertJSONPath("data.retention.closed_conversation_days", 365).
			AssertJSONPath("data.retention.trash_days", 365)
		retention(testkit.JSON{"spam_days": 7}).AssertOK()
		retention(testkit.JSON{"closed_conversation_days": nil}).AssertOK().
			AssertJSONPath("data.retention.closed_conversation_days", nil)

		for _, body := range []testkit.JSON{
			{"closed_conversation_days": 29},
			{"closed_conversation_days": 3651},
			{"spam_days": 6},
			{"trash_days": 366},
			{"raw_email_days": -1},
			{"export_days": 31},
		} {
			retention(body).AssertStatus(422).AssertJSONPath("code", "invalid_retention")
		}

		d.TC.Table("audit_logs").AssertCount(3, testkit.Row{"event": "workspace.updated"})
	})

	t.Run("counts what a period would move before it is set", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		closed(t, d, "ann")
		closed(t, d, "bob")
		d.Customer(t, "cy", "Still open")
		age(t, d)

		d.Owner.PostJSON("/api/v1/retention-previews", testkit.JSON{"closed_conversation_days": 30}).AssertOK().
			AssertJSONPath("data.object", "retention_preview").
			AssertJSONPath("data.count", 2)
		d.Owner.PostJSON("/api/v1/retention-previews", testkit.JSON{"closed_conversation_days": 60}).AssertOK().
			AssertJSONPath("data.count", 0)
		d.Owner.PostJSON("/api/v1/retention-previews", testkit.JSON{"closed_conversation_days": 10}).
			AssertStatus(422).AssertJSONPath("code", "invalid_retention")
	})

	t.Run("moves closed conversations past the period to the trash, never the day it is set", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		old := closed(t, d, "ann")
		touched := closed(t, d, "bob")
		age(t, d)
		d.Owner.PostJSON(touched+"/notes", testkit.JSON{"body": "Still relevant"}).AssertCreated()
		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"retention": testkit.JSON{"closed_conversation_days": 30}}).
			AssertOK()
		apply := d.TC.App.Actions.Inbox.ApplyRetention

		moved, err := apply.Handle(t.Context())
		require.NoError(t, err)
		assert.Equal(t, int64(0), moved)

		d.TC.Clock.Travel(25 * time.Hour)
		moved, err = apply.Handle(t.Context())
		require.NoError(t, err)
		assert.Equal(t, int64(1), moved)
		d.Owner.GetJSON(old).AssertNotFound()
		d.Owner.GetJSON("/api/v1/conversations?filter[view]=deleted").AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[status]=any").AssertOK().
			AssertJSONPath("data.#", 1)
	})

	t.Run("leaves a conversation the retention guard holds", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Retention = holdAll{} }))
		inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
		require.NoError(t, err)
		d := tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
		closed(t, d, "ann")
		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"retention": testkit.JSON{"closed_conversation_days": 30}}).
			AssertOK()
		age(t, d)
		tc.Clock.Travel(25 * time.Hour)

		moved, err := tc.App.Actions.Inbox.ApplyRetention.Handle(t.Context())
		require.NoError(t, err)
		assert.Equal(t, int64(0), moved)
		d.Owner.GetJSON("/api/v1/conversations?filter[view]=deleted").AssertOK().AssertJSONPath("data.#", 0)
	})

	t.Run("lets only admins change retention and members not preview it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)
		none := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, nil)

		member.PatchJSON("/api/v1/workspace", testkit.JSON{"retention": testkit.JSON{"spam_days": 7}}).
			AssertForbidden()
		member.PostJSON("/api/v1/retention-previews", testkit.JSON{"closed_conversation_days": 30}).
			AssertForbidden()
		_, err := d.TC.App.Actions.Inbox.PreviewRetention.Handle(t.Context(), retention.PreviewRetentionInput{
			Membership: none, ClosedConversationDays: 30,
		})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
	})
}

/*
brokenGuard is a retention guard that cannot answer.
*/
type brokenGuard struct{}

/*
MayDestroy fails.
*/
func (brokenGuard) MayDestroy(context.Context, ids.UUID, contracts.RetentionSubject) (bool, error) {
	return false, errors.New("holds unavailable")
}

func TestRetentionFailures(t *testing.T) {
	t.Parallel()

	t.Run("fails the round and the preview when conversations cannot be read or moved", func(t *testing.T) {
		t.Parallel()

		for _, broken := range []func(tc *tests.TestCase){
			func(tc *tests.TestCase) { tc.FailWrites("conversations", "UPDATE") },
			func(tc *tests.TestCase) { tc.BreakTable("conversations") },
		} {
			d := tests.NewDesk(t)
			closed(t, d, "ann")
			d.Owner.PatchJSON("/api/v1/workspace",
				testkit.JSON{"retention": testkit.JSON{"closed_conversation_days": 30}}).AssertOK()
			age(t, d)
			d.TC.Clock.Travel(25 * time.Hour)
			broken(d.TC)

			_, err := d.TC.App.Actions.Inbox.ApplyRetention.Handle(t.Context())
			require.Error(t, err)
		}

		d := tests.NewDesk(t)
		d.TC.BreakTable("conversations")
		d.Owner.PostJSON("/api/v1/retention-previews", testkit.JSON{"closed_conversation_days": 30}).AssertStatus(500)
	})

	t.Run("fails the round when the guard cannot answer", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Retention = brokenGuard{} }))
		inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
		require.NoError(t, err)
		d := tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
		closed(t, d, "ann")
		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"retention": testkit.JSON{"closed_conversation_days": 30}}).
			AssertOK()
		age(t, d)
		tc.Clock.Travel(25 * time.Hour)

		_, err = tc.App.Actions.Inbox.ApplyRetention.Handle(t.Context())
		require.ErrorContains(t, err, "ask retention guard")
	})
}
