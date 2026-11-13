/*
|--------------------------------------------------------------------------
| Ticket Activity Controller
|--------------------------------------------------------------------------
|
| What happens around a ticket: its history and the workspace's log,
| internal notes, links to tickets and conversations, a tracker's
| conversations and messages to all of them, a conversation's own
| ticket and trackers, and the AI's field values.
|
*/

package v1

import (
	"context"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketActivityController serves what happens around tickets.
*/
type TicketActivityController struct {
	TicketController
}

/*
TicketEventsOutput is a page of history.
*/
type TicketEventsOutput struct {
	Body fhttp.Collection[resources.TicketEvent]
}

/*
Events lists a ticket's history.
*/
func (c *TicketActivityController) Events(ctx context.Context, in *requests.TicketEventsRequest,
) (*TicketEventsOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	var after *ids.UUID
	if err := c.optional(models.PrefixTicketEvent, in.Cursor, &after); err != nil {
		return nil, err
	}

	page, err := c.Tickets.History(ctx, membership(ctx), ref, after)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.TicketEvent]{Items: resources.NewTicketEvents(page.Events), Limit: 100}
	if page.Next != nil {
		out.NextCursor = models.PublicID(models.PrefixTicketEvent, *page.Next)
	}

	return &TicketEventsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
Export lists the workspace's ticket log after the cursor.
*/
func (c *TicketActivityController) Export(ctx context.Context, in *requests.WorkspaceTicketEventsRequest,
) (*TicketEventsOutput, error) {
	var after *ids.UUID
	if err := c.optional(models.PrefixTicketEvent, in.Cursor, &after); err != nil {
		return nil, err
	}

	var since *time.Time
	if in.Since != "" {
		t, err := time.Parse(time.RFC3339, in.Since)
		if err != nil {
			return nil, errDateInvalid.OnField("since")
		}

		since = &t
	}

	page, err := c.Tickets.Export(ctx, membership(ctx), after, since)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.TicketEvent]{Items: resources.NewExportEvents(page.Events), Limit: 100}
	if page.Next != nil {
		out.NextCursor = models.PublicID(models.PrefixTicketEvent, *page.Next)
	}

	return &TicketEventsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
TicketEventOutput is one history entry.
*/
type TicketEventOutput struct {
	Body fhttp.Resource[resources.TicketEvent]
}

/*
Note leaves an internal note.
*/
func (c *TicketActivityController) Note(ctx context.Context, in *requests.StoreTicketNoteRequest,
) (*TicketEventOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	e, err := c.Tickets.Note(ctx, membership(ctx), ref, in.Body.Body)
	if err != nil {
		return nil, err
	}

	return &TicketEventOutput{Body: fhttp.NewResource(resources.NewTicketEvent(e))}, nil
}

/*
TicketLinksOutput is a ticket's links.
*/
type TicketLinksOutput struct {
	Body fhttp.Resource[[]resources.TicketLink]
}

/*
TicketLinkOutput is one link.
*/
type TicketLinkOutput struct {
	Body fhttp.Resource[resources.TicketLink]
}

/*
Links lists a ticket's links.
*/
func (c *TicketActivityController) Links(ctx context.Context, in *requests.TicketRequest) (*TicketLinksOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	links, err := c.Tickets.Links(ctx, membership(ctx), ref)
	if err != nil {
		return nil, err
	}

	return &TicketLinksOutput{Body: fhttp.NewResource(resources.NewTicketLinks(links))}, nil
}

/*
Link links a ticket to another ticket or a conversation.
*/
func (c *TicketActivityController) Link(ctx context.Context, in *requests.StoreTicketLinkRequest,
) (*TicketLinkOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	link, err := c.linkInput(in.Body)
	if err != nil {
		return nil, err
	}

	made, err := c.Tickets.AddLink(ctx, membership(ctx), ref, link)
	if err != nil {
		return nil, err
	}

	return &TicketLinkOutput{Body: fhttp.NewResource(resources.NewTicketLink(tickets.LinkView{Link: made}))}, nil
}

/*
Unlink removes a link.
*/
func (c *TicketActivityController) Unlink(ctx context.Context, in *requests.TicketLinkRequest) (*struct{}, error) {
	id, err := c.IDs.Decode(string(models.PrefixTicketLink), in.Link)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return &struct{}{}, c.Tickets.RemoveLink(ctx, membership(ctx), id)
}

/*
Conversations lists a tracker's conversations.
*/
func (c *TicketActivityController) Conversations(ctx context.Context, in *requests.TicketConversationsRequest,
) (*TrackerConversationsOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	var after *ids.UUID
	if err := c.optional(models.PrefixTicketLink, in.Cursor, &after); err != nil {
		return nil, err
	}

	page, err := c.Tickets.Conversations(ctx, membership(ctx), ref, after)
	if err != nil {
		return nil, err
	}

	out := &TrackerConversationsOutput{}
	out.Body.Data = resources.NewTrackerConversations(page.Conversations)
	out.Body.Meta.Impact = resources.TicketImpact{
		Conversations: page.Impact.Conversations, Customers: page.Impact.Customers,
		Companies: page.Impact.Companies, Revenue: page.Impact.Revenue,
	}

	if page.Next != nil {
		out.Body.Meta.NextCursor = new(models.PublicID(models.PrefixTicketLink, *page.Next))
	}

	return out, nil
}

/*
SendUpdate messages everyone a tracker links, or counts them.
*/
func (c *TicketActivityController) SendUpdate(ctx context.Context, in *requests.StoreTicketUpdateRequest,
) (*TicketUpdatePlanOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	plan, err := c.Tickets.SendToLinked(ctx, membership(ctx), ref, tickets.UpdateInput{
		Body: in.Body.Body, Resolve: in.Body.Resolve, DryRun: in.Body.DryRun,
	})
	if err != nil {
		return nil, err
	}

	out := &TicketUpdatePlanOutput{}
	out.Body.Data.Recipients = len(plan.Recipients)
	out.Body.Data.Held = []HeldRecipient{}

	for _, r := range plan.Recipients {
		if r.Held != "" {
			out.Body.Data.Held = append(out.Body.Data.Held, HeldRecipient{
				ConversationID: models.PublicID(models.PrefixConversation, r.ConversationID), Reason: r.Held,
			})
		}
	}

	if plan.Update != nil {
		out.Body.Data.Update = new(resources.NewTicketUpdate(tickets.UpdateReport{Update: *plan.Update}))
	}

	return out, nil
}

/*
Updates lists a tracker's messages.
*/
func (c *TicketActivityController) Updates(ctx context.Context, in *requests.TicketRequest,
) (*TicketUpdatesOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	list, err := c.Tickets.Updates(ctx, membership(ctx), ref)
	if err != nil {
		return nil, err
	}

	return &TicketUpdatesOutput{Body: fhttp.NewResource(resources.NewTicketUpdates(list))}, nil
}

/*
ConversationTickets answers a conversation's tickets, for its panel.
*/
func (c *TicketActivityController) ConversationTickets(ctx context.Context, in *requests.ConversationTicketsRequest,
) (*ConversationTicketsOutput, error) {
	conv, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	own, linked, err := c.Tickets.ConversationTickets(ctx, membership(ctx), conversations.Ref{ID: conv})
	if err != nil {
		return nil, err
	}

	out := &ConversationTicketsOutput{}
	out.Body.Data.Trackers = resources.NewTickets(linked)

	if own != nil {
		out.Body.Data.Ticket = new(resources.NewTicketDetail(*own))
	}

	return out, nil
}

/*
Suggestions answers the AI's values for a type's fields from a
conversation.
*/
func (c *TicketActivityController) Suggestions(ctx context.Context, in *requests.TicketSuggestionsRequest,
) (*TicketSuggestionsOutput, error) {
	conv, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	typeID, err := c.IDs.Decode(string(models.PrefixTicketType), in.TypeID)
	if err != nil {
		return nil, tickets.ErrTypeNotFound.OnField("type_id")
	}

	values, err := c.Tickets.Suggest(ctx, membership(ctx), conversations.Ref{ID: conv}, typeID)
	if err != nil {
		return nil, err
	}

	return &TicketSuggestionsOutput{Body: fhttp.NewResource(values)}, nil
}
