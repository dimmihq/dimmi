/*
|--------------------------------------------------------------------------
| Widget Ticket Controller
|--------------------------------------------------------------------------
|
| A visitor's requests: the types they may open, the form that opens
| one, their requests and their company's, and one request with the
| states it went through. A request is a conversation with a ticket on
| it: the form starts the conversation the way a first message does,
| then makes the ticket, so replies go through the conversation's own
| message routes. A form sent twice with the same client id makes one
| request. Company requests show only to a visitor whose identity is
| verified.
|
*/

package widget

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/useragent"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketController serves a visitor's requests.
*/
type TicketController struct {
	Tickets *tickets.Tickets
	Post    *actions.PostWidgetMessage
	IDs     *ids.Registry
	Clock   clock.Clock
	Agents  *useragent.Parser
}

/*
TicketTypesOutput lists the types a visitor may open.
*/
type TicketTypesOutput struct {
	Body fhttp.Resource[[]resources.WidgetTicketType]
}

/*
TicketsOutput lists a visitor's requests.
*/
type TicketsOutput struct {
	Body fhttp.Resource[[]resources.WidgetTicket]
}

/*
TicketOutput is one request: 201 when the form made it, 200 when
the browser sent the same form before or reads it.
*/
type TicketOutput struct {
	Status int
	Body   fhttp.Resource[resources.WidgetTicket]
}

/*
Types lists the types the messenger offers under "Get help with".
*/
func (c *TicketController) Types(ctx context.Context, _ *requests.NoInput) (*TicketTypesOutput, error) {
	resolved, _ := actions.CurrentResolved(ctx)

	types, err := c.Tickets.CustomerTypes(ctx, resolved.Workspace.ID)
	if err != nil {
		return nil, err
	}

	out := &TicketTypesOutput{Body: fhttp.NewResource([]resources.WidgetTicketType{})}
	for _, t := range types {
		out.Body.Data = append(out.Body.Data, resources.NewWidgetTicketType(t))
	}

	return out, nil
}

/*
Index lists the visitor's requests, or their company's when asked for
and allowed, newest first.
*/
func (c *TicketController) Index(
	ctx context.Context, in *requests.IndexWidgetTicketsRequest,
) (*TicketsOutput, error) {
	who, err := c.customer(ctx)
	if err != nil {
		return nil, err
	}

	scope := tickets.ScopeMine
	if in.Scope == tickets.ScopeCompany {
		scope = tickets.ScopeCompany
	}

	list, err := c.Tickets.CustomerTickets(ctx, who, scope, in.Open)
	if err != nil {
		return nil, err
	}

	out := &TicketsOutput{Body: fhttp.NewResource([]resources.WidgetTicket{})}
	for _, t := range list {
		if scope == tickets.ScopeCompany && !t.Type.CompanySharing {
			continue
		}

		out.Body.Data = append(out.Body.Data, resources.NewWidgetTicket(t, who.ContactID))
	}

	return out, nil
}

/*
Show answers with one of the visitor's requests and its history.
*/
func (c *TicketController) Show(
	ctx context.Context, in *requests.ShowWidgetTicketRequest,
) (*TicketOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	who, err := c.customer(ctx)
	if err != nil {
		return nil, err
	}

	found, err := c.Tickets.CustomerShow(ctx, who, ref)
	if err != nil {
		return nil, err
	}

	return &TicketOutput{
		Status: fhttp.StatusOK, Body: fhttp.NewResource(resources.NewWidgetTicket(found, who.ContactID)),
	}, nil
}

/*
Store takes the visitor's form: it checks it, starts the request's
conversation and makes its ticket.
*/
func (c *TicketController) Store(
	ctx context.Context, in *requests.StoreWidgetTicketRequest,
) (*TicketOutput, error) {
	who, err := c.customer(ctx)
	if err != nil {
		return nil, err
	}

	form, err := c.form(who, in)
	if err != nil {
		return nil, err
	}

	if err := c.Tickets.CustomerCheck(ctx, who, form); err != nil {
		return nil, err
	}

	posted, err := c.start(ctx, in, form)
	if err != nil {
		return nil, err
	}

	form.ConversationID = posted.Conversation.ID

	made, err := c.Tickets.CustomerCreate(ctx, who, form)
	if err != nil {
		return nil, err
	}

	shown, err := c.Tickets.CustomerShow(ctx, who, tickets.Ref{ID: made.Ticket.ID})
	if err != nil {
		return nil, err
	}

	out := &TicketOutput{Status: fhttp.StatusCreated, Body: fhttp.NewResource(resources.NewWidgetTicket(shown,
		who.ContactID))}
	if !made.New {
		out.Status = fhttp.StatusOK
	}

	return out, nil
}

