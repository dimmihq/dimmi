/*
|--------------------------------------------------------------------------
| Merge And Bulk Controllers
|--------------------------------------------------------------------------
|
| POST /conversation-bulk-changes changes many conversations at once and
| answers how each one went. POST /conversations/{conversation}/merges
| folds other conversations of the same customer into the one the path
| names, and POST /contacts/{contact}/merges folds a duplicate contact
| into the one the path names. Ids of other workspaces are simply not
| found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/merges"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationBulkResult is how one conversation of a bulk change went: the fields
that changed, or the code of the refusal that stopped it.
*/
type ConversationBulkResult struct {
	ID      string   `json:"id"`
	OK      bool     `json:"ok"`
	Changed []string `json:"changed"`
	Error   string   `json:"error,omitempty"`
}

/*
BulkChangesOutput is how each conversation of a bulk change went, in
the order named.
*/
type BulkChangesOutput struct {
	Body struct {
		Data []ConversationBulkResult `json:"data"`
	}
}

/*
MergeController serves bulk changes and merges.
*/
type MergeController struct {
	Bulk          *conversations.BulkChangeConversations
	Conversations *ConversationController
	Merge         *merges.MergeConversations
	MergeContacts *merges.MergeContacts
	ShowContact   *contacts.ShowContact
	IDs           *ids.Registry
}

/*
StoreBulk changes the conversations named and answers how each went. An
id that is not a conversation's is reported as not found.
*/
func (c *MergeController) StoreBulk(
	ctx context.Context, in *requests.BulkConversationChangesRequest,
) (*BulkChangesOutput, error) {
	input := conversations.BulkChangesInput{
		Membership: membership(ctx), Trash: in.Body.Changes == nil, MutationID: mutationID(ctx),
	}

	if in.Body.Changes != nil {
		changes, err := c.Conversations.changes(ctx, *in.Body.Changes, in.SentChanges())
		if err != nil {
			return nil, err
		}

		input.Changes = changes
	}

	out := &BulkChangesOutput{}
	out.Body.Data = []ConversationBulkResult{}

	for _, named := range in.Body.ConversationIDs {
		id, err := c.IDs.Decode(string(models.PrefixConversation), named)
		if err != nil {
			out.Body.Data = append(out.Body.Data, ConversationBulkResult{
				ID: named, Changed: []string{}, Error: exceptions.ErrNotFound.Code(),
			})

			continue
		}

		input.IDs = append(input.IDs, id)
	}

	if len(input.IDs) == 0 {
		return out, nil
	}

	results, err := c.Bulk.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	for _, r := range results {
		out.Body.Data = append(out.Body.Data, ConversationBulkResult{
			ID: models.PublicID(models.PrefixConversation, r.ID), OK: r.Error == "", Changed: r.Changed, Error: r.Error,
		})
	}

	return out, nil
}

/*
StoreConversationMerge folds the conversations named into the one the
path names, and answers with it as it now is.
*/
func (c *MergeController) StoreConversationMerge(
	ctx context.Context, in *requests.MergeConversationsRequest,
) (*ConversationOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	sources := make([]ids.UUID, 0, len(in.Body.SourceIDs))

	for _, named := range in.Body.SourceIDs {
		id, err := c.IDs.Decode(string(models.PrefixConversation), named)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}

		sources = append(sources, id)
	}

	loaded, err := c.Merge.Handle(ctx, merges.MergeConversationsInput{
		Membership: membership(ctx), Target: ref, Sources: sources, MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	return &ConversationOutput{Body: fhttp.NewResource(resources.NewConversation(loaded))}, nil
}

/*
StoreContactMerge folds the contact named into the one the path names,
and answers with the survivor as it now is.
*/
func (c *MergeController) StoreContactMerge(
	ctx context.Context, in *requests.MergeContactRequest,
) (*ContactOutput, error) {
	target, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	source, err := c.IDs.Decode(string(models.PrefixContact), in.Body.SourceID)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	m := membership(ctx)

	merged, err := c.MergeContacts.Handle(ctx, merges.MergeContactsInput{
		Membership: m, TargetID: target, SourceID: source,
	})
	if err != nil {
		return nil, err
	}

	d, err := c.ShowContact.Handle(ctx, contacts.ShowContactInput{Membership: m, ContactID: merged.ID})
	if err != nil {
		return nil, err
	}

	return &ContactOutput{Body: fhttp.NewResource(resources.NewContactDetail(d))}, nil
}
