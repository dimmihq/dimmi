/*
|--------------------------------------------------------------------------
| Widget Article Rating Controller
|--------------------------------------------------------------------------
|
| A visitor says whether an article they read in the messenger helped.
| The answer lands beside the ones readers give on the help center, kept
| under the visitor's contact, so answering again changes it rather than
| adding another.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
Rate keeps the visitor's answer about the article. A known visitor
answers as the reader the help center knows, so their answer is the same
one they give there; anyone else answers as their own contact.
*/
func (c *HelpController) Rate(ctx context.Context, in *requests.StoreWidgetArticleRatingRequest) (*struct{}, error) {
	auth := authenticated(ctx)
	if auth.Resolved.Widget.HelpCenterID == nil || c.Public == nil || c.Ratings == nil {
		return nil, ErrNoHelpCenter
	}

	snap, err := c.Public.LoadByID(ctx, auth.Resolved.Workspace.ID, *auth.Resolved.Widget.HelpCenterID)
	if err != nil {
		return nil, ErrNoHelpCenter
	}

	var contact *ids.UUID
	if r, ok := c.known(auth); ok {
		contact = r.Contact
		if contact == nil {
			contact = c.readsAs(ctx, auth, r.Email)
		}
	}

	rater := "contact:" + auth.Session.ContactID.String()
	if contact != nil {
		rater = "contact:" + contact.String()
	}

	err = c.Ratings.Rate(ctx, helpcenters.Rating{
		Center: snap.HelpCenter, Slug: in.Body.Article, Rater: rater, Contact: contact, Helpful: in.Body.Helpful,
	})
	if err != nil {
		return nil, err
	}

	return &struct{}{}, nil
}
