/*
|--------------------------------------------------------------------------
| Retention Preview Controller
|--------------------------------------------------------------------------
|
| POST /retention-previews says how many closed conversations a period
| would move to the trash, so the setting can warn before it is saved.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/retention"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
RetentionPreview is how many conversations a period would move.
*/
type RetentionPreview struct {
	Object string `json:"object"`
	Count  int64  `json:"count"`
}

/*
RetentionPreviewOutput is the preview.
*/
type RetentionPreviewOutput struct {
	Body fhttp.Resource[RetentionPreview]
}

/*
RetentionController serves retention previews.
*/
type RetentionController struct {
	Preview *retention.PreviewRetention
}

/*
Store counts the conversations the period would move.
*/
func (c *RetentionController) Store(
	ctx context.Context, in *requests.RetentionPreviewRequest,
) (*RetentionPreviewOutput, error) {
	count, err := c.Preview.Handle(ctx, retention.PreviewRetentionInput{
		Membership: membership(ctx), ClosedConversationDays: in.Body.ClosedConversationDays,
	})
	if err != nil {
		return nil, err
	}

	return &RetentionPreviewOutput{
		Body: fhttp.NewResource(RetentionPreview{Object: "retention_preview", Count: count}),
	}, nil
}
