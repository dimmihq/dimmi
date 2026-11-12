/*
|--------------------------------------------------------------------------
| Widget Article View Controller
|--------------------------------------------------------------------------
|
| The messenger carries its help center's articles at once and opens
| them without asking again, so it tells when a visitor opened one. The
| view is counted beside the help center's own page views, as a count
| for the day and nothing about the visitor.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
View counts one view of the article in the messenger. An article the
help center does not show is not counted, and is no error.
*/
func (c *HelpController) View(ctx context.Context, in *requests.StoreWidgetArticleViewRequest) (*struct{}, error) {
	auth := authenticated(ctx)
	if auth.Resolved.Widget.HelpCenterID == nil || c.Public == nil {
		return nil, ErrNoHelpCenter
	}

	snap, err := c.Public.LoadByID(ctx, auth.Resolved.Workspace.ID, *auth.Resolved.Widget.HelpCenterID)
	if err != nil {
		return nil, ErrNoHelpCenter
	}

	for _, a := range snap.Articles {
		if a.Slug == in.Body.Article {
			c.Analytics.Viewed(ctx, snap.HelpCenter, a.ID, helpcenters.ViewInMessenger)
			break
		}
	}

	return &struct{}{}, nil
}
