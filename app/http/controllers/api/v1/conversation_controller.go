/*
|--------------------------------------------------------------------------
| Conversation Controller
|--------------------------------------------------------------------------
|
| GET /conversations lists a view of the workspace's conversations a
| page at a time, GET /conversations/{conversation} shows one by id or
| number, POST /conversations starts one, PATCH changes several of its
| fields at once, and GET .../neighbors finds the ones around it in a
| list. Ids of other workspaces' conversations are simply not found.
|
*/

package v1

import (
	"context"
	"slices"
	"strconv"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmiddleware "github.com/dimmihq/dimmi/app/http/middleware"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationsOutput is a page of conversations.
*/
type ConversationsOutput struct {
	Body fhttp.Collection[resources.Conversation]
}

/*
ConversationOutput is one conversation.
*/
type ConversationOutput struct {
	Body fhttp.Resource[resources.Conversation]
}

/*
ConversationChangeOutput is a changed conversation and the fields that
changed, in the order they were applied.
*/
type ConversationChangeOutput struct {
	Body struct {
		Data resources.Conversation `json:"data"`
		Meta struct {
			Changed []string `json:"changed"`
		} `json:"meta"`
	}
}

/*
NeighborsOutput is the conversations around one in a list.
*/
type NeighborsOutput struct {
	Body fhttp.Resource[resources.ConversationNeighbors]
}

/*
ConversationController serves the conversations of the workspace in
play.
*/
type ConversationController struct {
	List      *conversations.ListConversations
	Show      *conversations.ShowConversation
	Create    *conversations.CreateConversation
	Apply     *conversations.ApplyConversationChanges
	Neighbors *conversations.ShowNeighbors
	Trash     *conversations.TrashConversation
	Restore   *conversations.RestoreConversation
	Viewers   *appbroadcasting.Viewers
	IDs       *ids.Registry
	Cursors   *pagination.Cursors
}

/*
Index lists a page of a view.
*/
func (c *ConversationController) Index(ctx context.Context, _ *requests.NoInput) (*ConversationsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), ConversationListing())
	if err != nil {
		return nil, err
	}

	lq, err := parseListQuery(c.IDs, query)
	if err != nil {
		return nil, err
	}

	in := conversations.ListConversationsInput{
		Membership: membership(ctx), Filters: lq.filters, Sort: lq.sort, DefaultSort: lq.defaultSort,
		Limit: query.Limit,
	}

	start, err := c.cursor(query.Cursor, lq.hash)
	if err != nil {
		return nil, err
	}

	if start != nil {
		in.After = &start.Keyset
	}

	page, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	if start != nil && start.ViewHash != page.ViewHash {
		return nil, exceptions.ErrCursorMismatch
	}

	out := pagination.Page[resources.Conversation]{
		Items: resources.NewConversations(page.Conversations), Limit: in.Limit,
	}
	if slices.Contains(query.Includes, "viewers") {
		c.includeViewers(ctx, out.Items, page.Conversations)
	}

	if page.More && page.Last != nil {
		cursor := conversationCursor{Keyset: *page.Last, Hash: lq.hash, ViewHash: page.ViewHash}
		if out.NextCursor, err = c.Cursors.Encode(cursor); err != nil {
			return nil, err
		}
	}

	return &ConversationsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
Get answers with one conversation.
*/
func (c *ConversationController) Get(
	ctx context.Context, in *requests.ConversationRequest,
) (*ConversationOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	loaded, err := c.Show.Handle(ctx, conversations.ShowConversationInput{Membership: membership(ctx), Ref: ref})
	if err != nil {
		return nil, err
	}

	return &ConversationOutput{Body: fhttp.NewResource(resources.NewConversation(loaded))}, nil
}

