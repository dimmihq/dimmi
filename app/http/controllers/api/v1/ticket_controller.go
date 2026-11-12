/*
|--------------------------------------------------------------------------
| Ticket Controller
|--------------------------------------------------------------------------
|
| Tickets: a filtered page, a new ticket, one ticket by id or reference,
| a change guarded by its version, the trash, turning a conversation
| into a ticket, and escalating to back-office work. Every rule lives
| in the tickets action, so the API obeys what the inbox obeys.
|
*/

package v1

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"
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
TicketController serves tickets.
*/
type TicketController struct {
	Tickets *tickets.Tickets
	IDs     *ids.Registry
}

/*
TicketsOutput is a page of tickets.
*/
type TicketsOutput struct {
	Body fhttp.Collection[resources.Ticket]
}

/*
TicketOutput is one ticket.
*/
type TicketOutput struct {
	Body fhttp.Resource[resources.Ticket]
}

/*
TicketStoreOutput is a ticket made, 201, or found again by its external
id, 200.
*/
type TicketStoreOutput struct {
	Status int
	Body   fhttp.Resource[resources.Ticket]
}

/*
errDateInvalid refuses a time that is not RFC 3339.
*/
var errDateInvalid = fault.New(fault.Invalid, "date_invalid", "Times are RFC 3339, such as 2026-11-12T09:00:00Z.")

/*
Index lists a page of tickets.
*/
func (c *TicketController) Index(ctx context.Context, in *requests.TicketsRequest) (*TicketsOutput, error) {
	f, err := c.filter(in)
	if err != nil {
		return nil, err
	}

	page, err := c.Tickets.List(ctx, membership(ctx), f)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.Ticket]{Items: resources.NewTickets(page.Tickets), Limit: f.Limit}
	if page.Next != nil {
		out.NextCursor = models.PublicID(models.PrefixTicket, *page.Next)
	}

	return &TicketsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
filter reads a page request.
*/
func (c *TicketController) filter(in *requests.TicketsRequest) (tickets.Filter, error) {
	f := tickets.Filter{Kinds: list(in.Kind), Groups: list(in.Group), Trashed: in.Trashed, Limit: in.Limit}
	if f.Limit == 0 {
		f.Limit = 25
	}

	if in.Active && len(f.Groups) == 0 {
		f.Groups = []string{"submitted", "in_progress", "waiting_customer", "waiting_team"}
	}

	var err error
	if f.TypeIDs, err = c.idList(models.PrefixTicketType, in.TypeID); err != nil {
		return f, err
	}

	if f.StateIDs, err = c.idList(models.PrefixTicketState, in.StateID); err != nil {
		return f, err
	}

	if err := c.people(in, &f); err != nil {
		return f, err
	}

	if err := c.ranges(in, &f); err != nil {
		return f, err
	}

	err = c.extras(in, &f)

	return f, err
}

/*
people reads the teammate, team, company, customer and conversation
filters.
*/
func (c *TicketController) people(in *requests.TicketsRequest, f *tickets.Filter) error {
	if in.AssigneeID == "none" {
		f.Unassigned = true
	} else if err := c.optional(models.PrefixMember, in.AssigneeID, &f.Assignee); err != nil {
		return err
	}

	if err := c.optional(models.PrefixTeam, in.TeamID, &f.Team); err != nil {
		return err
	}

	if err := c.optional(models.PrefixCompany, in.CompanyID, &f.Company); err != nil {
		return err
	}

	if err := c.optional(models.PrefixContact, in.ContactID, &f.Contact); err != nil {
		return err
	}

	if err := c.optional(models.PrefixConversation, in.ConversationID, &f.Conversation); err != nil {
		return err
	}

	return c.optional(models.PrefixTicket, in.Cursor, &f.After)
}

/*
ranges reads the created and resolved ranges.
*/
func (c *TicketController) ranges(in *requests.TicketsRequest, f *tickets.Filter) error {
	for _, r := range []struct {
		text string
		dst  **time.Time
	}{
		{in.CreatedAfter, &f.CreatedFrom},
		{in.CreatedBefore, &f.CreatedTo},
		{in.ResolvedAfter, &f.ResolvedFrom},
		{in.ResolvedBefore, &f.ResolvedTo},
	} {
		if r.text == "" {
			continue
		}

		t, err := time.Parse(time.RFC3339, r.text)
		if err != nil {
			return errDateInvalid
		}

		*r.dst = &t
	}

	return nil
}

/*
extras reads the number, external id, priority, issue status, text and
field filters.
*/
func (c *TicketController) extras(in *requests.TicketsRequest, f *tickets.Filter) error {
	if in.Number != "" {
		n, ok := tickets.ParseNumber(in.Number)
		if !ok {
			return exceptions.ErrNotFound.OnField("number")
		}

		f.Number = &n
	}

	f.ExternalID, f.Priority = textOrNil(in.ExternalID), textOrNil(in.Priority)
	f.IssueGroup, f.Query = textOrNil(in.IssueStatus), textOrNil(in.Q)

	if in.Fields != "" {
		if err := json.Unmarshal([]byte(in.Fields), &f.Fields); err != nil {
			return tickets.ErrFieldInvalid.OnField("fields")
		}
	}

	return nil
}