/*
form reads the visitor's form as the ticket's input, keyed by the
client id within the visitor, so a retry finds the first request.
*/
func (c *TicketController) form(
	who tickets.Customer, in *requests.StoreWidgetTicketRequest,
) (tickets.CustomerInput, error) {
	typeID, err := c.IDs.Decode(string(models.PrefixTicketType), in.Body.TypeID)
	if err != nil {
		return tickets.CustomerInput{}, tickets.ErrTypeNotFound.OnField("type_id")
	}

	key := "widget:" + models.PublicID(models.PrefixContact, who.ContactID) + ":" + in.Body.ClientID

	return tickets.CustomerInput{
		TypeID: typeID, Title: strings.TrimSpace(in.Body.Title), Description: strings.TrimSpace(in.Body.Description),
		Fields: in.Body.Fields, ExternalID: &key,
	}, nil
}

/*
start opens the request's conversation with the form's words as the
visitor's first message, the uploads of its file fields attached.
*/
func (c *TicketController) start(
	ctx context.Context, in *requests.StoreWidgetTicketRequest, form tickets.CustomerInput,
) (actions.PostedMessage, error) {
	body := form.Title
	if form.Description != "" {
		body += "\n\n" + form.Description
	}

	msg := actions.MessageInput{Auth: authenticated(ctx), ClientID: in.Body.ClientID, Body: body}

	if in.Body.Page != nil {
		page := in.Body.Page.Page()
		msg.Page = &page
	}

	if r, ok := fhttp.RequestFrom(ctx); ok && r != nil {
		agent := c.Agents.Parse(r.UserAgent())
		msg.Browser, msg.OS = agent.Browser, agent.OS
	}

	for _, public := range uploadsOf(in) {
		id, err := c.IDs.Decode(string(models.PrefixAttachment), public)
		if err != nil {
			return actions.PostedMessage{}, exceptions.ErrAttachmentNotUploaded
		}

		msg.AttachmentIDs = append(msg.AttachmentIDs, id)
	}

	return c.Post.Handle(ctx, msg)
}

/*
uploadsOf lists the attachments the form names, in its file fields and
beside them, each once.
*/
func uploadsOf(in *requests.StoreWidgetTicketRequest) []string {
	seen := map[string]bool{}
	out := []string{}

	add := func(id string) {
		if strings.HasPrefix(id, string(models.PrefixAttachment)+"_") && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}

	for _, id := range in.Body.AttachmentIDs {
		add(id)
	}

	for _, v := range in.Body.Fields {
		if list, ok := v.([]any); ok {
			for _, item := range list {
				s, _ := item.(string)
				add(s)
			}
		}
	}

	return out
}

/*
customer returns the visitor as a customer: verified only while their
session proves who they are.
*/
func (c *TicketController) customer(ctx context.Context) (tickets.Customer, error) {
	auth := authenticated(ctx)
	s := auth.Session

	verified := s.Kind == string(enums.WidgetSessionIdentified) && !s.Test && s.VerifiedAt != nil &&
		c.Clock.Now().Before(s.ExpiresAt)

	return c.Tickets.CustomerFor(ctx, s.WorkspaceID, s.ContactID, verified)
}

/*
ref reads a request's id or reference; anything else is not found.
*/
func (c *TicketController) ref(s string) (tickets.Ref, error) {
	if n, ok := tickets.ParseNumber(s); ok {
		return tickets.Ref{Number: n}, nil
	}

	id, err := c.IDs.Decode(string(models.PrefixTicket), s)
	if err != nil {
		return tickets.Ref{}, tickets.ErrTicketNotFound
	}

	return tickets.Ref{ID: id}, nil
}
