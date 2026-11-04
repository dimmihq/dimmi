/*
|--------------------------------------------------------------------------
| Show Assignment Status
|--------------------------------------------------------------------------
|
| An admin asking why a conversation waits gets the answer member by
| member: who is in the inbox's pool, how much each holds against their
| limits, and why each may not take the next one. The waiting
| conversations are summed up by reason. Nothing is written; the answer is
| worked out the way a pick would work it out at this moment.
|
*/

package assignment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
StatusInput is who asks about which inbox.
*/
type StatusInput struct {
	Membership authz.Membership
	InboxID    ids.UUID
}

/*
Waiting sums up the inbox's waiting conversations: how many, since when
the oldest waits, and how many wait for each reason.
*/
type Waiting struct {
	Count    int
	OldestAt *time.Time
	Reasons  map[string]int
}

/*
Status is the inbox's assignment as it stands: its mode, the pool's
members with whether each may take a conversation, and what waits. The
members are judged for conversations of ChannelType.
*/
type Status struct {
	Mode        string
	ChannelType enums.ChannelType
	Members     []Candidate
	Waiting     Waiting
}

/*
ShowAssignmentStatus explains an inbox's automatic assignment.

Rules:
  - The caller needs settings:read.
  - Members are judged as for a conversation of the inbox's widget
    channel, the channel most inboxes take conversations through.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowAssignmentStatus struct {
	DB *database.DB
}

/*
Handle returns the inbox's status.
*/
func (a *ShowAssignmentStatus) Handle(ctx context.Context, in StatusInput) (Status, error) {
	if !in.Membership.Can("settings:read") {
		return Status{}, exceptions.ErrForbidden
	}

	var out Status

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		ws := in.Membership.Workspace.ID

		inbox, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: ws, ID: in.InboxID})
		if errors.Is(err, database.ErrNotFound) || (err == nil && inbox.DeletedAt != nil) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find inbox: %w", err)
		}

		settings := inbox.ReadSettings().Assignment
		out = Status{Mode: settings.Mode, ChannelType: enums.ChannelTypeWidget}

		out.Members, err = Candidates(ctx, q, PickInput{
			WorkspaceID: ws, InboxID: inbox.ID, TeamID: inbox.DefaultTeamID, Settings: settings,
			ChannelType: out.ChannelType,
		})
		if err != nil {
			return err
		}

		out.Waiting, err = waiting(ctx, q, ws, inbox.ID)

		return err
	})

	return out, err
}

/*
waiting sums up the inbox's waiting conversations.
*/
func waiting(ctx context.Context, q *models.Queries, workspaceID, inboxID ids.UUID) (Waiting, error) {
	rows, err := q.CountWaitingConversations(ctx, models.CountWaitingConversationsParams{
		WorkspaceID: workspaceID, InboxID: inboxID,
	})
	if err != nil {
		return Waiting{}, fmt.Errorf("count waiting conversations: %w", err)
	}

	out := Waiting{Reasons: map[string]int{}}

	for _, row := range rows {
		out.Count += int(row.Waiting)
		out.Reasons[row.Reason] = int(row.Waiting)

		if out.OldestAt == nil || row.Oldest.Before(*out.OldestAt) {
			oldest := row.Oldest
			out.OldestAt = &oldest
		}
	}

	return out, nil
}