/*
Store starts a conversation and answers with it.
*/
func (c *ConversationController) Store(
	ctx context.Context, in *requests.StoreConversationRequest,
) (*ConversationOutput, error) {
	b := in.Body
	create := conversations.CreateConversationInput{
		Membership: membership(ctx), ByContact: b.Author != nil && *b.Author == "contact", Subject: b.Subject,
		Body: b.Body, AssignSet: requests.Sent(in.RawBody)["assignee_id"],
		Contact:    contacts.Ref{ExternalID: b.Contact.ExternalID, Email: b.Contact.Email, Name: b.Contact.Name},
		MutationID: mutationID(ctx),
	}

	var err error

	create.InboxID, err = decodeField(c.IDs, models.PrefixInbox, b.InboxID, exceptions.ErrInboxNotFound)
	if err != nil {
		return nil, err
	}

	if create.ConnectionID, err = decodeField(c.IDs, models.PrefixChannelConnection, b.ChannelConnectionID,
		exceptions.ErrChannelCannotInitiate); err != nil {
		return nil, err
	}

	create.Contact.ID, err = decodeField(c.IDs, models.PrefixContact, b.Contact.ID, exceptions.ErrNotFound)
	if err != nil {
		return nil, err
	}

	if create.Assignee, err = c.member(ctx, b.AssigneeID); err != nil {
		return nil, err
	}

	loaded, err := c.Create.Handle(ctx, create)
	if err != nil {
		return nil, err
	}

	return &ConversationOutput{Body: fhttp.NewResource(resources.NewConversation(loaded))}, nil
}

/*
Update changes the fields sent and answers with the conversation as it
now is and the fields that changed.
*/
func (c *ConversationController) Update(
	ctx context.Context, in *requests.UpdateConversationRequest,
) (*ConversationChangeOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	changes, err := c.changes(ctx, in)
	if err != nil {
		return nil, err
	}

	applied, err := c.Apply.Handle(ctx, conversations.ApplyChangesInput{
		Membership: membership(ctx), Ref: ref, Changes: changes, MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	out := &ConversationChangeOutput{}
	out.Body.Data = resources.NewConversation(applied.Conversation)
	out.Body.Meta.Changed = append([]string{}, applied.Changed...)

	return out, nil
}

/*
changes turns the request into the action's changes, resolving the ids
it names.
*/
func (c *ConversationController) changes(
	ctx context.Context, in *requests.UpdateConversationRequest,
) (conversations.Changes, error) {
	b, sent := in.Body, requests.Sent(in.RawBody)
	ch := conversations.Changes{
		Spam: b.Spam, AssignMember: sent["assignee_id"], AssignTeam: sent["team_id"], Status: b.Status,
		SnoozedUntil: b.SnoozedUntil, Note: b.Note, Unread: b.Unread,
		Attributes: conversations.Attributes{Subject: b.Subject},
	}

	if b.Priority != nil {
		ch.Attributes.Priority = new(enums.ConversationPriority(*b.Priority))
	}

	if b.SnoozePreset != nil {
		ch.SnoozePreset = *b.SnoozePreset
	}

	if b.SnoozeOption != nil {
		ch.SnoozeOption = *b.SnoozeOption
	}

	var err error
	if ch.Assign.Member, err = c.member(ctx, b.AssigneeID); err != nil {
		return ch, err
	}

	if ch.Assign.Team, err = decodeField(c.IDs, models.PrefixTeam, b.TeamID, exceptions.ErrTeamNotFound); err != nil {
		return ch, err
	}

	ch.AddTags, ch.RemoveTags = tagRefs(c.IDs, b.AddTags), decodeTagIDs(c.IDs, b.RemoveTags)
	ch.Attributes.InboxID, err = decodeField(c.IDs, models.PrefixInbox, b.InboxID, exceptions.ErrInboxNotFound)

	return ch, err
}

/*
member resolves an assignee: a member id, or me for the caller.
*/
func (c *ConversationController) member(ctx context.Context, named *string) (*ids.UUID, error) {
	if named != nil && *named == requests.CurrentMember {
		return new(membership(ctx).Member.ID), nil
	}

	return decodeField(c.IDs, models.PrefixMember, named, exceptions.ErrMemberInactive)
}

/*
ListNeighbors answers with the conversations around one in the list the
query names.
*/
func (c *ConversationController) ListNeighbors(
	ctx context.Context, in *requests.ConversationRequest,
) (*NeighborsOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), ConversationListing())
	if err != nil {
		return nil, err
	}

	lq, err := parseListQuery(c.IDs, query)
	if err != nil {
		return nil, err
	}

	n, err := c.Neighbors.Handle(ctx, conversations.NeighborsInput{
		Membership: membership(ctx), Ref: ref, Filters: lq.filters, Sort: lq.sort,
	})
	if err != nil {
		return nil, err
	}

	return &NeighborsOutput{Body: fhttp.NewResource(resources.NewConversationNeighbors(n))}, nil
}

