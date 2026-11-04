/*
|--------------------------------------------------------------------------
| Inbox View Controller
|--------------------------------------------------------------------------
|
| What the inbox's chrome reads: GET /conversation-counts for the
| sidebar's counts, GET and PATCH /conversations/{conversation}/read-state
| for how far the caller has read a conversation, and GET /search for
| the quick search.
|
*/

package v1

import (
	"context"
	"strconv"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Quick search page sizes: the default and the most results of each kind.
*/
const (
	defaultSearchLimit = 5
	maxSearchLimit     = 20
)

/*
CountsOutput is the sidebar's counts.
*/
type CountsOutput struct {
	Body fhttp.Resource[resources.ConversationCounts]
}

/*
ReadStateOutput is the caller's read state of a conversation.
*/
type ReadStateOutput struct {
	Body fhttp.Resource[resources.ConversationReadState]
}

/*
SearchOutput is what a quick search found.
*/
type SearchOutput struct {
	Body fhttp.Resource[resources.SearchResults]
}

/*
InboxViewController serves counts, read state and search.
*/
type InboxViewController struct {
	Counts   *conversations.CountConversations
	ReadShow *conversations.ShowReadState
	ReadSave *conversations.UpdateReadState
	Search   *conversations.Search
	IDs      *ids.Registry
	Clock    clock.Clock
}

/*
CountIndex answers with how many conversations wait in each view.
*/
func (c *InboxViewController) CountIndex(ctx context.Context, _ *requests.NoInput) (*CountsOutput, error) {
	counts, err := c.Counts.Handle(ctx, conversations.CountsInput{Membership: membership(ctx)})
	if err != nil {
		return nil, err
	}

	return &CountsOutput{Body: fhttp.NewResource(resources.NewConversationCounts(counts, c.Clock.Now()))}, nil
}

/*
ReadState answers with how far the caller has read the conversation.
*/
func (c *InboxViewController) ReadState(
	ctx context.Context, in *requests.ConversationRequest,
) (*ReadStateOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	state, err := c.ReadShow.Handle(ctx, conversations.ReadStateInput{Membership: membership(ctx), Ref: ref})
	if err != nil {
		return nil, err
	}

	return &ReadStateOutput{Body: fhttp.NewResource(resources.NewConversationReadState(state))}, nil
}

/*
UpdateReadState moves the caller's read marker and answers with the
read state as it now is.
*/
func (c *InboxViewController) UpdateReadState(
	ctx context.Context, in *requests.ReadStateRequest,
) (*ReadStateOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	update := conversations.UpdateReadStateInput{
		Membership: membership(ctx), Ref: ref, Unread: in.Body.Unread != nil && *in.Body.Unread,
	}

	if named := in.Body.LastReadMessageID; named != nil {
		id, err := c.IDs.Decode(string(models.PrefixMessage), *named)
		if err != nil {
			return nil, exceptions.ErrMessageNotInConversation
		}

		update.UpTo = &id
	}

	state, err := c.ReadSave.Handle(ctx, update)
	if err != nil {
		return nil, err
	}

	return &ReadStateOutput{Body: fhttp.NewResource(resources.NewConversationReadState(state))}, nil
}

/*
SearchIndex runs the quick search of ?q=, ?limit= results of each kind.
*/
func (c *InboxViewController) SearchIndex(ctx context.Context, _ *requests.NoInput) (*SearchOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)
	values := r.URL.Query()

	limit := defaultSearchLimit
	if raw := values.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxSearchLimit {
			return nil, pagination.ErrInvalidLimit
		}

		limit = n
	}

	found, err := c.Search.Handle(ctx, conversations.SearchInput{
		Membership: membership(ctx), Query: values.Get("q"), Limit: limit,
	})
	if err != nil {
		return nil, err
	}

	return &SearchOutput{Body: fhttp.NewResource(resources.NewSearchResults(found))}, nil
}
