/*
|--------------------------------------------------------------------------
| Feedback Status Controller
|--------------------------------------------------------------------------
|
| The statuses feature requests move through, in the order the workspace
| shows them on its portal and in the app: listed, put in a new order,
| added, changed, and removed with their requests moved to another.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
FeedbackStatusController serves the workspace's status order.
*/
type FeedbackStatusController struct {
	Posts *feedback.Posts
}

/*
FeedbackStatusesOutput is every status, first to last.
*/
type FeedbackStatusesOutput struct {
	Body fhttp.Resource[[]resources.FeedbackStatus]
}

/*
Index lists the statuses in the workspace's order.
*/
func (c *FeedbackStatusController) Index(ctx context.Context, _ *requests.NoInput) (*FeedbackStatusesOutput, error) {
	list, err := c.Posts.StatusOrder(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	return &FeedbackStatusesOutput{Body: fhttp.NewResource(resources.NewFeedbackStatuses(list))}, nil
}

/*
Reorder saves a new order of the statuses and returns them in it.
*/
func (c *FeedbackStatusController) Reorder(ctx context.Context, in *requests.ReorderFeedbackStatusesRequest,
) (*FeedbackStatusesOutput, error) {
	list, err := c.Posts.Reorder(ctx, membership(ctx), in.Body.Statuses)
	if err != nil {
		return nil, err
	}
	return &FeedbackStatusesOutput{Body: fhttp.NewResource(resources.NewFeedbackStatuses(list))}, nil
}

/*
FeedbackStatusOutput is one status.
*/
type FeedbackStatusOutput struct {
	Body fhttp.Resource[resources.FeedbackStatus]
}

/*
Store adds a status after the others.
*/
func (c *FeedbackStatusController) Store(ctx context.Context, in *requests.StoreFeedbackStatusRequest,
) (*FeedbackStatusOutput, error) {
	s, err := c.Posts.CreateStatus(ctx, membership(ctx), feedback.StatusChange(in.Body))
	if err != nil {
		return nil, err
	}
	return &FeedbackStatusOutput{Body: fhttp.NewResource(resources.NewFeedbackStatus(s))}, nil
}

/*
Update renames, recolors or recategorizes a status.
*/
func (c *FeedbackStatusController) Update(ctx context.Context, in *requests.UpdateFeedbackStatusRequest,
) (*FeedbackStatusOutput, error) {
	s, err := c.Posts.UpdateStatus(ctx, membership(ctx), in.Status, feedback.StatusChange(in.Body))
	if err != nil {
		return nil, err
	}
	return &FeedbackStatusOutput{Body: fhttp.NewResource(resources.NewFeedbackStatus(s))}, nil
}

/*
Destroy removes a status, moving its requests to another.
*/
func (c *FeedbackStatusController) Destroy(ctx context.Context, in *requests.DestroyFeedbackStatusRequest,
) (*struct{}, error) {
	return &struct{}{}, c.Posts.DeleteStatus(ctx, membership(ctx), in.Status, in.MoveTo)
}
