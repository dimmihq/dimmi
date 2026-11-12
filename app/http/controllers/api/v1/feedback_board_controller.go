/*
|--------------------------------------------------------------------------
| Feedback Board Controller
|--------------------------------------------------------------------------
|
| A help center's boards of feature requests: listed in the team's
| order with how many requests each holds, added, changed, removed with
| their requests moved onto another board, and put in a new order.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
FeedbackBoardController serves a help center's boards.
*/
type FeedbackBoardController struct {
	Posts *feedback.Posts
	IDs   *ids.Registry
}

/*
FeedbackBoardsOutput is a help center's boards, first to last.
*/
type FeedbackBoardsOutput struct {
	Body fhttp.Resource[[]resources.FeedbackBoard]
}

/*
FeedbackBoardOutput is one board.
*/
type FeedbackBoardOutput struct {
	Body fhttp.Resource[resources.FeedbackBoard]
}

/*
Index lists a help center's boards.
*/
func (c *FeedbackBoardController) Index(ctx context.Context, in *requests.HelpCenterRequest,
) (*FeedbackBoardsOutput, error) {
	hc, err := c.ids().decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	return boardsOutput(c.Posts.Boards(ctx, membership(ctx), hc))
}

/*
Store adds a board.
*/
func (c *FeedbackBoardController) Store(ctx context.Context, in *requests.StoreFeedbackBoardRequest,
) (*FeedbackBoardOutput, error) {
	hc, err := c.ids().decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	return boardOutput(c.Posts.CreateBoard(ctx, membership(ctx), hc, feedback.BoardChange(in.Body)))
}

/*
Update changes a board.
*/
func (c *FeedbackBoardController) Update(ctx context.Context, in *requests.UpdateFeedbackBoardRequest,
) (*FeedbackBoardOutput, error) {
	id, err := c.ids().decode(models.PrefixFeedbackBoard, in.Board)
	if err != nil {
		return nil, err
	}
	return boardOutput(c.Posts.UpdateBoard(ctx, membership(ctx), id, feedback.BoardChange(in.Body)))
}

/*
Destroy removes a board, moving its requests onto another.
*/
func (c *FeedbackBoardController) Destroy(ctx context.Context, in *requests.DestroyFeedbackBoardRequest,
) (*struct{}, error) {
	id, err := c.ids().decode(models.PrefixFeedbackBoard, in.Board)
	if err != nil {
		return nil, err
	}
	moveTo, err := c.ids().optional(models.PrefixFeedbackBoard, in.MoveTo)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Posts.DeleteBoard(ctx, membership(ctx), id, moveTo)
}

/*
Reorder saves a new order of a help center's boards.
*/
func (c *FeedbackBoardController) Reorder(ctx context.Context, in *requests.ReorderFeedbackBoardsRequest,
) (*FeedbackBoardsOutput, error) {
	hc, err := c.ids().decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	order := make([]ids.UUID, len(in.Body.Boards))
	for i, public := range in.Body.Boards {
		if order[i], err = c.ids().decode(models.PrefixFeedbackBoard, public); err != nil {
			return nil, feedback.ErrBoardOrderInvalid
		}
	}
	return boardsOutput(c.Posts.ReorderBoards(ctx, membership(ctx), hc, order))
}

func (c *FeedbackBoardController) ids() *FeedbackController {
	return &FeedbackController{Posts: c.Posts, IDs: c.IDs}
}

func boardsOutput(list []feedback.Board, err error) (*FeedbackBoardsOutput, error) {
	if err != nil {
		return nil, err
	}
	return &FeedbackBoardsOutput{Body: fhttp.NewResource(resources.NewFeedbackBoards(list))}, nil
}

func boardOutput(b feedback.Board, err error) (*FeedbackBoardOutput, error) {
	if err != nil {
		return nil, err
	}
	return &FeedbackBoardOutput{Body: fhttp.NewResource(resources.NewFeedbackBoard(b))}, nil
}
