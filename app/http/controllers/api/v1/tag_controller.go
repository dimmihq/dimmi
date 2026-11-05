/*
|--------------------------------------------------------------------------
| Tag Controllers
|--------------------------------------------------------------------------
|
| GET /tags lists the workspace's vocabulary, POST /tags adds to it, and
| PATCH and DELETE /tags/{tag} rename, recolor and remove. On a
| conversation, POST .../tags adds tags by id or name and DELETE
| .../tags/{tag} takes one off, as single moves beside the fields of a
| conversation's PATCH. Any public id that does not decode as a tag is
| read as a name when adding, and not found when acting on one tag.
|
*/

package v1

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The most tags a list may filter by.
*/
const (
	maxTagFilter    = 5
	maxAnyTagFilter = 20
	maxTagPage      = tags.MaxInWorkspace
)

/*
TagsOutput is the workspace's tags.
*/
type TagsOutput struct {
	Body fhttp.Collection[resources.ListedTag]
}

/*
TagOutput is one tag.
*/
type TagOutput struct {
	Body fhttp.Resource[resources.Tag]
}

/*
TagController serves the workspace's tags.
*/
type TagController struct {
	List   *tags.ListTags
	Create *tags.CreateTag
	Change *tags.UpdateTag
	Remove *tags.DeleteTag
	IDs    *ids.Registry
}

/*
Index lists the tags by name: q narrows to names starting with it, and
include=counts adds how many records carry each.
*/
func (c *TagController) Index(ctx context.Context, _ *requests.NoInput) (*TagsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)
	values := r.URL.Query()
	in := tags.ListTagsInput{Membership: membership(ctx), Counts: values.Get("include") == "counts"}

	if q := strings.TrimSpace(values.Get("q")); q != "" {
		in.Prefix = &q
	}

	rows, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	items := resources.NewListedTags(rows)
	if in.Counts {
		items = resources.NewCountedTags(rows)
	}

	page := pagination.Page[resources.ListedTag]{Items: items, Limit: maxTagPage}

	return &TagsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store adds a tag and answers with it.
*/
func (c *TagController) Store(ctx context.Context, in *requests.StoreTagRequest) (*TagOutput, error) {
	tag, err := c.Create.Handle(ctx, tags.CreateTagInput{
		Membership: membership(ctx), Name: in.Body.Name, Color: in.Body.Color, Icon: in.Body.Icon,
	})
	if err != nil {
		return nil, err
	}

	return &TagOutput{Body: fhttp.NewResource(resources.NewTag(tag))}, nil
}

/*
Update renames a tag or changes its color or icon, and answers with it
as it now is.
*/
func (c *TagController) Update(ctx context.Context, in *requests.UpdateTagRequest) (*TagOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixTag), in.Tag)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	nulls := requests.Nulls(in.RawBody)

	tag, err := c.Change.Handle(ctx, tags.UpdateTagInput{
		Membership: membership(ctx), TagID: id, Name: in.Body.Name, Color: in.Body.Color,
		ClearColor: nulls["color"], Icon: in.Body.Icon, ClearIcon: nulls["icon"],
	})
	if err != nil {
		return nil, err
	}

	return &TagOutput{Body: fhttp.NewResource(resources.NewTag(tag))}, nil
}

/*
Destroy removes a tag from the vocabulary and every record.
*/
func (c *TagController) Destroy(ctx context.Context, in *requests.TagRequest) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixTag), in.Tag)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	if err := c.Remove.Handle(ctx, tags.DeleteTagInput{Membership: membership(ctx), TagID: id}); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
ConversationTagController puts tags on a conversation and takes them
off.
*/
type ConversationTagController struct {
	Tag   *conversations.TagConversation
	Untag *conversations.UntagConversation
	IDs   *ids.Registry
}

/*
Store adds tags and answers with the conversation as it now is.
*/
func (c *ConversationTagController) Store(
	ctx context.Context, in *requests.StoreConversationTagsRequest,
) (*ConversationOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	message, err := decodeField(c.IDs, models.PrefixMessage, in.Body.MessageID,
		exceptions.ErrTagMessageNotInConversation)
	if err != nil {
		return nil, err
	}

	loaded, err := c.Tag.Handle(ctx, conversations.TagConversationInput{
		Membership: membership(ctx), Ref: ref, Tags: tagRefs(c.IDs, in.Body.Tags), MessageID: message,
		MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	return &ConversationOutput{Body: fhttp.NewResource(resources.NewConversation(loaded))}, nil
}

/*
Destroy takes a tag off and answers with the conversation as it now is.
A tag not on it changes nothing.
*/
func (c *ConversationTagController) Destroy(
	ctx context.Context, in *requests.ConversationTagRequest,
) (*ConversationOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	id, err := c.IDs.Decode(string(models.PrefixTag), in.Tag)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	loaded, err := c.Untag.Handle(ctx, conversations.UntagConversationInput{
		Membership: membership(ctx), Ref: ref, TagID: id, MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	return &ConversationOutput{Body: fhttp.NewResource(resources.NewConversation(loaded))}, nil
}

/*
tagRefs splits what a request names into tag ids and names.
*/
func tagRefs(registry *ids.Registry, named []string) tags.Refs {
	var refs tags.Refs

	for _, value := range named {
		id, err := registry.Decode(string(models.PrefixTag), value)
		if err != nil {
			refs.Names = append(refs.Names, value)

			continue
		}

		refs.IDs = append(refs.IDs, id)
	}

	return refs
}

/*
decodeTagIDs reads tag ids, dropping what is no tag id: removing a tag
that cannot exist changes nothing.
*/
func decodeTagIDs(registry *ids.Registry, named []string) []ids.UUID {
	var out []ids.UUID

	for _, value := range named {
		if id, err := registry.Decode(string(models.PrefixTag), value); err == nil {
			out = append(out, id)
		}
	}

	return out
}

/*
decodeTagFilter reads a filter of up to limit tag ids.
*/
func decodeTagFilter(registry *ids.Registry, named []string, limit int) ([]ids.UUID, error) {
	if len(named) > limit {
		return nil, pagination.ErrInvalidFilter
	}

	out := make([]ids.UUID, 0, len(named))

	for _, value := range named {
		id, err := registry.Decode(string(models.PrefixTag), value)
		if err != nil {
			return nil, pagination.ErrInvalidFilter
		}

		out = append(out, id)
	}

	return out, nil
}
