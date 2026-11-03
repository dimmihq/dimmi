/*
|--------------------------------------------------------------------------
| Conversation Counts
|--------------------------------------------------------------------------
|
| The sidebar says how many conversations wait in each view: the
| caller's own, those nobody holds, everyone's, spam and each inbox's.
| Counts stop at a cap, so a busy workspace costs no more to show than a
| quiet one; the caller's own and the unassigned also say how many are
| unread for them.
|
*/

package conversations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CountsInput is who asks.
*/
type CountsInput struct {
	Membership authz.Membership
}

/*
Counts is how many conversations wait in each view of the sidebar, each
capped at CountCap: the caller's open, unread and snoozed ones, the
open and unread ones nobody holds, every open one, spam, and the open
ones of each inbox.
*/
type Counts struct {
	MineOpen, MineUnread, MineSnoozed int64
	UnassignedOpen, UnassignedUnread  int64
	AllOpen, SpamTotal                int64
	Inboxes                           map[ids.UUID]int64
}

/*
CountCap is the largest count shown; anything above shows as the cap.
*/
const CountCap = 999

/*
CountConversations counts the conversations of each view.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type CountConversations struct {
	DB *database.DB
}

/*
Handle returns the counts.
*/
func (a *CountConversations) Handle(ctx context.Context, in CountsInput) (Counts, error) {
	if err := CanRead(in.Membership); err != nil {
		return Counts{}, err
	}

	ws, me := in.Membership.Workspace.ID, in.Membership.Member.ID
	out := Counts{Inboxes: map[ids.UUID]int64{}}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		totals, err := q.CountConversations(ctx, models.CountConversationsParams{WorkspaceID: ws, MemberID: &me})
		if err != nil {
			return fmt.Errorf("count conversations: %w", err)
		}

		unread, err := q.CountUnreadConversations(ctx, models.CountUnreadConversationsParams{
			WorkspaceID: ws, MemberID: me,
		})
		if err != nil {
			return fmt.Errorf("count unread conversations: %w", err)
		}

		inboxes, err := q.CountOpenConversationsByInbox(ctx, ws)
		if err != nil {
			return fmt.Errorf("count conversations by inbox: %w", err)
		}

		out.MineOpen, out.MineSnoozed, out.MineUnread = totals.MineOpen, totals.MineSnoozed, unread.Mine
		out.UnassignedOpen, out.UnassignedUnread = totals.UnassignedOpen, unread.Unassigned
		out.AllOpen, out.SpamTotal = totals.AllOpen, totals.SpamTotal

		for _, row := range inboxes {
			out.Inboxes[row.InboxID] = row.Open
		}

		return nil
	})

	return out, err
}
