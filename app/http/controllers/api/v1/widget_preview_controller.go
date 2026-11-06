/*
|--------------------------------------------------------------------------
| Widget Preview Controller
|--------------------------------------------------------------------------
|
| Answers with the config a widget would serve if the change in the body
| were saved, for the settings page's live preview. Nothing is written.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
WidgetPreviewConfig is a widget's config as a browser would receive it,
named apart from the other configs the API describes.
*/
type WidgetPreviewConfig widget.Config

/*
WidgetConfigOutput is the previewed config.
*/
type WidgetConfigOutput struct {
	Body fhttp.Resource[WidgetPreviewConfig]
}

/*
PreviewConfig answers with the widget's config with the change applied.
*/
func (c *WidgetSettingsController) PreviewConfig(
	ctx context.Context, in *requests.PreviewWidgetSettingsRequest,
) (*WidgetConfigOutput, error) {
	id, err := c.widgetID(in.Widget)
	if err != nil {
		return nil, err
	}

	change, err := c.change(in.Body, in.RawBody)
	if err != nil {
		return nil, err
	}

	config, err := c.Preview.Handle(ctx, widget.PreviewWidgetConfigInput{
		Membership: membership(ctx), WidgetID: id, Change: change, Locale: in.Locale, BaseURL: c.Base,
	})
	if err != nil {
		return nil, err
	}

	return &WidgetConfigOutput{Body: fhttp.NewResource(WidgetPreviewConfig(config))}, nil
}
