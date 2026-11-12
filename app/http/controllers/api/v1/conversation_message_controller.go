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
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/auth"
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
EntryOutput is an entry a request changed, with the conversation as it
now is.
*/
type EntryOutput struct {
	Body struct {
		Data resources.TimelineEntry `json:"data"`
		Meta struct {
			Conversation resources.Conversation `json:"conversation"`
		} `json:"meta"`
	}
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
	List     *messages.ListMessages
	Reply    *messages.ReplyToConversation
	Note     *messages.AddNote
	Relay    *messages.AddContactMessage
	Import   *messages.ImportMessage
	Delete   *messages.DeleteMessage
	Edit     *messages.EditNote
	Original *messages.ShowOriginal
	Raw      *messages.DownloadOriginal
	Resend   *messages.RetryDelivery
	IDs      *ids.Registry
	Cursors  *pagination.Cursors
}

/*
timelineCursor is where a page of a timeline starts.
*/
type timelineCursor struct {
	At time.Time `json:"t"`
	ID ids.UUID  `json:"i"`
}

/*
TimelineListing is what a conversation's timeline accepts: a filter on
the type of entry, and pages of the timeline's own default size.
*/
func TimelineListing() pagination.Allow {
	return pagination.Allow{Filters: []string{"type"}, DefaultLimit: defaultTimelinePage}
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

	query, err := pagination.Parse(values, TimelineListing())
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
		Items: resources.NewTimelineEntries(t.Messages, t.Contact, t.Members, t.Files), Limit: limit,
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
	if b.Importing() {
		return c.importEntry(ctx, historyEntry{
			ref: ref, kind: messages.ImportReply, body: b.Body, history: b.HistoryFields, then: b.Then,
		})
	}

	reply := messages.ReplyInput{
		Membership: membership(ctx), Ref: ref, Body: b.Body, ClientMessageID: b.ClientMessageID,
		Then: messages.ThenKeep, Delay: b.Delay && bySession(ctx), MutationID: mutationID(ctx),
	}

	if reply.Files, err = c.files(b.AttachmentIDs); err != nil {
		return nil, err
	}

	if b.Then != nil {
		reply.Then = *b.Then
	}

	if b.Snooze != nil {
		reply.Snooze = snoozeInput(b.Snooze)
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
snoozeInput returns when the snooze a reply asks for ends.
*/
func snoozeInput(s *requests.SnoozeBody) messages.SnoozeInput {
	out := messages.SnoozeInput{Until: s.Until}
	if s.Option != nil {
		out.Option = *s.Option
	}

	if s.Preset != nil {
		out.Preset = *s.Preset
	}

	return out
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

	if in.Body.Importing() {
		return c.importEntry(ctx, historyEntry{
			ref: ref, kind: messages.ImportNote, body: in.Body.Body, history: in.Body.HistoryFields,
		})
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

	if in.Body.CreatedAt != nil || in.Body.ExternalID != nil {
		history := requests.HistoryFields{CreatedAt: in.Body.CreatedAt, ExternalID: in.Body.ExternalID}

		return c.importEntry(ctx, historyEntry{
			ref: ref, kind: messages.ImportContactMessage, body: in.Body.Body, history: history,
		})
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

/*
UndoneOutput is a message taken away: undone with its text, so the
composer holds it again, or deleted.
*/
type UndoneOutput struct {
	Body struct {
		Data struct {
			Object string `json:"object"`
			Undone bool   `json:"undone"`
			Body   string `json:"body"`
		} `json:"data"`
		Meta struct {
			Conversation resources.Conversation `json:"conversation"`
		} `json:"meta"`
	}
}

/*
Destroy undoes the caller's reply still waiting to leave, or deletes the
message.
*/
func (c *ConversationMessageController) Destroy(
	ctx context.Context, in *requests.MessageRequest,
) (*UndoneOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	id, err := c.IDs.Decode(string(models.PrefixMessage), in.Message)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	removed, err := c.Delete.Handle(ctx, messages.UndoInput{
		Membership: membership(ctx), Ref: ref, MessageID: id, MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	out := &UndoneOutput{}
	out.Body.Data.Object, out.Body.Data.Undone, out.Body.Data.Body = "message", removed.Undone, removed.Body
	out.Body.Meta.Conversation = resources.NewConversation(removed.Conversation)

	return out, nil
}

/*
bySession reports whether the request came from a signed-in browser,
the only caller whose replies wait for undo; programs send at once.
*/
func bySession(ctx context.Context) bool {
	p, ok := auth.CurrentPrincipal(ctx)

	return ok && p.Kind == auth.KindSession
}

/*
Patch rewrites one of the caller's notes.
*/
func (c *ConversationMessageController) Patch(
	ctx context.Context, in *requests.EditMessageRequest,
) (*EntryOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	id, err := c.IDs.Decode(string(models.PrefixMessage), in.Message)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	written, err := c.Edit.Handle(ctx, messages.EditInput{
		Membership: membership(ctx), Ref: ref, MessageID: id, Body: in.Body.Body, MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	out := writtenOutput(written)

	return &EntryOutput{Body: out.Body}, nil
}

/*
files decodes the attachment ids a reply names; one that is not an
attachment id is simply not found.
*/
func (c *ConversationMessageController) files(named []string) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(named))
	for _, public := range named {
		id, err := c.IDs.Decode(string(models.PrefixAttachment), strings.TrimSpace(public))
		if err != nil {
			return nil, exceptions.ErrAttachmentNotFound
		}

		out = append(out, id)
	}

	return out, nil
}
