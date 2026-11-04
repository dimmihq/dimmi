/*
|--------------------------------------------------------------------------
| Assignment
|--------------------------------------------------------------------------
|
| Automatic assignment follows what happens to conversations and
| members. A conversation a customer starts, one handed to a team
| without a member, one reopened, and one an away member handed back
| look for an assignee; anything that frees someone up, such as a close,
| a snooze, a reassignment or someone coming back, drains the waiting
| ones. A conversation that gets a member, closes, or leaves the inbox
| stops waiting in the same transaction.
|
*/

package providers

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/assignment"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
bootAssignment builds automatic assignment, its listeners and its jobs.
*/
func bootAssignment(a *app.App) error {
	arrival := &assignment.AssignOnArrival{DB: a.DB, Events: a.Events, Clock: a.Clock}
	assigner := &assignment.Assigner{Arrival: arrival, Drain: &assignment.DrainAssignmentQueue{Arrival: arrival}}
	a.Actions.Assignment = assigner
	a.Actions.AssignmentStatus = &assignment.ShowAssignmentStatus{DB: a.DB}

	listenArrivals(a.Events)
	listenDrains(a.Events)
	listenWaitEnds(a.Events, a)

	return errors.Join(
		queue.Register(a.Queue, &jobs.AssignConversation{Assigner: assigner}),
		queue.Register(a.Queue, &jobs.DrainAssignmentQueue{Assigner: assigner}),
		queue.Register(a.Queue, &jobs.SweepAssignmentQueues{Assigner: assigner, Queue: a.Queue}),
	)
}

/*
assign returns the job that assigns the conversation for the reason.
*/
func assign(e appevents.Base, conversation ids.UUID, reason string) queue.Args {
	return jobs.AssignConversationArgs{WorkspaceID: e.WorkspaceID, ConversationID: conversation, Reason: reason}
}

/*
listenArrivals queues an assignment for each conversation that starts
looking for an assignee.
*/
func listenArrivals(d *events.Dispatcher) {
	events.ListenQueued(d, func(e appevents.ConversationCreated) queue.Args {
		if e.InitiatedBy != string(enums.AuthorTypeContact) {
			return nil
		}

		return assign(e.Base, e.ConversationID, assignment.ReasonArrival)
	})
	events.ListenQueued(d, func(e appevents.ConversationReopened) queue.Args {
		return assign(e.Base, e.ConversationID, assignment.ReasonReopened)
	})
	events.ListenQueued(d, func(e appevents.ConversationAssigned) queue.Args {
		if e.ToMemberID != nil {
			return nil
		}

		if e.Reason == conversations.ReasonAwayReassign {
			return assign(e.Base, e.ConversationID, conversations.ReasonAwayReassign)
		}

		if e.Reason == "" && e.ToTeamID != nil && !sameTeam(e.FromTeamID, e.ToTeamID) {
			return assign(e.Base, e.ConversationID, assignment.ReasonTeamAssigned)
		}

		return nil
	})
}

/*
sameTeam reports whether two team ids name the same team or both none.
*/
func sameTeam(a, b *ids.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

/*
drain returns the job that drains the workspace.
*/
func drain(workspaceID ids.UUID) queue.Args {
	return jobs.DrainAssignmentQueueArgs{WorkspaceID: workspaceID}
}

/*
listenDrains queues a drain after anything that may free someone up.
*/
func listenDrains(d *events.Dispatcher) {
	events.ListenQueued(d, func(e appevents.ConversationClosed) queue.Args { return drain(e.WorkspaceID) })
	events.ListenQueued(d, func(e appevents.ConversationSnoozed) queue.Args { return drain(e.WorkspaceID) })
	events.ListenQueued(d, func(e appevents.ConversationMarkedSpam) queue.Args { return drain(e.WorkspaceID) })
	events.ListenQueued(d, func(e appevents.ConversationDeleted) queue.Args { return drain(e.WorkspaceID) })
	events.ListenQueued(d, func(e appevents.ConversationAssigned) queue.Args {
		if e.FromMemberID == nil || (e.ToMemberID != nil && *e.ToMemberID == *e.FromMemberID) {
			return nil
		}

		return drain(e.WorkspaceID)
	})
	events.ListenQueued(d, func(e appevents.MemberAvailabilityChanged) queue.Args {
		return drain(e.WorkspaceID)
	})
}

/*
listenWaitEnds ends a conversation's wait in the transaction that gave
it a member, closed it, marked it as spam or deleted it.
*/
func listenWaitEnds(d *events.Dispatcher, a *app.App) {
	end := func(ctx context.Context, workspaceID, conversation ids.UUID) error {
		return models.New(a.DB).ClearConversationAssignmentWait(ctx, models.ClearConversationAssignmentWaitParams{
			WorkspaceID: workspaceID, ID: conversation,
		})
	}

	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationAssigned) error {
		if e.ToMemberID == nil {
			return nil
		}

		return end(ctx, e.WorkspaceID, e.ConversationID)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationClosed) error {
		return end(ctx, e.WorkspaceID, e.ConversationID)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationMarkedSpam) error {
		return end(ctx, e.WorkspaceID, e.ConversationID)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationDeleted) error {
		return end(ctx, e.WorkspaceID, e.ConversationID)
	})
}
