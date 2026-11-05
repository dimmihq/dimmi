/*
|--------------------------------------------------------------------------
| Draft Controller
|--------------------------------------------------------------------------
|
| Each member lists, saves and discards their own drafts. A save that
| loses to a newer stored draft still succeeds and answers with what is
| stored, flagged stale, so the client takes it.
|
*/

package v1

import (
	"context"
	"encoding/json"
	"fmt"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/drafts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxDraftsListed is the most drafts one list shows.
*/
const maxDraftsListed = 100

/*
DraftsOutput is the caller's drafts.
*/
type DraftsOutput struct {
	Body fhttp.Collection[resources.Draft]
}

/*
DraftOutput is a saved draft, under 201 when the save started it.
Upserted says an existing draft was updated; Stale says the stored draft
was newer and nothing was written.
*/
type DraftOutput struct {
	Status int
	Body   struct {
		Data resources.Draft `json:"data"`
		Meta struct {
			Upserted bool `json:"upserted"`
			Stale    bool `json:"stale"`
		} `json:"meta"`
	}
}

/*
DraftController serves the caller's drafts.
*/
type DraftController struct {
	List   *drafts.ListDrafts
	Save   *drafts.SaveDraft
	Update *drafts.UpdateDraft
	Delete *drafts.DeleteDraft
	IDs    *ids.Registry
}

/*
Index lists the caller's drafts, those of filter[conversation] or
filter[mode] when sent.
*/
func (c *DraftController) Index(ctx context.Context, _ *requests.NoInput) (*DraftsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)
	query := r.URL.Query()
	in := drafts.ListInput{Membership: membership(ctx)}

	if named := query.Get("filter[conversation]"); named != "" {
		id, err := c.IDs.Decode(string(models.PrefixConversation), named)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}

		in.Conversation = &id
	}

	if mode := query.Get("filter[mode]"); mode != "" {
		in.Mode = &mode
	}

	rows, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.Draft]{Items: resources.NewDrafts(rows), Limit: maxDraftsListed}

	return &DraftsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store saves a draft of a conversation and mode, updating the one there
when upsert is asked.
*/
func (c *DraftController) Store(ctx context.Context, in *requests.StoreDraftRequest) (*DraftOutput, error) {
	conversation, err := decodeField(c.IDs, models.PrefixConversation, in.Body.ConversationID, exceptions.ErrNotFound)
	if err != nil {
		return nil, err
	}

	content, err := c.content(in.Body.DraftContentBody)
	if err != nil {
		return nil, err
	}

	saved, err := c.Save.Handle(ctx, drafts.SaveInput{
		Membership: membership(ctx), Conversation: conversation, Mode: in.Body.Mode, Content: content,
		Upsert: in.Upsert, MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	return draftOutput(saved), nil
}

/*
Patch replaces a draft's content.
*/
func (c *DraftController) Patch(ctx context.Context, in *requests.UpdateDraftRequest) (*DraftOutput, error) {
	id, err := c.draftID(in.Draft)
	if err != nil {
		return nil, err
	}

	content, err := c.content(in.Body)
	if err != nil {
		return nil, err
	}

	saved, err := c.Update.Handle(ctx, drafts.UpdateInput{
		Membership: membership(ctx), ID: id, Content: content, MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	return draftOutput(saved), nil
}

/*
Destroy discards a draft.
*/
func (c *DraftController) Destroy(ctx context.Context, in *requests.DraftRequest) (*EmptyOutput, error) {
	id, err := c.draftID(in.Draft)
	if err != nil {
		return nil, err
	}

	err = c.Delete.Handle(ctx, drafts.DeleteInput{Membership: membership(ctx), ID: id, MutationID: mutationID(ctx)})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
content reads what the body says the draft holds.
*/
func (c *DraftController) content(b requests.DraftContentBody) (drafts.Content, error) {
	out := drafts.Content{Body: b.Body, ClientUpdatedAt: b.ClientUpdatedAt}

	for _, named := range b.AttachmentIDs {
		id, err := c.IDs.Decode(string(models.PrefixAttachment), named)
		if err != nil {
			return out, exceptions.ErrDraftAttachments
		}

		out.AttachmentIDs = append(out.AttachmentIDs, id)
	}

	if b.Recipients != nil {
		raw, err := json.Marshal(b.Recipients)
		if err != nil {
			return out, fmt.Errorf("encode draft recipients: %w", err)
		}

		out.Recipients = raw
	}

	return out, nil
}

/*
draftID decodes a draft's public id; text that is none is not found.
*/
func (c *DraftController) draftID(named string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixDraft), named)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}

func draftOutput(saved drafts.Saved) *DraftOutput {
	out := &DraftOutput{Status: fhttp.StatusOK}
	if saved.Created {
		out.Status = fhttp.StatusCreated
	}

	out.Body.Data = resources.NewDraft(saved.Draft)
	out.Body.Meta.Upserted = !saved.Created && !saved.Stale
	out.Body.Meta.Stale = saved.Stale

	return out
}
