/*
|--------------------------------------------------------------------------
| Feedback Status Controller
|--------------------------------------------------------------------------
|
| The statuses feature requests move through, in the order the workspace
| shows them on its portal and in the app, and the change of that order.
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
