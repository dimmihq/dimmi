package workspaces_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
legalHold is a retention guard that holds every workspace, as a legal
hold would.
*/
type legalHold struct{}

/*
MayDestroy refuses.
*/
func (legalHold) MayDestroy(context.Context, ids.UUID, contracts.RetentionSubject) (bool, error) {
	return false, nil
}

/*
push queues the job and runs every job that is ready.
*/
func push(tc *tests.TestCase, args queue.Args) {
	tc.T.Helper()

	require.NoError(tc.T, tc.App.Queue.Push(tc.T.Context(), args))
	tc.Drain()
}

/*
overdue trashes the test case's workspace and moves its purge date into
the past.
*/
func overdue(t *testing.T, tc *tests.TestCase) {
	t.Helper()

	trash(tc).AssertOK()
	exec(t, tc, `UPDATE workspaces SET purge_after = $1 WHERE id = $2`,
		tests.Epoch().Add(-time.Minute), tc.Workspace.Workspace.ID)
}

/*
storeFile writes a file into the workspace's folder of the default disk
and returns its path.
*/
func storeFile(tc *tests.TestCase, workspaceID ids.UUID) (filesystem.Filesystem, string) {
	tc.T.Helper()

	disk, err := tc.App.Storage.Default()
	require.NoError(tc.T, err)

	path := jobs.WorkspaceStoragePrefix(workspaceID) + "attachments/report.txt"
	require.NoError(tc.T, disk.Put(tc.T.Context(), path, []byte("quarterly"), filesystem.PutOptions{}))

	return disk, path
}

func TestPurgeWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("deletes an overdue workspace, its rows and files, and retires its organization", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws := tc.Workspace.Workspace
		join(t, tc, enums.RoleMember)
		invite(tc, "grace@example.com").AssertOK()
		disk, path := storeFile(tc, ws.ID)
		overdue(t, tc)

		push(tc, jobs.ScheduleWorkspacePurgesArgs{})

		tc.Table("workspaces").AssertCount(0, testkit.Row{"id": ws.ID})
		tc.Table("workspace_members").AssertCount(0, testkit.Row{"workspace_id": ws.ID})
		tc.Table("invitations").AssertCount(0, testkit.Row{"workspace_id": ws.ID})
		tc.Table("organizations").AssertCount(0, testkit.Row{"id": ws.OrganizationID, "deleted_at": nil})
		tc.Table("organizations").AssertCount(1, testkit.Row{"id": ws.OrganizationID})
		tc.Table("river_job").AssertCount(1, testkit.Row{"kind": "purge_workspace", "state": "completed"})

		exists, err := disk.Exists(t.Context(), path)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("leaves a workspace the retention guard holds in the trash, untouched", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Retention = legalHold{} }))
		ws := tc.Workspace.Workspace
		disk, path := storeFile(tc, ws.ID)
		overdue(t, tc)

		push(tc, jobs.PurgeWorkspaceArgs{WorkspaceID: ws.ID})

		tc.Table("workspaces").AssertHas(testkit.Row{"id": ws.ID})
		tc.Table("workspace_members").AssertCount(1, testkit.Row{"workspace_id": ws.ID})
		exists, err := disk.Exists(t.Context(), path)
		require.NoError(t, err)
		assert.True(t, exists)

		in(tc, tc.Owner).PatchJSON("/api/v1/workspaces/"+workspaceID(tc), testkit.JSON{"trashed": false}).
			AssertStatus(410)
	})

	t.Run("leaves live workspaces and ones still in their grace period alone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws := tc.Workspace.Workspace

		push(tc, jobs.PurgeWorkspaceArgs{WorkspaceID: ws.ID})
		trash(tc).AssertOK()
		push(tc, jobs.ScheduleWorkspacePurgesArgs{})
		push(tc, jobs.PurgeWorkspaceArgs{WorkspaceID: ws.ID})
		push(tc, jobs.PurgeWorkspaceArgs{WorkspaceID: ids.New()})

		tc.Table("workspaces").AssertHas(testkit.Row{"id": ws.ID})
		in(tc, tc.Owner).PatchJSON("/api/v1/workspaces/"+workspaceID(tc), testkit.JSON{"trashed": false}).AssertOK()
	})

	t.Run("carries on where a stopped purge left off", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws := tc.Workspace.Workspace
		overdue(t, tc)
		exec(t, tc, `UPDATE workspaces SET settings = settings || '{"purge_started_at": "2026-11-01T00:00:00Z"}'`)
		exec(t, tc, `DELETE FROM inboxes WHERE workspace_id = $1`, ws.ID)

		push(tc, jobs.PurgeWorkspaceArgs{WorkspaceID: ws.ID})

		tc.Table("workspaces").AssertCount(0, testkit.Row{"id": ws.ID})
	})
}

