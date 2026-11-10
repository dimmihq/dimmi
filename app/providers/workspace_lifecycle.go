/*
|--------------------------------------------------------------------------
| Workspace Lifecycle
|--------------------------------------------------------------------------
|
| The actions and jobs that bring people into a workspace and take a
| workspace away: invitations, deletion and restore, the purge that
| follows the trash, and the daily rounds that remind owners and keep
| every workspace owned. The invitation actions are built before the
| sign-in actions, since signing up can answer an invitation.
|
*/

package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootInvitations builds the actions that send, change, show and accept
invitations.
*/
func bootInvitations(a *app.App) {
	acts := a.Actions
	delivery := invitations.Delivery{
		Clock: a.Clock, Mail: a.Mail, Links: a.Links, TTL: a.Config.Invitations.TTL,
	}

	acts.CreateInvitations = &invitations.CreateInvitations{
		DB: a.DB, Events: a.Events, Limiter: a.Limiter, Delivery: delivery,
		DailyCap: a.Config.Invitations.DailyCap(),
	}
	acts.ListInvitations = &invitations.ListInvitations{DB: a.DB, Clock: a.Clock}
	acts.ResendInvitation = &invitations.ResendInvitation{DB: a.DB, Limiter: a.Limiter, Delivery: delivery}
	acts.RevokeInvitation = &invitations.RevokeInvitation{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.PreviewInvitation = &invitations.PreviewInvitation{DB: a.DB, Clock: a.Clock}
	acts.AcceptInvitation = &invitations.AcceptInvitation{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Mail: a.Mail, Links: a.Links,
	}
}

/*
bootWorkspaceLifecycle builds the actions that delete, restore and
repair workspaces.
*/
func bootWorkspaceLifecycle(a *app.App) {
	acts := a.Actions
	window := a.Config.Auth.PasswordConfirmation.TTL

	acts.TrashWorkspace = &workspaces.TrashWorkspace{
		Events: a.Events, Clock: a.Clock, Mail: a.Mail, GraceDays: a.Config.Workspaces.DeletionGraceDays,
	}
	acts.DeleteWorkspace = &workspaces.DeleteWorkspace{
		DB: a.DB, Clock: a.Clock, ConfirmWindow: window, Trash: acts.TrashWorkspace,
	}
	acts.RestoreWorkspace = &workspaces.RestoreWorkspace{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Mail: a.Mail, Links: a.Links, ConfirmWindow: window,
	}
	acts.EnsureWorkspaceHasOwner = &members.EnsureWorkspaceHasOwner{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Mail: a.Mail, Links: a.Links, Trash: acts.TrashWorkspace,
	}
}

/*
registerWorkspaceJobs registers the workers of the workspace jobs,
among them the release of a thawed workspace's held chat deliveries
and the daily prune of kept ones; the console routes schedule the
daily rounds.
*/
func registerWorkspaceJobs(a *app.App) error {
	disk, err := a.Storage.Default()
	if err != nil {
		return fmt.Errorf("workspace disk: %w", err)
	}

	return errors.Join(
		queue.Register(a.Queue, &jobs.PurgeWorkspace{
			DB: a.DB, Clock: a.Clock, Disk: disk, Retention: a.Retention, Log: a.Log,
			Revoke: func(ctx context.Context, workspaceID ids.UUID) error {
				return a.Actions.Integrations.RevokeWorkspace.Handle(ctx, workspaceID)
			},
		}),
		queue.Register(a.Queue, &jobs.ScheduleWorkspacePurges{DB: a.DB, Clock: a.Clock, Queue: a.Queue}),
		queue.Register(a.Queue, &jobs.SendWorkspaceDeletionReminders{DB: a.DB, Clock: a.Clock, Mail: a.Mail}),
		queue.Register(a.Queue, &jobs.CheckWorkspaceOwners{DB: a.DB, Ensure: a.Actions.EnsureWorkspaceHasOwner}),
		queue.Register(a.Queue, &jobs.ReleaseHeldInboundEvents{DB: a.DB, Clock: a.Clock, Queue: a.Queue}),
		queue.Register(a.Queue, &jobs.PruneChannelInboundEvents{
			DB: a.DB, Clock: a.Clock, Retention: a.Retention,
			ProcessedRetentionDays: a.Config.Channels.InboundEvents.ProcessedRetentionDays,
			FailedRetentionDays:    a.Config.Channels.InboundEvents.FailedRetentionDays,
		}),
	)
}
