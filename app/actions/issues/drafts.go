/*
|--------------------------------------------------------------------------
| Closing The Loop
|--------------------------------------------------------------------------
|
| A linked issue that is done or canceled moves its ticket to the
| close-the-loop state, as Plain does; nothing writes to customers on
| its own. What customers are told comes from a person, or from the
| workflows the ticket.close_loop trigger runs. The note an engineer
| writes in a comment starting with @customers is drafted for a person
| to send to everyone linked. When sent, customers who wrote since or
| whom a teammate already told are held with the reason, as for any
| message to everyone.
|
*/

package issues

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The reasons a draft was made and the states it moves through.
*/
const (
	ReasonComment   = "comment"
	DraftReady      = "ready"
	DraftSent       = "sent"
	DraftDiscarded  = "discarded"
	customersPrefix = "@customers"
)

/*
draftInput is a draft to make: the link it comes from, nil for none,
why, its text and who wrote it.
*/
type draftInput struct {
	link   *models.TicketIssue
	reason string
	body   string
	author string
}

/*
draft stores a draft for a person to send, once per reason while one
waits, except an engineer's notes, each its own.
*/
func (s *Issues) draft(ctx context.Context, w *tickets.Writer, t models.Ticket, in draftInput) error {
	if in.reason != ReasonComment {
		_, err := w.Q.OpenDraftOfTicket(ctx, models.OpenDraftOfTicketParams{
			WorkspaceID: t.WorkspaceID, TicketID: t.ID, Reason: in.reason,
		})
		if err == nil {
			return nil
		}
	}

	var issueID *ids.UUID
	if in.link != nil {
		issueID = &in.link.ID
	}

	_, err := w.Q.CreateTicketUpdateDraft(ctx, models.CreateTicketUpdateDraftParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID, IssueID: issueID, Source: s.source(in), Reason: in.reason,
		Body: clip(in.body, 20000), AuthorName: clip(in.author, 200), Status: DraftReady, Now: w.Now,
	})
	if err != nil {
		return fmt.Errorf("store draft: %w", err)
	}

	return nil
}

/*
source names where a draft came from.
*/
func (*Issues) source(in draftInput) string {
	if in.link != nil {
		return in.link.Provider
	}

	return "dimmi"
}

/*
sending is a draft to send on its ticket, by the member.
*/
type sending struct {
	ticket models.Ticket
	draft  models.TicketUpdateDraft
	member *ids.UUID
}

/*
sendDraft sends a draft to everyone linked, as the member when one
sends it, and records the message it became.
*/
func (*Issues) sendDraft(ctx context.Context, w *tickets.Writer, in sending) (models.TicketUpdateDraft, error) {
	d, member := in.draft, in.member

	plan, err := w.PlanMessage(ctx, in.ticket, tickets.UpdateInput{Body: d.Body, Resolve: d.Resolve})
	if err != nil {
		return d, err
	}

	var updateID *ids.UUID
	if plan.Update != nil {
		updateID = &plan.Update.ID
	}

	now := w.Now

	return w.Q.DecideTicketUpdateDraft(ctx, models.DecideTicketUpdateDraftParams{
		Status: DraftSent, Body: d.Body, UpdateID: updateID, DecidedByMemberID: member, DecidedAt: &now, Now: now,
		WorkspaceID: d.WorkspaceID, ID: d.ID,
	})
}

/*
commentOn turns a tool's comment into an internal note on the link's
ticket, and a comment starting with @customers into a draft to
everyone linked.
*/
func (s *Issues) commentOn(ctx context.Context, ws, linkID ids.UUID, e trackers.Event) error {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return nil
	}

	return s.asTool(ctx, ws, linkID, func(ctx context.Context, w *tickets.Writer, link models.TicketIssue) error {
		t, err := w.Lock(ctx, ws, link.TicketID)
		if err != nil || t.DeletedAt != nil {
			return nil //nolint:nilerr // a ticket gone takes no notes
		}

		who := first(e.Author, "Someone")
		note := clip(fmt.Sprintf("%s commented on %s %s: %s", who, Label(link.Provider), link.Key, body), 20000)

		if _, err := w.Note(ctx, t, note); err != nil {
			return err
		}

		rest, ok := cutCustomers(body)
		if !ok || rest == "" || t.Kind != tickets.KindTracker {
			return nil
		}

		return s.draft(ctx, w, t, draftInput{link: &link, reason: ReasonComment, body: rest, author: e.Author})
	})
}

/*
cutCustomers returns what follows @customers at the start of a
comment.
*/
func cutCustomers(body string) (string, bool) {
	if len(body) < len(customersPrefix) || !strings.EqualFold(body[:len(customersPrefix)], customersPrefix) {
		return "", false
	}

	return strings.TrimSpace(strings.TrimLeft(body[len(customersPrefix):], ":")), true
}
