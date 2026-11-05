/*
|--------------------------------------------------------------------------
| Assign On Arrival
|--------------------------------------------------------------------------
|
| A conversation nobody holds gets an assignee from its inbox's rule as
| soon as it arrives, is handed to a team, opens again, or leaves an away
| member. Everything is checked again under the conversation's lock, so a
| conversation someone took in the meantime, one marked as spam, closed,
| or still with the AI agent, is left alone.
|
| When nobody can take it, the conversation keeps its team and waits,
| saying why on its timeline once per wait. Whoever frees up drains the
| waiting ones in the inbox's queue order.
|
*/

package assignment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The reasons a conversation is assigned on its own.
*/
const (
	ReasonArrival      = "arrival"
	ReasonTeamAssigned = "team_assigned"
	ReasonReopened     = "reopened"
	ReasonDrain        = "queue"
)

/*
ArrivalInput names the conversation and why it looks for an assignee.
*/
type ArrivalInput struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	Reason         string
}

/*
AssignOnArrival gives a conversation nobody holds an assignee by its
inbox's rule, or makes it wait.

Rules:
  - Only a live, open conversation nobody holds, not spam, of a contact
    not blocked and not with the AI agent, is assigned.
  - A manual inbox assigns nobody and ends any wait.
  - The pool is the conversation's team, else the inbox's default team,
    else every active member; the team stays as it is.

Events: [appevents.ConversationAssigned] when someone was picked, and
[appevents.ConversationUpdated] when the conversation starts waiting.

Errors: a wrapped database error.
*/
type AssignOnArrival struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle runs the assignment and returns what the pick decided; an empty
outcome means the conversation was not one to assign.
*/
func (a *AssignOnArrival) Handle(ctx context.Context, in ArrivalInput) (PickResult, error) {
	var out PickResult

	scoped := tenancy.With(ctx, in.WorkspaceID.String())
	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		c, err := models.New(tx).LockConversation(ctx, models.LockConversationParams{
			WorkspaceID: in.WorkspaceID, ID: in.ConversationID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("lock conversation: %w", err)
		}

		out, err = a.assign(ctx, a.writer(tx), c, in.Reason)

		return err
	})

	return out, err
}

/*
writer returns a writer acting as the system.
*/
func (a *AssignOnArrival) writer(tx *database.Tx) *conversations.Writer {
	return &conversations.Writer{
		Q: models.New(tx), Tx: tx, Events: a.Events, Now: a.Clock.Now(),
		Actor: conversations.Actor{Type: "system", ID: ids.Nil()},
	}
}

/*
assign picks for the locked conversation and assigns it or makes it
wait.
*/
func (a *AssignOnArrival) assign(
	ctx context.Context, w *conversations.Writer, c models.Conversation, reason string,
) (PickResult, error) {
	ok, err := assignable(ctx, w.Q, c)
	if err != nil {
		return PickResult{}, err
	}

	if !ok {
		return PickResult{}, clearWait(ctx, w.Q, c)
	}

	in, manual, err := pickInput(ctx, w.Q, c, w.Now)
	if err != nil {
		return PickResult{}, err
	}

	if manual {
		return PickResult{}, clearWait(ctx, w.Q, c)
	}

	res, err := PickAssignee(ctx, w.Q, in)
	if err != nil {
		return res, err
	}

	if res.MemberID == nil {
		return res, wait(ctx, w, c, res.Outcome)
	}

	if err := clearWait(ctx, w.Q, c); err != nil {
		return res, err
	}

	_, _, err = w.AssignFor(ctx, c, conversations.Assignment{Member: res.MemberID, Team: c.AssigneeTeamID}, reason)

	return res, err
}

/*
assignable reports whether the conversation is one to assign: live,
open, held by nobody, not spam, not with the AI agent, and of a contact
not blocked.
*/
func assignable(ctx context.Context, q *models.Queries, c models.Conversation) (bool, error) {
	if c.AssigneeMemberID != nil || c.Spam || c.DeletedAt != nil ||
		c.Status != string(enums.ConversationStatusOpen) || c.AiState == "active" {
		return false, nil
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: c.WorkspaceID, ID: c.ContactID})
	if err != nil {
		return false, fmt.Errorf("find contact: %w", err)
	}

	return contact.BlockedAt == nil, nil
}

/*
pickInput reads the inbox's rule and the conversation's channel, and
reports a manual inbox.
*/
func pickInput(ctx context.Context, q *models.Queries, c models.Conversation, now time.Time) (PickInput, bool, error) {
	inbox, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: c.WorkspaceID, ID: c.InboxID})
	if err != nil {
		return PickInput{}, false, fmt.Errorf("find inbox: %w", err)
	}

	settings := inbox.ReadSettings().Assignment
	if settings.Mode == models.AssignmentManual {
		return PickInput{}, true, nil
	}

	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: c.WorkspaceID, ID: c.ChannelConnectionID,
	})
	if err != nil {
		return PickInput{}, false, fmt.Errorf("find channel connection: %w", err)
	}

	team := c.AssigneeTeamID
	if team == nil {
		team = inbox.DefaultTeamID
	}

	return PickInput{
		WorkspaceID: c.WorkspaceID, InboxID: c.InboxID, TeamID: team, Settings: settings,
		ChannelType: enums.ChannelType(conn.Type), Now: now,
	}, false, nil
}

/*
wait makes the conversation wait for the reason, saying so on its
timeline the first time.
*/
func wait(ctx context.Context, w *conversations.Writer, c models.Conversation, reason string) error {
	updated, err := w.Q.SetConversationAssignmentWait(ctx, models.SetConversationAssignmentWaitParams{
		Reason: &reason, Now: &w.Now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	})
	if err != nil {
		return fmt.Errorf("make conversation wait: %w", err)
	}

	if c.AssignmentWaitReason != nil && *c.AssignmentWaitReason == reason {
		return nil
	}

	if c.AssignmentWaitReason == nil {
		if err := w.Record(ctx, updated, "conversation.assignment_waiting", waitData(c, reason)); err != nil {
			return err
		}
	}

	return w.Dispatch(ctx, appevents.ConversationUpdated{
		Base: w.Base(c.WorkspaceID), ConversationRef: appevents.ConversationRef{ConversationID: c.ID},
		Changes: map[string][2]any{"assignment_wait": {c.AssignmentWaitReason, reason}},
	})
}

/*
waitData is what the timeline says about a wait: why, and which team.
*/
func waitData(c models.Conversation, reason string) map[string]any {
	data := map[string]any{"reason": reason, "team": nil}
	if c.AssigneeTeamID != nil {
		data["team"] = map[string]any{"id": models.PublicID(models.PrefixTeam, *c.AssigneeTeamID)}
	}

	return data
}

/*
clearWait ends the conversation's wait, when it waits.
*/
func clearWait(ctx context.Context, q *models.Queries, c models.Conversation) error {
	if c.AssignmentWaitReason == nil {
		return nil
	}

	err := q.ClearConversationAssignmentWait(ctx, models.ClearConversationAssignmentWaitParams{
		WorkspaceID: c.WorkspaceID, ID: c.ID,
	})
	if err != nil {
		return fmt.Errorf("end conversation wait: %w", err)
	}

	return nil
}
