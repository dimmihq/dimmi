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
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
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
open and unread ones nobody holds, every open one, spam, those whose
notes mention the caller since they last read them, and the open ones
of each inbox, team, member and pinned saved view.
*/
type Counts struct {
	MineOpen, MineUnread, MineSnoozed int64
	UnassignedOpen, UnassignedUnread  int64
	AllOpen, SpamTotal                int64
	MentionsUnread                    int64
	Inboxes, Teams, Members, Views    map[ids.UUID]int64
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
	DB    *database.DB
	IDs   *ids.Registry
	Clock clock.Clock
	Cache *cache.Store
}

/*
Handle returns the counts.
*/
func (a *CountConversations) Handle(ctx context.Context, in CountsInput) (Counts, error) {
	if err := CanRead(in.Membership); err != nil {
		return Counts{}, err
	}

	ws, me, access := in.Membership.Workspace.ID, in.Membership.Member.ID, VisibleScope(in.Membership).Params()
	out := Counts{Inboxes: map[ids.UUID]int64{}, Teams: map[ids.UUID]int64{}, Members: map[ids.UUID]int64{}}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		totals, err := q.CountConversations(ctx, models.CountConversationsParams{
			WorkspaceID: ws, MemberID: &me, AccessMode: access.Mode, AccessViewer: access.Viewer,
			AccessTeams: access.Teams, AccessUnassigned: access.Unassigned,
		})
		if err != nil {
			return fmt.Errorf("count conversations: %w", err)
		}

		unread, err := q.CountUnreadConversations(ctx, models.CountUnreadConversationsParams{
			WorkspaceID: ws, MemberID: me, AccessMode: access.Mode, AccessViewer: access.Viewer,
			AccessTeams: access.Teams, AccessUnassigned: access.Unassigned,
		})
		if err != nil {
			return fmt.Errorf("count unread conversations: %w", err)
		}

		inboxes, err := q.CountOpenConversationsByInbox(ctx, models.CountOpenConversationsByInboxParams{
			WorkspaceID: ws, AccessMode: access.Mode, AccessViewer: access.Viewer, AccessTeams: access.Teams,
			AccessUnassigned: access.Unassigned,
		})
		if err != nil {
			return fmt.Errorf("count conversations by inbox: %w", err)
		}

		out.MineOpen, out.MineSnoozed, out.MineUnread = totals.MineOpen, totals.MineSnoozed, unread.Mine
		out.UnassignedOpen, out.UnassignedUnread = totals.UnassignedOpen, unread.Unassigned
		out.AllOpen, out.SpamTotal = totals.AllOpen, totals.SpamTotal

		for _, row := range inboxes {
			out.Inboxes[row.InboxID] = row.Open
		}

		out.Views = a.countViews(ctx, tx, in.Membership)

		return errors.Join(countMentions(ctx, q, in.Membership, &out), countHolders(ctx, q, in.Membership, out))
	})

	return out, err
}

/*
countMentions counts the conversations the member may see whose notes
mention them after what they last read.
*/
func countMentions(ctx context.Context, q *models.Queries, m authz.Membership, out *Counts) error {
	access := VisibleScope(m).Params()

	n, err := q.CountUnreadMentions(ctx, models.CountUnreadMentionsParams{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, AccessMode: access.Mode, AccessViewer: access.Viewer,
		AccessTeams: access.Teams, AccessUnassigned: access.Unassigned,
	})
	if err != nil {
		return fmt.Errorf("count unread mentions: %w", err)
	}

	out.MentionsUnread = n

	return nil
}

/*
countHolders counts the open conversations of each team and member,
among those the member may see.
*/
func countHolders(ctx context.Context, q *models.Queries, m authz.Membership, out Counts) error {
	ws, access := m.Workspace.ID, VisibleScope(m).Params()

	teams, err := q.CountOpenConversationsByTeam(ctx, models.CountOpenConversationsByTeamParams{
		WorkspaceID: ws, AccessMode: access.Mode, AccessViewer: access.Viewer, AccessTeams: access.Teams,
		AccessUnassigned: access.Unassigned,
	})
	if err != nil {
		return fmt.Errorf("count conversations by team: %w", err)
	}

	members, err := q.CountOpenConversationsByAssignee(ctx, models.CountOpenConversationsByAssigneeParams{
		WorkspaceID: ws, AccessMode: access.Mode, AccessViewer: access.Viewer, AccessTeams: access.Teams,
		AccessUnassigned: access.Unassigned,
	})
	if err != nil {
		return fmt.Errorf("count conversations by member: %w", err)
	}

	for _, row := range teams {
		out.Teams[row.TeamID] = row.Open
	}

	for _, row := range members {
		out.Members[row.MemberID] = row.Open
	}

	return nil
}
