/*
|--------------------------------------------------------------------------
| Working Links
|--------------------------------------------------------------------------
|
| A teammate links a ticket to another ticket or a conversation, takes a
| link away, and lists a tracker's conversations with the customer,
| company, plan and revenue of each, newest link first.
|
*/

package tickets

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Links returns a ticket's links.
*/
func (t *Tickets) Links(ctx context.Context, m authz.Membership, ref Ref) ([]LinkView, error) {
	if err := canRead(m); err != nil {
		return nil, err
	}

	var out []LinkView

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		out, err = linkViews(ctx, q, found, t.Clock.Now())

		return err
	})

	return out, err
}

/*
AddLink links a ticket the member may see; the target conversation
must be one they may see too.
*/
func (t *Tickets) AddLink(ctx context.Context, m authz.Membership, ref Ref, in LinkInput) (models.TicketLink, error) {
	if err := canWrite(m); err != nil {
		return models.TicketLink{}, err
	}

	var out models.TicketLink

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		found, err := find(ctx, w.Q, m, ref)
		if err != nil {
			return err
		}

		if in.ConversationID != nil {
			if _, err := conversations.Find(ctx, w.Q, m, conversations.Ref{ID: *in.ConversationID}); err != nil {
				return ErrLinkInvalid.OnField("conversation_id")
			}
		}

		out, err = w.Link(ctx, found, in)
		if err == nil && out.ID == ids.Nil() {
			out, err = existingLink(ctx, w.Q, found, in)
		}

		return err
	})

	return out, err
}

/*
existingLink finds the link a repeated link asked for.
*/
func existingLink(ctx context.Context, q *models.Queries, t models.Ticket, in LinkInput) (models.TicketLink, error) {
	links, err := q.ListTicketLinks(ctx, models.ListTicketLinksParams{WorkspaceID: t.WorkspaceID, TicketID: t.ID})
	if err != nil {
		return models.TicketLink{}, fmt.Errorf("list ticket links: %w", err)
	}

	for _, l := range links {
		if l.Kind == in.Kind && l.FromTicketID == t.ID && samePtr(l.ToTicketID, in.TicketID) &&
			samePtr(l.ConversationID, in.ConversationID) {
			return l, nil
		}
	}

	return models.TicketLink{}, ErrLinkNotFound
}

/*
RemoveLink takes a link away from a ticket the member may see.
*/
func (t *Tickets) RemoveLink(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if err := canWrite(m); err != nil {
		return err
	}

	return t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		l, err := w.Q.FindTicketLink(ctx, models.FindTicketLinkParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil {
			return ErrLinkNotFound
		}

		if _, err := find(ctx, w.Q, m, Ref{ID: l.FromTicketID}); err != nil {
			return ErrLinkNotFound
		}

		return w.Unlink(ctx, l)
	})
}

/*
LinkedConversation is one conversation a tracker reaches.
*/
type LinkedConversation = models.ListTrackerConversationsRow

/*
LinkedPage is a page of a tracker's conversations, who it reaches in
all, and the next cursor.
*/
type LinkedPage struct {
	Conversations []LinkedConversation
	Impact        Impact
	Next          *ids.UUID
}

/*
Conversations returns a page of a tracker's linked conversations.
*/
func (t *Tickets) Conversations(
	ctx context.Context, m authz.Membership, ref Ref, after *ids.UUID,
) (LinkedPage, error) {
	if err := canRead(m); err != nil {
		return LinkedPage{}, err
	}

	var out LinkedPage

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		out, err = linkedPage(ctx, q, found, after)

		return err
	})

	return out, err
}

/*
linkedPage reads one page of linked conversations after the cursor.
*/
func linkedPage(ctx context.Context, q *models.Queries, t models.Ticket, after *ids.UUID) (LinkedPage, error) {
	p := models.ListTrackerConversationsParams{WorkspaceID: t.WorkspaceID, TicketID: t.ID, RowLimit: maxPage + 1}

	if after != nil {
		l, err := q.FindTicketLink(ctx, models.FindTicketLinkParams{WorkspaceID: t.WorkspaceID, ID: *after})
		if err != nil {
			return LinkedPage{}, ErrLinkNotFound.OnField("after")
		}

		p.AfterCreated, p.AfterID = &l.CreatedAt, &l.ID
	}

	rows, err := q.ListTrackerConversations(ctx, p)
	if err != nil {
		return LinkedPage{}, fmt.Errorf("list tracker conversations: %w", err)
	}

	out := LinkedPage{Conversations: rows}
	if len(rows) > maxPage {
		out.Conversations = rows[:maxPage]
		out.Next = &out.Conversations[maxPage-1].LinkID
	}

	out.Impact, err = impactOf(ctx, q, t)

	return out, err
}

/*
ConversationTickets returns the customer ticket of a conversation the
member may see, and the trackers it is linked to.
*/
func (t *Tickets) ConversationTickets(
	ctx context.Context, m authz.Membership, ref conversations.Ref,
) (*Detail, []Summary, error) {
	if err := canRead(m); err != nil {
		return nil, nil, err
	}

	var (
		own    *Detail
		linked []Summary
	)

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := conversations.Find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		own, linked, err = conversationTickets(ctx, q, c, t.Clock.Now())

		return err
	})

	return own, linked, err
}

/*
conversationTickets loads a conversation's own ticket and the trackers
it joined.
*/
func conversationTickets(
	ctx context.Context, q *models.Queries, c models.Conversation, now time.Time,
) (*Detail, []Summary, error) {
	var own *Detail

	ticket, err := q.FindConversationTicket(ctx, models.FindConversationTicketParams{
		WorkspaceID: c.WorkspaceID, ConversationID: &c.ID,
	})
	if err == nil && ticket.DeletedAt == nil {
		d, err := detailOf(ctx, q, ticket, now)
		if err != nil {
			return nil, nil, err
		}

		own = &d
	} else if err != nil && !isNoRows(err) {
		return nil, nil, fmt.Errorf("find conversation ticket: %w", err)
	}

	links, err := q.ListConversationLinks(ctx, models.ListConversationLinksParams{
		WorkspaceID: c.WorkspaceID, ConversationID: &c.ID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list conversation links: %w", err)
	}

	trackerIDs := make([]ids.UUID, 0, len(links))
	for _, l := range links {
		trackerIDs = append(trackerIDs, l.FromTicketID)
	}

	rows, err := q.ListTicketsByIDs(ctx, models.ListTicketsByIDsParams{WorkspaceID: c.WorkspaceID, Ids: trackerIDs})
	if err != nil {
		return nil, nil, fmt.Errorf("list trackers: %w", err)
	}

	live := rows[:0]
	for _, r := range rows {
		if r.DeletedAt == nil {
			live = append(live, r)
		}
	}

	linked, err := summarize(ctx, q, live, now)

	return own, linked, err
}
