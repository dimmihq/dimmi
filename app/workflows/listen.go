/*
|--------------------------------------------------------------------------
| Workflow Triggers
|--------------------------------------------------------------------------
|
| Domain events become workflow events here. A customer's first message
| and a customer reopening a conversation are matched inside the
| transaction that wrote them, so a customer-facing workflow holds the
| conversation before the automatic agent could look at it. Everything
| else is matched from a job. A teammate's reply, a close, spam and
| deletion reach running workflows inside their own transaction, before
| anything is sent. Workflow and agent messages never trigger anything;
| the agent handing a conversation over moves a run waiting on it.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
Listen wires the engine to the domain events.
*/
func (e *Engine) Listen(d *events.Dispatcher) {
	events.Listen(d, events.InTx, e.onMessage)
	events.Listen(d, events.InTx, e.onReopened)
	events.Listen(d, events.InTx, func(ctx context.Context, ev appevents.ConversationClosed) error {
		return e.interruptNow(ctx, ev.Base, ev.ConversationID, EndConversationClosed)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, ev appevents.ConversationMarkedSpam) error {
		return e.interruptNow(ctx, ev.Base, ev.ConversationID, EndConversationSpam)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, ev appevents.ConversationDeleted) error {
		return e.interruptNow(ctx, ev.Base, ev.ConversationID, EndConversationGone)
	})
	events.ListenQueued(d, queuedMessage)
	events.ListenQueued(d, resumeOnMessage)
	events.ListenQueued(d, armQuiet)
	events.ListenQueued(d, func(ev appevents.AiHandoffRequested) queue.Args {
		return jobs.ResumeWorkflowOnAgentArgs{
			WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, Reason: ev.Reason, EventID: ev.ID,
		}
	})
	events.ListenQueued(d, func(ev appevents.ConversationAssigned) queue.Args {
		return queuedChange(ev.Base, ev.ConversationID, workflowspec.TriggerAssignmentChanged, assignmentData(ev))
	})
	events.ListenQueued(d, rated)
	events.ListenQueued(d, agentHandedOver)
	e.listenStatus(d)
}

/*
listenStatus matches status changes.
*/
func (e *Engine) listenStatus(d *events.Dispatcher) {
	status := func(b appevents.Base, conv ids.UUID, to string) queue.Args {
		return queuedChange(b, conv, workflowspec.TriggerStatusChanged, map[string]any{"trigger.to": to})
	}

	events.ListenQueued(d, func(ev appevents.ConversationClosed) queue.Args {
		return status(ev.Base, ev.ConversationID, "closed")
	})
	events.ListenQueued(d, func(ev appevents.ConversationSnoozed) queue.Args {
		return status(ev.Base, ev.ConversationID, "snoozed")
	})
	events.ListenQueued(d, func(ev appevents.ConversationUnsnoozed) queue.Args {
		return status(ev.Base, ev.ConversationID, "open")
	})
}

/*
customerMessage reports whether the entry is a real message from the
customer.
*/
func customerMessage(ev appevents.MessageCreated) bool {
	return ev.AuthorType == string(enums.AuthorTypeContact) && ev.Type == string(enums.MessageTypeMessage) &&
		ev.Substantive
}

/*
teammateEntry reports whether a teammate wrote the entry.
*/
func teammateEntry(ev appevents.MessageCreated) bool {
	return ev.AuthorType == string(enums.AuthorTypeMember)
}

/*
onMessage matches a customer's first message in its transaction, and
ends customer-facing runs when a teammate replies publicly.
*/
func (e *Engine) onMessage(ctx context.Context, ev appevents.MessageCreated) error {
	switch {
	case customerMessage(ev) && ev.ConversationCreated:
		return e.Match(ctx, Event{
			WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, Trigger: workflowspec.TriggerFirstMessage,
			EventID: ev.ID.String(), Chain: ev.WorkflowChain, Data: map[string]any{"message_id": ev.MessageID.String()},
		})
	case teammateEntry(ev) && ev.Type == string(enums.MessageTypeMessage):
		return e.interruptNow(ctx, ev.Base, ev.ConversationID, EndTeammateReplied)
	}

	return nil
}

/*
onReopened matches a customer reopening a closed conversation.
*/
func (e *Engine) onReopened(ctx context.Context, ev appevents.ConversationReopened) error {
	if ev.Reason != "customer_message" {
		return nil
	}

	return e.Match(ctx, Event{
		WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, Trigger: workflowspec.TriggerCustomerReopened,
		EventID: ev.ID.String(), Chain: ev.WorkflowChain,
	})
}

