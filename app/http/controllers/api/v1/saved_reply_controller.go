/*
|--------------------------------------------------------------------------
| Saved Reply Controllers
|--------------------------------------------------------------------------
|
| GET /saved-replies lists the shared replies and the caller's own, q
| searching titles and shortcuts. POST, PATCH and DELETE add, change and
| remove one, and POST /saved-reply-imports adds many from a spreadsheet.
| Another member's personal reply is simply not found.
|
*/

package v1

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/savedreplies"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SavedRepliesOutput is the replies the caller may use.
*/
type SavedRepliesOutput struct {
	Body fhttp.Collection[resources.SavedReply]
}

/*
SavedReplyOutput is one reply.
*/
type SavedReplyOutput struct {
	Body fhttp.Resource[resources.SavedReply]
}

/*
SavedReplyController serves the workspace's saved replies.
*/
type SavedReplyController struct {
	List   *savedreplies.ListSavedReplies
	Show   *savedreplies.ShowSavedReply
	Create *savedreplies.CreateSavedReply
	Change *savedreplies.UpdateSavedReply
	Remove *savedreplies.DeleteSavedReply
	Import *savedreplies.ImportSavedReplies
	IDs    *ids.Registry
}

/*
Index lists the replies by title, q narrowing to titles and shortcuts
holding it.
*/
func (c *SavedReplyController) Index(ctx context.Context, _ *requests.NoInput) (*SavedRepliesOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)
	in := savedreplies.ListSavedRepliesInput{Membership: membership(ctx)}

	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		in.Search = &q
	}

	rows, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.SavedReply]{
		Items: resources.NewSavedReplies(rows), Limit: savedreplies.MaxListed,
	}

	return &SavedRepliesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Get returns one reply.
*/
func (c *SavedReplyController) Get(ctx context.Context, in *requests.SavedReplyRequest) (*SavedReplyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixSavedReply), in.Reply)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	row, err := c.Show.Handle(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &SavedReplyOutput{Body: fhttp.NewResource(resources.NewSavedReply(row))}, nil
}

/*
Store adds a reply and answers with it.
*/
func (c *SavedReplyController) Store(
	ctx context.Context, in *requests.StoreSavedReplyRequest,
) (*SavedReplyOutput, error) {
	b := in.Body

	macro, err := c.macro(b.Actions)
	if err != nil {
		return nil, err
	}

	row, err := c.Create.Handle(ctx, savedreplies.CreateSavedReplyInput{
		Membership: membership(ctx),
		Fields: savedreplies.Fields{
			Title: &b.Title, Body: &b.Body, Shortcut: b.Shortcut, Shared: b.Shared, Macro: macro,
		},
	})
	if err != nil {
		return nil, err
	}

	return &SavedReplyOutput{Body: fhttp.NewResource(resources.NewSavedReply(row))}, nil
}

/*
Update changes a reply and answers with it as it now is.
*/
func (c *SavedReplyController) Update(
	ctx context.Context, in *requests.UpdateSavedReplyRequest,
) (*SavedReplyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixSavedReply), in.Reply)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	b := in.Body

	macro, err := c.macro(b.Actions)
	if err != nil {
		return nil, err
	}

	row, err := c.Change.Handle(ctx, savedreplies.UpdateSavedReplyInput{
		Membership: membership(ctx), ID: id,
		Fields: savedreplies.Fields{
			Title: b.Title, Body: b.Body, Shortcut: b.Shortcut, ClearShortcut: requests.Nulls(in.RawBody)["shortcut"],
			Shared: b.Shared, Macro: macro,
		},
	})
	if err != nil {
		return nil, err
	}

	return &SavedReplyOutput{Body: fhttp.NewResource(resources.NewSavedReply(row))}, nil
}

/*
Destroy removes a reply.
*/
func (c *SavedReplyController) Destroy(ctx context.Context, in *requests.SavedReplyRequest) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixSavedReply), in.Reply)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	err = c.Remove.Handle(ctx, savedreplies.DeleteSavedReplyInput{Membership: membership(ctx), ID: id})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
StoreImport adds every row's reply, all or none, and answers with them.
*/
func (c *SavedReplyController) StoreImport(
	ctx context.Context, in *requests.StoreSavedReplyImportRequest,
) (*SavedRepliesOutput, error) {
	rows := make([]savedreplies.ImportRow, 0, len(in.Body.Replies))
	for _, r := range in.Body.Replies {
		rows = append(rows, savedreplies.ImportRow{
			Title: r.Title, Body: r.Body, Shortcut: r.Shortcut, Shared: r.Shared,
		})
	}

	created, err := c.Import.Handle(ctx, savedreplies.ImportSavedRepliesInput{Membership: membership(ctx), Rows: rows})
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.SavedReply]{
		Items: resources.NewSavedReplies(created), Limit: savedreplies.MaxImported,
	}

	return &SavedRepliesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
macro decodes the ids a macro names; one that is not an id of its kind
makes the macro invalid.
*/
func (c *SavedReplyController) macro(in *requests.SavedReplyMacroInput) (*savedreplies.Macro, error) {
	if in == nil {
		return nil, nil
	}

	out := savedreplies.Macro{Status: in.Status, SnoozePreset: in.SnoozePreset, Priority: in.Priority}

	var err error
	if out.AssigneeID, err = c.optional(models.PrefixMember, in.AssigneeID); err != nil {
		return nil, err
	}

	if out.TeamID, err = c.optional(models.PrefixTeam, in.TeamID); err != nil {
		return nil, err
	}

	for _, public := range in.TagIDs {
		id, err := c.IDs.Decode(string(models.PrefixTag), public)
		if err != nil {
			return nil, exceptions.ErrInvalidMacro
		}

		out.TagIDs = append(out.TagIDs, id)
	}

	return &out, nil
}

/*
optional decodes an id a macro may leave out.
*/
func (c *SavedReplyController) optional(prefix models.Prefix, public *string) (*ids.UUID, error) {
	if public == nil || *public == "" {
		return nil, nil
	}

	id, err := c.IDs.Decode(string(prefix), *public)
	if err != nil {
		return nil, exceptions.ErrInvalidMacro
	}

	return &id, nil
}