func TestWorkspaceDeletionReminders(t *testing.T) {
	t.Parallel()

	t.Run("reminds the owners once, a few days before the purge", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		join(t, tc, enums.RoleOwner)
		trash(tc).AssertOK()

		push(tc, jobs.SendWorkspaceDeletionRemindersArgs{})
		reminders, _ := tests.Sent[appmail.WorkspaceDeletionReminderMail](tc, fake)
		assert.Empty(t, reminders)

		exec(t, tc, `UPDATE workspaces SET purge_after = $1`, tests.Epoch().Add(2*24*time.Hour))
		push(tc, jobs.SendWorkspaceDeletionRemindersArgs{})
		push(tc, jobs.SendWorkspaceDeletionRemindersArgs{})

		reminders, sent := tests.Sent[appmail.WorkspaceDeletionReminderMail](tc, fake)
		require.Len(t, reminders, 2)
		assert.Equal(t, tc.Workspace.Workspace.Name, reminders[0].Workspace)
		assert.Contains(t, sent[0].Message.Subject, "deleted for good")
	})
}

func TestCheckWorkspaceOwners(t *testing.T) {
	t.Parallel()

	t.Run("promotes the longest-standing admin of a workspace left without an owner", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		changed := record[appevents.MemberRoleChanged](tc)
		join(t, tc, enums.RoleMember)
		_, admin := join(t, tc, enums.RoleAdmin)
		join(t, tc, enums.RoleAdmin)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, tc.Workspace.Owner))

		push(tc, jobs.CheckWorkspaceOwnersArgs{})

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": admin.ID, "role": "owner"})
		tc.Table("workspace_members").AssertCount(1, testkit.Row{"role": "owner", "status": "active"})
		require.Len(t, changed.all(), 1)
		assert.Equal(t, ids.Nil(), changed.all()[0].ChangedBy)

		mail, _ := tests.LastSent[appmail.OwnerPromotedBySystemMail](tc, fake)
		assert.Equal(t, tc.Workspace.Workspace.Name, mail.Workspace)
	})

	t.Run("promotes a member when no admin is left", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, tc.Workspace.Owner))

		push(tc, jobs.CheckWorkspaceOwnersArgs{})

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": member.ID, "role": "owner"})
	})

	t.Run("moves a workspace nobody is active in to the trash", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		deleted := record[appevents.WorkspaceDeleted](tc)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, tc.Workspace.Owner))

		push(tc, jobs.CheckWorkspaceOwnersArgs{})

		tc.Table("workspaces").AssertCount(0, testkit.Row{"id": tc.Workspace.Workspace.ID, "deleted_at": nil})
		require.Len(t, deleted.all(), 1)
		assert.Equal(t, ids.Nil(), deleted.all()[0].DeletedBy)
	})

	t.Run("leaves owned workspaces as they are", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		changed := record[appevents.MemberRoleChanged](tc)
		join(t, tc, enums.RoleAdmin)

		push(tc, jobs.CheckWorkspaceOwnersArgs{})

		assert.Empty(t, changed.all())
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": tc.Workspace.Owner.ID, "role": "owner"})
	})
}
