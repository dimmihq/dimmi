/*
|--------------------------------------------------------------------------
| Conversation View Controller
|--------------------------------------------------------------------------
|
| The saved views of the inbox: everyone lists the shared views and their
| own, creates personal views, and with views:manage_shared creates,
| orders and deletes shared ones. All of them fit on one page.
|
*/

package v1

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	conversationviews "github.com/dimmihq/dimmi/app/actions/conversation_views"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxViewsListed is the most views one caller sees: every shared view and
all their own.
*/
const maxViewsListed = 250

/*
ConversationViewsOutput is the views a caller sees.
*/
type ConversationViewsOutput struct {
	Body fhttp.Collection[resources.ConversationView]
}

/*
ConversationViewOutput is one view.
*/
type ConversationViewOutput struct {
	Body fhttp.Resource[resources.ConversationView]
}

/*
ConversationViewController serves the saved views.
*/
type ConversationViewController struct {
	List    *conversationviews.ListViews
	Show    *conversationviews.ShowView
	Create  *conversationviews.CreateView
	Update  *conversationviews.UpdateView
	Delete  *conversationviews.DeleteView
	Reorder *conversationviews.ReorderViews
	IDs     *ids.Registry
}

/*
Index lists the shared views in order, then the caller's own, those
whose name holds q when it is sent.
*/
func (c *ConversationViewController) Index(
	ctx context.Context, _ *requests.NoInput,
) (*ConversationViewsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	views, err := c.List.Handle(ctx, conversationviews.ListInput{
		Membership: membership(ctx), Search: strings.TrimSpace(r.URL.Query().Get("q")),
	})
	if err != nil {
		return nil, err
	}

	return viewCollection(views), nil
}

/*
Get answers with one view.
*/
func (c *ConversationViewController) Get(
	ctx context.Context, in *requests.ConversationViewRequest,
) (*ConversationViewOutput, error) {
	id, err := c.viewID(in.View)
	if err != nil {
		return nil, err
	}

	v, err := c.Show.Handle(ctx, conversationviews.ShowInput{Membership: membership(ctx), ID: id})
	if err != nil {
		return nil, err
	}

	return &ConversationViewOutput{Body: fhttp.NewResource(resources.NewConversationView(v))}, nil
}

/*
Store creates a view.
*/
func (c *ConversationViewController) Store(
	ctx context.Context, in *requests.StoreConversationViewRequest,
) (*ConversationViewOutput, error) {
	b := in.Body

	v, err := c.Create.Handle(ctx, conversationviews.CreateInput{
		Membership: membership(ctx), Scope: b.Scope,
		Fields: conversationviews.Fields{
			Name: &b.Name, Icon: b.Icon, Definition: b.Definition, Sort: b.Sort, StatusFilter: b.StatusFilter,
		},
	})
	if err != nil {
		return nil, err
	}

	return &ConversationViewOutput{Body: fhttp.NewResource(resources.NewConversationView(v))}, nil
}

/*
Patch changes a view.
*/
func (c *ConversationViewController) Patch(
	ctx context.Context, in *requests.UpdateConversationViewRequest,
) (*ConversationViewOutput, error) {
	id, err := c.viewID(in.View)
	if err != nil {
		return nil, err
	}

	b := in.Body

	v, err := c.Update.Handle(ctx, conversationviews.UpdateInput{
		Membership: membership(ctx), ID: id,
		Fields: conversationviews.Fields{
			Name: b.Name, Icon: b.Icon, Definition: b.Definition, Sort: b.Sort, StatusFilter: b.StatusFilter,
		},
	})
	if err != nil {
		return nil, err
	}

	return &ConversationViewOutput{Body: fhttp.NewResource(resources.NewConversationView(v))}, nil
}

/*
Destroy moves a view to the trash.
*/
func (c *ConversationViewController) Destroy(
	ctx context.Context, in *requests.ConversationViewRequest,
) (*EmptyOutput, error) {
	id, err := c.viewID(in.View)
	if err != nil {
		return nil, err
	}

	if err := c.Delete.Handle(ctx, conversationviews.ShowInput{Membership: membership(ctx), ID: id}); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
ReorderShared orders the shared views and answers with every view the
caller sees.
*/
func (c *ConversationViewController) ReorderShared(
	ctx context.Context, in *requests.ReorderConversationViewsRequest,
) (*ConversationViewsOutput, error) {
	order := make([]ids.UUID, len(in.Body.Order))

	for i, named := range in.Body.Order {
		id, err := c.viewID(named)
		if err != nil {
			return nil, err
		}

		order[i] = id
	}

	views, err := c.Reorder.Handle(ctx, conversationviews.ReorderInput{Membership: membership(ctx), Order: order})
	if err != nil {
		return nil, err
	}

	return viewCollection(views), nil
}

/*
viewID decodes a view's public id; text that is none is not found.
*/
func (c *ConversationViewController) viewID(named string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixConversationView), named)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}

func viewCollection(views []conversationviews.View) *ConversationViewsOutput {
	page := pagination.Page[resources.ConversationView]{
		Items: resources.NewConversationViews(views), Limit: maxViewsListed,
	}

	return &ConversationViewsOutput{Body: fhttp.NewCollection(page)}
}
