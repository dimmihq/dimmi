/*
|--------------------------------------------------------------------------
| Keeping The Bell In Step
|--------------------------------------------------------------------------
|
| Elsewhere in the product, things happen that change the bell: a member
| reads a conversation, so its notifications are read too; someone
| decides an action call, so the request to approve it gives way to how
| it ended. Both run inside the transaction of what happened, and the
| member's tabs hear the new counts once it commits.
|
*/

package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Read is a member reading a conversation in a workspace, at a moment.
*/
type Read struct {
	WorkspaceID    ids.UUID
	MemberID       ids.UUID
	ConversationID ids.UUID
	At             time.Time
}

/*
ConversationRead marks a member's notifications about a conversation
read once they read it, inside the transaction that recorded the read.
*/
func ConversationRead(ctx context.Context, q *models.Queries, r Read) (bool, error) {
	tenant := r.WorkspaceID.String()
	read, err := q.MarkConversationNotificationsRead(ctx, models.MarkConversationNotificationsReadParams{
		Now: &r.At, TenantID: &tenant, NotifiableID: RecipientKey(r.WorkspaceID, r.MemberID),
		ConversationID: r.ConversationID.String(),
	})
	if err != nil {
		return false, fmt.Errorf("mark conversation notifications read: %w", err)
	}
	return len(read) > 0, nil
}

/*
Decision is how an action call ended, in a workspace, at a moment.
*/
type Decision struct {
	WorkspaceID ids.UUID
	CallID      ids.UUID
	State       string
	At          time.Time
}

/*
ResolveApproval marks every notification about the action call read
and records how it ended, inside the transaction that decided it.
*/
func ResolveApproval(ctx context.Context, q *models.Queries, d Decision) error {
	tenant := d.WorkspaceID.String()
	err := q.ResolveApprovalNotifications(ctx, models.ResolveApprovalNotificationsParams{
		Now: &d.At, State: d.State, TenantID: &tenant, ActionCallID: d.CallID.String(),
	})
	if err != nil {
		return fmt.Errorf("resolve approval notifications: %w", err)
	}
	return nil
}

/*
MemberCounts returns a member's bell counts, for a live update.
*/
func MemberCounts(ctx context.Context, q *models.Queries, ws, member ids.UUID) (Counts, error) {
	tenant := ws.String()
	row, err := q.CountMemberNotifications(ctx, models.CountMemberNotificationsParams{
		TenantID: &tenant, NotifiableID: RecipientKey(ws, member),
	})
	if err != nil {
		return Counts{}, fmt.Errorf("count notifications: %w", err)
	}
	return Counts{Unread: row.Unread, Unseen: row.Unseen}, nil
}

/*
BroadcastPublisher publishes to members' channels.
*/
type BroadcastPublisher struct {
	Broadcaster interface {
		BroadcastMany(ctx context.Context, channels []string, event string, data any) error
	}
}

/*
PublishToMember sends the event to the member's channel.
*/
func (p BroadcastPublisher) PublishToMember(ctx context.Context, member ids.UUID, event string, data any) error {
	return p.Broadcaster.BroadcastMany(ctx, []string{appbroadcasting.MemberChannel(member)}, event, data)
}
