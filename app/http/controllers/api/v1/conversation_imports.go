/*
|--------------------------------------------------------------------------
| Conversation Imports
|--------------------------------------------------------------------------
|
| The conversation and timeline endpoints also take history from another
| tool: a created_at, an external_id or an author_id turns a request
| into an import, written at its own time by its own author and seen by
| nothing that reacts to new work. Sending the same external id again
| answers with the first copy and the Idempotent-Replayed header.
|
*/

package v1

import (
	"context"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
StoredConversationOutput is a conversation a request started, or the
one an earlier import with the same external id brought in.
*/
type StoredConversationOutput struct {
	Status   int
	Replayed string `header:"Idempotent-Replayed"`
	Body     fhttp.Resource[resources.Conversation]
}

/*
storedConversation shapes a started conversation, 201 Created.
*/
func storedConversation(loaded conversations.Loaded) *StoredConversationOutput {
	return &StoredConversationOutput{
		Status: fhttp.StatusCreated, Body: fhttp.NewResource(resources.NewConversation(loaded)),
	}
}

/*
storeImported brings a conversation in from another tool.
*/
func (c *ConversationController) storeImported(
	ctx context.Context, in *requests.StoreConversationRequest, create conversations.CreateConversationInput,
) (*StoredConversationOutput, error) {
	b := in.Body
	imp := conversations.ConversationImport{
		Backfill: conversations.Backfill{At: orZero(b.CreatedAt), ExternalID: b.ExternalID},
		Closed:   b.Status != nil && *b.Status == "closed", ClosedAt: b.ClosedAt,
	}

	var err error
	if imp.AuthorID, err = decodeField(c.IDs, models.PrefixMember, b.AuthorID,
		exceptions.ErrNotFound.OnField("author_id")); err != nil {
		return nil, err
	}

	loaded, replayed, err := c.Create.Import(ctx, create, imp)
	if err != nil {
		return nil, err
	}

	out := storedConversation(loaded)
	if replayed {
		out.Status, out.Replayed = fhttp.StatusOK, "true"
	}

	return out, nil
}

/*
historyEntry is one entry of history a request brings in: the
conversation, the kind of entry and its body, what it says about
itself, and what the conversation does afterwards, nil to keep it open.
*/
type historyEntry struct {
	ref     conversations.Ref
	kind    string
	body    string
	history requests.HistoryFields
	then    *string
}

/*
importEntry brings one entry of history into the conversation.
*/
func (c *ConversationMessageController) importEntry(ctx context.Context, e historyEntry) (*WrittenOutput, error) {
	h := e.history
	in := messages.ImportMessageInput{
		Membership: membership(ctx), Ref: e.ref, Kind: e.kind, Body: e.body, MutationID: mutationID(ctx),
		Backfill: conversations.Backfill{At: orZero(h.CreatedAt), ExternalID: h.ExternalID},
		Then:     messages.ThenKeep,
	}
	if e.then != nil {
		in.Then = *e.then
	}

	var err error
	if in.Backfill.AuthorID, err = decodeField(c.IDs, models.PrefixMember, h.AuthorID,
		exceptions.ErrNotFound.OnField("author_id")); err != nil {
		return nil, err
	}

	written, err := c.Import.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	return writtenOutput(written), nil
}

/*
orZero returns the time, or the zero time for none.
*/
func orZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}

	return *t
}