/*
list splits a comma-separated parameter.
*/
func list(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}

	return out
}

/*
idList decodes a comma-separated list of public ids.
*/
func (c *TicketController) idList(prefix models.Prefix, s string) ([]ids.UUID, error) {
	var out []ids.UUID
	for _, p := range list(s) {
		id, err := c.IDs.Decode(string(prefix), p)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}

		out = append(out, id)
	}

	return out, nil
}

/*
optional decodes a public id into dst when one is given.
*/
func (c *TicketController) optional(prefix models.Prefix, s string, dst **ids.UUID) error {
	if s == "" {
		return nil
	}

	id, err := c.IDs.Decode(string(prefix), s)
	if err != nil {
		return exceptions.ErrNotFound
	}

	*dst = &id

	return nil
}

/*
optionalPtr decodes an optional public id from a body.
*/
func (c *TicketController) optionalPtr(prefix models.Prefix, s *string) (*ids.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil //nolint:nilnil // no id sent is no id
	}

	var out *ids.UUID

	err := c.optional(prefix, *s, &out)

	return out, err
}

/*
ref reads a ticket's id or reference.
*/
func (c *TicketController) ref(s string) (tickets.Ref, error) {
	if n, ok := tickets.ParseNumber(s); ok {
		return tickets.Ref{Number: n}, nil
	}

	id, err := c.IDs.Decode(string(models.PrefixTicket), s)
	if err != nil {
		return tickets.Ref{}, exceptions.ErrNotFound
	}

	return tickets.Ref{ID: id}, nil
}

/*
Store makes a ticket.
*/
func (c *TicketController) Store(ctx context.Context, in *requests.StoreTicketRequest) (*TicketStoreOutput, error) {
	input, err := c.createInput(in)
	if err != nil {
		return nil, err
	}

	made, err := c.Tickets.Create(ctx, membership(ctx), input)
	if err != nil {
		return nil, err
	}

	return c.stored(ctx, made)
}

/*
stored answers a create with the ticket, 201 when new.
*/
func (c *TicketController) stored(ctx context.Context, made tickets.Created) (*TicketStoreOutput, error) {
	d, err := c.Tickets.Show(ctx, membership(ctx), tickets.Ref{ID: made.Ticket.ID})
	if err != nil {
		return nil, err
	}

	status := fhttp.StatusOK
	if made.New {
		status = fhttp.StatusCreated
	}

	return &TicketStoreOutput{Status: status, Body: fhttp.NewResource(resources.NewTicketDetail(d))}, nil
}

/*
createInput reads a new ticket.
*/
func (c *TicketController) createInput(in *requests.StoreTicketRequest) (tickets.CreateInput, error) {
	b := in.Body
	out := tickets.CreateInput{
		Title: b.Title, Description: b.Description, Fields: b.Fields, ExternalID: b.ExternalID,
	}

	if b.Priority != nil {
		out.Priority = *b.Priority
	}

	typeID, err := c.IDs.Decode(string(models.PrefixTicketType), b.TypeID)
	if err != nil {
		return out, tickets.ErrTypeNotFound.OnField("type_id")
	}

	out.TypeID = typeID

	for _, d := range []struct {
		prefix models.Prefix
		value  *string
		dst    **ids.UUID
	}{
		{models.PrefixTicketState, b.StateID, &out.StateID},
		{models.PrefixContact, b.ContactID, &out.ContactID},
		{models.PrefixCompany, b.CompanyID, &out.CompanyID},
		{models.PrefixConversation, b.ConversationID, &out.ConversationID},
		{models.PrefixMember, b.AssigneeID, &out.AssigneeMember},
		{models.PrefixTeam, b.TeamID, &out.AssigneeTeam},
		{models.PrefixInbox, b.InboxID, &out.InboxID},
		{models.PrefixSLAPolicy, b.SLAPolicyID, &out.SLAPolicyID},
	} {
		if *d.dst, err = c.optionalPtr(d.prefix, d.value); err != nil {
			return out, err
		}
	}

	for _, l := range b.LinkTo {
		link, err := c.linkInput(l)
		if err != nil {
			return out, err
		}

		out.Links = append(out.Links, link)
	}

	return out, nil
}

/*
linkInput reads a link.
*/
func (c *TicketController) linkInput(b requests.TicketLinkBody) (tickets.LinkInput, error) {
	out := tickets.LinkInput{Kind: b.Kind}

	var err error
	if out.TicketID, err = c.optionalPtr(models.PrefixTicket, b.TicketID); err != nil {
		return out, err
	}

	out.ConversationID, err = c.optionalPtr(models.PrefixConversation, b.ConversationID)

	return out, err
}

/*
Show answers one ticket, by id or reference.
*/
func (c *TicketController) Show(ctx context.Context, in *requests.TicketRequest) (*TicketOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	d, err := c.Tickets.Show(ctx, membership(ctx), ref)
	if err != nil {
		return nil, err
	}

	return &TicketOutput{Body: fhttp.NewResource(resources.NewTicketDetail(d))}, nil
}

