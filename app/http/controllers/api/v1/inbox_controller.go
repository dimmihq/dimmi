/*
|--------------------------------------------------------------------------
| Inbox Controller
|--------------------------------------------------------------------------
|
| GET /inboxes lists the inboxes of the workspace in play, GET
| /inboxes/{inbox} shows one, POST /inboxes adds one with its API
| channel and PATCH /inboxes/{inbox} renames it. Ids of other
| workspaces' inboxes are simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxInboxes is the most inboxes a workspace lists, all on one page.
*/
const maxInboxes = 200

/*
InboxesOutput is the workspace's inboxes.
*/
type InboxesOutput struct {
	Body fhttp.Collection[resources.Inbox]
}

/*
InboxOutput is one inbox.
*/
type InboxOutput struct {
	Body fhttp.Resource[resources.Inbox]
}

/*
InboxController serves the inboxes of the workspace in play.
*/
type InboxController struct {
	List   *inboxes.ListInboxes
	Show   *inboxes.ShowInbox
	Create *inboxes.CreateInbox
	Change *inboxes.UpdateInbox
	IDs    *ids.Registry
}

/*
Index lists the inboxes, oldest first, on one page.
*/
func (c *InboxController) Index(ctx context.Context, _ *requests.NoInput) (*InboxesOutput, error) {
	rows, err := c.List.Handle(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.Inbox]{Items: resources.NewInboxes(rows), Limit: maxInboxes}

	return &InboxesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Get answers with one inbox.
*/
func (c *InboxController) Get(ctx context.Context, in *requests.ShowInboxRequest) (*InboxOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixInbox), in.Inbox)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	inbox, err := c.Show.Handle(ctx, inboxes.ShowInboxInput{Membership: membership(ctx), InboxID: id})
	if err != nil {
		return nil, err
	}

	return &InboxOutput{Body: fhttp.NewResource(resources.NewInbox(inbox))}, nil
}

/*
Store adds an inbox and answers with it.
*/
func (c *InboxController) Store(ctx context.Context, in *requests.StoreInboxRequest) (*InboxOutput, error) {
	inbox, err := c.Create.Handle(ctx, inboxes.CreateInboxInput{Membership: membership(ctx), Name: in.Body.Name})
	if err != nil {
		return nil, err
	}

	return &InboxOutput{Body: fhttp.NewResource(resources.NewInbox(inbox))}, nil
}

/*
Update renames an inbox and answers with it as it now is.
*/
func (c *InboxController) Update(ctx context.Context, in *requests.UpdateInboxRequest) (*InboxOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixInbox), in.Inbox)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	inbox, err := c.Change.Handle(ctx, inboxes.UpdateInboxInput{
		Membership: membership(ctx), InboxID: id, Name: in.Body.Name,
	})
	if err != nil {
		return nil, err
	}

	return &InboxOutput{Body: fhttp.NewResource(resources.NewInbox(inbox))}, nil
}
