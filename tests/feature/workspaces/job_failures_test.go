package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
assertUnfinished runs the job and checks it did not complete.
*/
func assertUnfinished(tc *tests.TestCase, args queue.Args) {
	tc.T.Helper()

	require.NoError(tc.T, tc.App.Queue.Push(tc.T.Context(), args))
	tc.Drain()
	tc.Table("river_job").AssertCount(0, testkit.Row{"kind": args.Kind(), "state": "completed"})
}

/*
assertUnqueued runs the job while nothing can be queued and checks it
did not complete.
*/
func assertUnqueued(tc *tests.TestCase, args queue.Args) {
	tc.T.Helper()

	require.NoError(tc.T, tc.App.Queue.Push(tc.T.Context(), args))
	tc.FailWrites("river_job", "INSERT")
	tc.Drain()
	tc.Table("river_job").AssertCount(0, testkit.Row{"kind": args.Kind(), "state": "completed"})
}

/*
soonPurged trashes the test case's workspace and moves its purge within
the reminder notice.
*/
func soonPurged(t *testing.T, tc *tests.TestCase) {
	t.Helper()

	trash(tc).AssertOK()
	exec(t, tc, `UPDATE workspaces SET purge_after = $1`, tests.Epoch().Add(2*24*time.Hour))
}

func TestPurgeWorkspaceFailures(t *testing.T) {
	t.Parallel()

	for name, out := range map[string]struct {
		opts   []tests.Option
		breaks func(tc *tests.TestCase)
		kept   bool
	}{
		"the workspace cannot be read": {breaks: func(tc *tests.TestCase) { tc.BreakTable("workspaces") }},
		"the retention guard cannot answer": {
			opts: []tests.Option{tests.Before(func(a *app.App) { a.Retention = failingGuard{} })}, kept: true,
		},
		"the purge cannot be marked": {
			breaks: func(tc *tests.TestCase) { tc.FailWrites("workspaces", "UPDATE") }, kept: true,
		},
		"its rows cannot be deleted": {
			breaks: func(tc *tests.TestCase) { tc.FailWrites("workspace_members", "DELETE") }, kept: true,
		},
		"its row cannot be deleted": {
			breaks: func(tc *tests.TestCase) { tc.FailWrites("workspaces", "DELETE") }, kept: true,
		},
		"its organization cannot be retired": {
			breaks: func(tc *tests.TestCase) { tc.FailWrites("organizations", "UPDATE") }, kept: true,
		},
	} {
		t.Run("keeps the workspace when "+name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t, out.opts...)
			overdue(t, tc)
			if out.breaks != nil {
				out.breaks(tc)
			}

			assertUnfinished(tc, jobs.PurgeWorkspaceArgs{WorkspaceID: tc.Workspace.Workspace.ID})

			if out.kept {
				tc.Table("workspaces").AssertHas(testkit.Row{"id": tc.Workspace.Workspace.ID})
			}
		})
	}
}

func TestWorkspaceMaintenanceFailures(t *testing.T) {
	t.Parallel()

	t.Run("queues no purge when workspaces cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("workspaces")

		assertUnfinished(tc, jobs.ScheduleWorkspacePurgesArgs{})
	})

	t.Run("retries when a purge cannot be queued", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		overdue(t, tc)

		assertUnqueued(tc, jobs.ScheduleWorkspacePurgesArgs{})
		tc.Table("workspaces").AssertHas(testkit.Row{"id": tc.Workspace.Workspace.ID})
	})

	t.Run("reminds nobody when workspaces cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("workspaces")

		assertUnfinished(tc, jobs.SendWorkspaceDeletionRemindersArgs{})
	})

	t.Run("records no reminder when the owners cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		soonPurged(t, tc)
		tc.BreakTable("workspace_members")

		assertUnfinished(tc, jobs.SendWorkspaceDeletionRemindersArgs{})
	})

	t.Run("records no reminder when its mail cannot be queued", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		soonPurged(t, tc)

		assertUnqueued(tc, jobs.SendWorkspaceDeletionRemindersArgs{})
	})

	t.Run("mails no reminder when it cannot be recorded", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		soonPurged(t, tc)
		tc.FailWrites("workspaces", "UPDATE")

		assertUnfinished(tc, jobs.SendWorkspaceDeletionRemindersArgs{})
	})

	t.Run("checks no owner when workspaces cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("workspaces")

		assertUnfinished(tc, jobs.CheckWorkspaceOwnersArgs{})
	})

	t.Run("reports a workspace whose owner cannot be replaced", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, admin := join(t, tc, enums.RoleAdmin)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, tc.Workspace.Owner))
		tc.FailWrites("workspace_members", "UPDATE")

		assertUnfinished(tc, jobs.CheckWorkspaceOwnersArgs{})

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": admin.ID, "role": "admin"})
	})

	t.Run("purges no expired sign-in record while one kind cannot be deleted", func(t *testing.T) {
		t.Parallel()

		for _, table := range []string{"user_auth_factors", "one_time_tokens", "password_reset_tokens", "sessions"} {
			t.Run(table, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				tc.BreakTable(table)

				assertUnfinished(tc, jobs.PurgeExpiredAuthRecordsArgs{})
			})
		}
	})
}

func TestNotifyExpiringTokenFailures(t *testing.T) {
	t.Parallel()

	t.Run("warns nobody when the warning cannot be recorded", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		in(tc, tc.Owner).PostJSON(patPath, testkit.JSON{
			"name": "Soon", "abilities": []string{"*"}, "expires_at": tests.Epoch().Add(48 * time.Hour),
		}).AssertCreated()
		tc.Drain()
		tc.FailWrites("personal_access_tokens", "UPDATE")

		job := &jobs.NotifyExpiringTokens{DB: tc.App.DB, Clock: tc.Clock, Mail: tc.App.Mail, Links: tc.App.Links}

		require.Error(t, job.Handle(t.Context(), nil))
	})

	t.Run("warns owners in their own language", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		in(tc, tc.Owner).PostJSON(patPath, testkit.JSON{
			"name": "Soon", "abilities": []string{"*"}, "expires_at": tests.Epoch().Add(48 * time.Hour),
		}).AssertCreated()
		tc.Drain()
		exec(t, tc, `UPDATE users SET locale = 'en-GB' WHERE id = $1`, tc.Owner.ID)

		job := &jobs.NotifyExpiringTokens{DB: tc.App.DB, Clock: tc.Clock, Mail: tc.App.Mail, Links: tc.App.Links}

		require.NoError(t, job.Handle(t.Context(), nil))
		tc.Table("personal_access_tokens").AssertCount(0, testkit.Row{"name": "Soon", "expiry_notified_at": nil})
	})
}
