/*
|--------------------------------------------------------------------------
| Conversation Message Controller
|--------------------------------------------------------------------------
|
| GET /conversations/{conversation}/messages reads the timeline a page
| at a time: the latest page, ?before= older ones and ?after= newer ones,
| filter[type] narrowing to messages, notes or events. POST .../replies
| answers the customer, POST .../notes writes to the team only, and POST
| .../messages relays a customer's message on an API conversation. A
| retried write that finds the entry its first attempt wrote answers 200
| with the Idempotent-Replayed header instead of 201.
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
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
defaultTimelinePage is how many entries a timeline page holds unless
asked otherwise.
*/
const defaultTimelinePage = 50

/*
TimelineOutput is a page of a timeline.
*/
type TimelineOutput struct {
	Body fhttp.Collection[resources.TimelineEntry]
}

/*
WrittenOutput is an entry a request wrote and the conversation as it
now is, under 201, or 200 for a replayed request.
*/
type WrittenOutput struct {
	Status   int
	Replayed string `header:"Idempotent-Replayed"`
	Body     struct {
		Data resources.TimelineEntry `json:"data"`
		Meta struct {
			Conversation resources.Conversation `json:"conversation"`
		} `json:"meta"`
	}
}

/*
ConversationMessageController serves conversations' timelines.
*/
type ConversationMessageController struct {
	List    *messages.ListMessages
	Reply   *messages.ReplyToConversation
	Note    *messages.AddNote
	Relay   *messages.AddContactMessage
	IDs     *ids.Registry
	Cursors *pagination.Cursors
}

/*
timelineCursor is where a page of a timeline starts.
*/
type timelineCursor struct {
	At time.Time `json:"t"`
	ID ids.UUID  `json:"i"`
}

/*
Index reads a page of a conversation's timeline. The page's prev cursor
reads older entries with ?before=, its next cursor newer ones with
?after=.
*/
func (c *ConversationMessageController) Index(
	ctx context.Context, in *requests.ConversationRequest,
) (*TimelineOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	r, _ := fhttp.RequestFrom(ctx)
	values := r.URL.Query()

	query, err := pagination.Parse(values, pagination.Allow{
		Filters: []string{"type"}, DefaultLimit: defaultTimelinePage,
	})
	if err != nil {
		return nil, err
	}

	list := messages.ListMessagesInput{
		Membership: membership(ctx), Ref: ref, Types: query.Filters["type"], Limit: query.Limit,
	}
	for _, t := range list.Types {
		if !enums.MessageType(t).Valid() {
			return nil, pagination.ErrInvalidFilter
		}
	}

	if list.Before, err = c.cursor(values.Get("before")); err != nil {
		return nil, err
	}

	if list.After, err = c.cursor(values.Get("after")); err != nil {
		return nil, err
	}

	timeline, err := c.List.Handle(ctx, list)
	if err != nil {
		return nil, err
	}

	return c.page(timeline, list.Limit)
}

/*
page shapes a timeline page with the cursors to either side.
*/
func (c *ConversationMessageController) page(t messages.Timeline, limit int) (*TimelineOutput, error) {
	out := pagination.Page[resources.TimelineEntry]{
		Items: resources.NewTimelineEntries(t.Messages, t.Contact, t.Members), Limit: limit,
	}

	if len(t.Messages) == 0 {
		return &TimelineOutput{Body: fhttp.NewCollection(out)}, nil
	}

	var err error

	first, last := t.Messages[0], t.Messages[len(t.Messages)-1]
	if t.Older {
		if out.PrevCursor, err = c.Cursors.Encode(timelineCursor{At: first.CreatedAt, ID: first.ID}); err != nil {
			return nil, err
		}
	}

	if out.NextCursor, err = c.Cursors.Encode(timelineCursor{At: last.CreatedAt, ID: last.ID}); err != nil {
		return nil, err
	}

	return &TimelineOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
cursor reads a timeline cursor, nil when none was sent.
*/
func (c *ConversationMessageController) cursor(raw string) (*messages.Cursor, error) {
	if raw == "" {
		return nil, nil
	}

	var tc timelineCursor
	if err := c.Cursors.Decode(raw, &tc); err != nil {
		return nil, err
	}

	return &messages.Cursor{At: tc.At, ID: tc.ID}, nil
}

/*
StoreReply sends a reply to the customer.
*/
func (c *ConversationMessageController) StoreReply(
	ctx context.Context, in *requests.ReplyRequest,
) (*WrittenOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	b := in.Body
	reply := messages.ReplyInput{
		Membership: membership(ctx), Ref: ref, Body: b.Body, ClientMessageID: b.ClientMessageID,
		Then: messages.ThenKeep, MutationID: mutationID(ctx),
	}

	if b.Then != nil {
		reply.Then = *b.Then
	}

	if b.Snooze != nil {
		reply.Snooze.Until = b.Snooze.Until
		if b.Snooze.Option != nil {
			reply.Snooze.Option = *b.Snooze.Option
		}

		if b.Snooze.Preset != nil {
			reply.Snooze.Preset = *b.Snooze.Preset
		}
	}

	if b.ExpectedLastMessageID != nil {
		id, err := c.IDs.Decode(string(models.PrefixMessage), *b.ExpectedLastMessageID)
		if err != nil {
			return nil, exceptions.ErrNewMessagesSince
		}

		reply.ExpectedLastMessageID = &id
	}

	written, err := c.Reply.Handle(ctx, reply)
	if err != nil {
		return nil, err
	}

	return writtenOutput(written), nil
}

/*
StoreNote writes a note to the team.
*/
func (c *ConversationMessageController) StoreNote(
	ctx context.Context, in *requests.NoteRequest,
) (*WrittenOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	written, err := c.Note.Handle(ctx, messages.NoteInput{
		Membership: membership(ctx), Ref: ref, Body: in.Body.Body, ClientMessageID: in.Body.ClientMessageID,
		MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	return writtenOutput(written), nil
}

/*
Store relays a customer's message on an API conversation.
*/
func (c *ConversationMessageController) Store(
	ctx context.Context, in *requests.ContactMessageRequest,
) (*WrittenOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	written, err := c.Relay.Handle(ctx, messages.ContactMessageInput{
		Membership: membership(ctx), Ref: ref, Body: in.Body.Body, DedupeKey: in.Body.DedupeKey,
		MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	return writtenOutput(written), nil
}

/*
writtenOutput shapes an entry a request wrote.
*/
func writtenOutput(w messages.Written) *WrittenOutput {
	out := &WrittenOutput{Status: fhttp.StatusCreated}
	if w.Replayed {
		out.Status, out.Replayed = fhttp.StatusOK, "true"
	}

	authors := map[ids.UUID]conversations.Member{w.Author.Member.ID: w.Author}
	out.Body.Data = resources.NewTimelineEntry(w.Message, w.Conversation.Contact, authors)
	out.Body.Meta.Conversation = resources.NewConversation(w.Conversation)

	return out
}