/*
interruptNow applies a change to the conversation's runs in the
transaction that made it.
*/
func (e *Engine) interruptNow(ctx context.Context, b appevents.Base, conv ids.UUID, reason string) error {
	return e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		c, err := models.New(tx).FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: b.WorkspaceID, ID: conv,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find conversation: %w", err)
		}

		if reason == EndConversationClosed && c.AiMode == "workflow" {
			if err := models.New(tx).ResetConversationAiMode(ctx, models.ResetConversationAiModeParams{
				WorkspaceID: b.WorkspaceID, ID: conv, Now: e.Clock.Now(),
			}); err != nil {
				return err
			}
		}

		return e.InterruptInTx(ctx, tx, Interruption{Conversation: c, Reason: reason, Chain: b.WorkflowChain})
	})
}

/*
queuedMessage matches messages other than the first: the customer's,
the team's replies and notes.
*/
func queuedMessage(ev appevents.MessageCreated) queue.Args {
	data := map[string]any{"message_id": ev.MessageID.String()}

	switch {
	case customerMessage(ev) && !ev.ConversationCreated:
		return queuedChange(ev.Base, ev.ConversationID, workflowspec.TriggerCustomerMessage, data)
	case teammateEntry(ev) && ev.Type == string(enums.MessageTypeMessage):
		return queuedChange(ev.Base, ev.ConversationID, workflowspec.TriggerTeammateMessage, data)
	case teammateEntry(ev) && ev.Type == string(enums.MessageTypeNote):
		return queuedChange(ev.Base, ev.ConversationID, workflowspec.TriggerNoteCreated, data)
	}

	return nil
}

/*
resumeOnMessage hands a customer message to the runs waiting for one.
*/
func resumeOnMessage(ev appevents.MessageCreated) queue.Args {
	if !customerMessage(ev) || ev.ConversationCreated {
		return nil
	}

	return jobs.ResumeWorkflowInputArgs{
		WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, MessageID: ev.MessageID,
	}
}

/*
queuedChange is the job matching a change, or nothing for changes the
automatic agent made: those are never the team's doing.
*/
func queuedChange(b appevents.Base, conv ids.UUID, trigger string, data map[string]any) queue.Args {
	if b.Actor.Type == string(enums.AuthorTypeAgent) {
		return nil
	}

	raw, err := json.Marshal(Event{
		WorkspaceID: b.WorkspaceID, ConversationID: conv, Trigger: trigger, EventID: b.ID.String(),
		Chain: b.WorkflowChain, Data: data,
	})
	if err != nil {
		return nil
	}

	return jobs.MatchWorkflowsArgs{Event: raw}
}

/*
assignmentData is what an assignment change tells conditions: where it
went, a member, a team or nobody, and who.
*/
func assignmentData(ev appevents.ConversationAssigned) map[string]any {
	to := "unassigned"
	switch {
	case ev.ToMemberID != nil:
		to = "member"
	case ev.ToTeamID != nil:
		to = "team"
	}

	return map[string]any{
		"trigger.to": to, "trigger.to_member": publicID(models.PrefixMember, ev.ToMemberID),
		"trigger.to_team": publicID(models.PrefixTeam, ev.ToTeamID),
	}
}

/*
armQuiet arms the quiet triggers a message may start: a public reply
by a teammate or the agent starts the customer's silence, a customer's
message may start their wait for the team.
*/
func armQuiet(ev appevents.MessageCreated) queue.Args {
	if ev.Type != string(enums.MessageTypeMessage) {
		return nil
	}

	args := jobs.ArmQuietTriggersArgs{
		WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, MessageID: ev.MessageID, Author: ev.AuthorType,
	}
	switch {
	case ev.AuthorType == string(enums.AuthorTypeMember) || ev.AuthorType == string(enums.AuthorTypeAgent):
		args.Trigger = workflowspec.TriggerCustomerQuiet
	case customerMessage(ev):
		args.Trigger = workflowspec.TriggerTeammateQuiet
	default:
		return nil
	}

	return args
}

/*
rated matches a customer's first answer to a rating ask; changing the
score or adding a comment later starts nothing new.
*/
func rated(ev appevents.ConversationRated) queue.Args {
	if !ev.First {
		return nil
	}

	return queuedChange(ev.Base, ev.ConversationID, workflowspec.TriggerRated, map[string]any{"rating.score": ev.Score})
}

/*
agentHandedOver matches the agent handing a conversation to the team.
The agent made this change, so it is matched even though changes the
agent makes start nothing else.
*/
func agentHandedOver(ev appevents.AiHandoffRequested) queue.Args {
	raw, err := json.Marshal(Event{
		WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, Trigger: workflowspec.TriggerDimmiHandedOver,
		EventID: ev.ID.String(), Chain: ev.WorkflowChain, Data: map[string]any{"dimmi.handover_reason": ev.Reason},
	})
	if err != nil {
		return nil
	}

	return jobs.MatchWorkflowsArgs{Event: raw}
}