/*
Update changes a ticket.
*/
func (c *TicketController) Update(ctx context.Context, in *requests.UpdateTicketRequest) (*TicketOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	change, err := c.change(in)
	if err != nil {
		return nil, err
	}

	t, err := c.Tickets.Update(ctx, membership(ctx), ref, change)
	if err != nil {
		return nil, err
	}

	return c.showTicket(ctx, t.ID)
}

/*
showTicket answers a ticket after a change.
*/
func (c *TicketController) showTicket(ctx context.Context, id ids.UUID) (*TicketOutput, error) {
	d, err := c.Tickets.Show(ctx, membership(ctx), tickets.Ref{ID: id})
	if err != nil {
		return nil, err
	}

	return &TicketOutput{Body: fhttp.NewResource(resources.NewTicketDetail(d))}, nil
}

/*
change reads a change, its version from If-Match or the body.
*/
func (c *TicketController) change(in *requests.UpdateTicketRequest) (tickets.Change, error) {
	b, sent := in.Body, requests.Sent(in.RawBody)
	out := tickets.Change{
		Version: b.Version, Title: b.Title, Description: b.Description, Fields: b.Fields, Priority: b.Priority,
		Accept: b.AcceptSuggestions, ClearAssignee: sent["assignee_id"] && b.AssigneeID == nil,
		ClearTeam: sent["team_id"] && b.TeamID == nil,
	}

	if tag := strings.Trim(strings.TrimPrefix(in.IfMatch, "W/"), `"`); tag != "" {
		v, err := strconv.ParseInt(tag, 10, 32)
		if err != nil {
			return out, tickets.ErrVersionClash
		}

		out.Version = new(int32(v))
	}

	var err error
	for _, d := range []struct {
		prefix models.Prefix
		value  *string
		dst    **ids.UUID
	}{
		{models.PrefixTicketState, b.StateID, &out.StateID},
		{models.PrefixTicketType, b.TypeID, &out.TypeID},
		{models.PrefixMember, b.AssigneeID, &out.Assignee},
		{models.PrefixTeam, b.TeamID, &out.Team},
		{models.PrefixSLAPolicy, b.SLAPolicyID, &out.SLAPolicyID},
	} {
		if *d.dst, err = c.optionalPtr(d.prefix, d.value); err != nil {
			return out, err
		}
	}

	return out, nil
}

/*
Destroy moves a ticket to the trash.
*/
func (c *TicketController) Destroy(ctx context.Context, in *requests.TicketRequest) (*struct{}, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	_, err = c.Tickets.Trash(ctx, membership(ctx), ref)

	return &struct{}{}, err
}

/*
Restore takes a ticket out of the trash.
*/
func (c *TicketController) Restore(ctx context.Context, in *requests.TicketRequest) (*TicketOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	t, err := c.Tickets.Restore(ctx, membership(ctx), ref)
	if err != nil {
		return nil, err
	}

	return c.showTicket(ctx, t.ID)
}

/*
Convert makes a conversation a customer ticket.
*/
func (c *TicketController) Convert(
	ctx context.Context, in *requests.ConvertConversationRequest,
) (*TicketStoreOutput, error) {
	conv, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	typeID, err := c.IDs.Decode(string(models.PrefixTicketType), in.Body.TypeID)
	if err != nil {
		return nil, tickets.ErrTypeNotFound.OnField("type_id")
	}

	input := tickets.CreateInput{TypeID: typeID, Title: in.Body.Title, Fields: in.Body.Fields, AIFill: in.Body.AIFill}
	if input.StateID, err = c.optionalPtr(models.PrefixTicketState, in.Body.StateID); err != nil {
		return nil, err
	}

	made, err := c.Tickets.Convert(ctx, membership(ctx), conversations.Ref{ID: conv}, input)
	if err != nil {
		return nil, err
	}

	return c.stored(ctx, made)
}

/*
Escalate makes back-office work for a customer ticket.
*/
func (c *TicketController) Escalate(
	ctx context.Context, in *requests.StoreTicketEscalationRequest,
) (*TicketOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	typeID, err := c.IDs.Decode(string(models.PrefixTicketType), in.Body.TypeID)
	if err != nil {
		return nil, tickets.ErrTypeNotFound.OnField("type_id")
	}

	input := tickets.EscalateInput{
		TypeID: typeID, Title: in.Body.Title, Description: in.Body.Description, Fields: in.Body.Fields,
	}
	if input.TeamID, err = c.optionalPtr(models.PrefixTeam, in.Body.TeamID); err != nil {
		return nil, err
	}

	if input.Assignee, err = c.optionalPtr(models.PrefixMember, in.Body.AssigneeID); err != nil {
		return nil, err
	}

	made, err := c.Tickets.Escalate(ctx, membership(ctx), ref, input)
	if err != nil {
		return nil, err
	}

	return c.showTicket(ctx, made.ID)
}