/*
conversationRef reads a conversation's public id or number from a path.
*/
func conversationRef(registry *ids.Registry, named string) (conversations.Ref, error) {
	if number, err := strconv.ParseInt(named, 10, 64); err == nil && number > 0 {
		return conversations.Ref{Number: number}, nil
	}

	id, err := registry.Decode(string(models.PrefixConversation), named)
	if err != nil {
		return conversations.Ref{}, exceptions.ErrNotFound
	}

	return conversations.Ref{ID: id}, nil
}

/*
decodeField reads an optional public id a body names, refusing one that
is not an id of the kind with the error given.
*/
func decodeField(registry *ids.Registry, prefix models.Prefix, named *string, refusal error) (*ids.UUID, error) {
	if named == nil {
		return nil, nil
	}

	id, err := registry.Decode(string(prefix), *named)
	if err != nil {
		return nil, refusal
	}

	return &id, nil
}

/*
mutationID returns the id the client gave the change the request makes,
"" when it gave none, so the events of the change carry it back.
*/
func mutationID(ctx context.Context) string {
	r, ok := fhttp.RequestFrom(ctx)
	if !ok {
		return ""
	}

	return appmiddleware.ClientMutationID(r)
}

/*
includeViewers adds who views each conversation of the page, the caller
left out, when the workspace shows viewers. Viewers that do not arrive
by the deadline are left out rather than holding the list back.
*/
func (c *ConversationController) includeViewers(
	ctx context.Context, items []resources.Conversation, page []conversations.Loaded,
) {
	m := membership(ctx)
	if !m.Workspace.ShowsViewersOnList() || len(page) == 0 {
		return
	}

	wanted := make([]ids.UUID, len(page))
	for i, l := range page {
		wanted[i] = l.Conversation.ID
	}

	ctx, cancel := context.WithTimeout(ctx, appbroadcasting.ViewersDeadline)
	defer cancel()

	viewers, err := c.Viewers.Of(ctx, m.Workspace.ID, wanted)
	if err != nil {
		return
	}

	for i, id := range wanted {
		others := slices.DeleteFunc(viewers[id], func(v appbroadcasting.Viewer) bool {
			return v.MemberID == m.Member.ID
		})
		items[i].Viewers = new(appbroadcasting.NewViewerPayloads(others))
	}
}

/*
cursor decodes where the page starts, nil for the first page, refusing
a cursor made for another list.
*/
func (c *ConversationController) cursor(encoded, hash string) (*conversationCursor, error) {
	if encoded == "" {
		return nil, nil
	}

	var start conversationCursor
	if err := c.Cursors.Decode(encoded, &start); err != nil {
		return nil, pagination.ErrInvalidCursor
	}

	if start.Hash != hash {
		return nil, exceptions.ErrCursorMismatch
	}

	return &start, nil
}

/*
Destroy moves a conversation to the trash.
*/
func (c *ConversationController) Destroy(ctx context.Context, in *requests.ConversationRequest) (*EmptyOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	err = c.Trash.Handle(ctx, conversations.TrashInput{Membership: membership(ctx), Ref: ref, MutationID: mutationID(ctx)})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
StoreRestore takes a conversation out of the trash.
*/
func (c *ConversationController) StoreRestore(
	ctx context.Context, in *requests.ConversationRequest,
) (*ConversationOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	loaded, err := c.Restore.Handle(ctx, conversations.TrashInput{
		Membership: membership(ctx), Ref: ref, MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	return &ConversationOutput{Body: fhttp.NewResource(resources.NewConversation(loaded))}, nil
}
