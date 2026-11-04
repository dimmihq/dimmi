/*
|--------------------------------------------------------------------------
| Show Widget Config Controller
|--------------------------------------------------------------------------
|
| GET /widget/v1/apps/{app}/config, the one request every page load makes.
| It is public and CDN cacheable: the entity tag names the widget, its
| version and the language, so a revalidation answers with no body.
|
*/

package widget

import (
	"context"
	"fmt"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
configCacheControl lets browsers and CDNs keep a config for a minute and
serve a stale one while revalidating.
*/
const configCacheControl = "public, max-age=60, s-maxage=60, stale-while-revalidate=600"

/*
ConfigOutput is a widget's config with its caching headers.
*/
type ConfigOutput struct {
	Status       int
	ETag         string `header:"ETag"`
	CacheControl string `header:"Cache-Control"`
	Body         *fhttp.Resource[actions.Config]
}

/*
ShowWidgetConfigController answers with a widget's config. BaseURL returns
the base address a request reached this install on.
*/
type ShowWidgetConfigController struct {
	Show    *actions.ShowWidgetConfig
	BaseURL func(ctx context.Context) string
}

/*
Invoke answers with the config, or with no body when the browser already
holds this version.
*/
func (c *ShowWidgetConfigController) Invoke(ctx context.Context, in *requests.ShowWidgetConfigRequest) (*ConfigOutput, error) {
	resolved, _ := actions.CurrentResolved(ctx)

	config, err := c.Show.Handle(ctx, actions.ShowWidgetConfigInput{
		Resolved: resolved, Locale: in.Locale, BaseURL: c.BaseURL(ctx),
	})
	if err != nil {
		return nil, err
	}

	out := &ConfigOutput{
		Status: fhttp.StatusOK, ETag: fmt.Sprintf(`"%s:%d:%s"`, config.AppID, config.ConfigVersion, config.Locale),
		CacheControl: configCacheControl,
	}

	if in.IfNoneMatch == out.ETag {
		out.Status = fhttp.StatusNotModified

		return out, nil
	}

	body := fhttp.NewResource(config)
	out.Body = &body

	return out, nil
}
