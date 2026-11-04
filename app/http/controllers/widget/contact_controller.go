/*
|--------------------------------------------------------------------------
| Widget Contact Controller
|--------------------------------------------------------------------------
|
| A visitor reads and, while anonymous, writes their own profile. The
| session decides whose profile it is; no id is ever sent.
|
*/

package widget

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ContactOutput is the visitor's profile.
*/
type ContactOutput struct {
	Body fhttp.Resource[resources.WidgetProfile]
}

/*
ContactController serves the visitor's own profile.
*/
type ContactController struct {
	DB     *database.DB
	Update *actions.UpdateWidgetContact
}

/*
Show answers with the visitor's profile.
*/
func (c *ContactController) Show(ctx context.Context, _ *requests.NoInput) (*ContactOutput, error) {
	session, _ := actions.CurrentSession(ctx)

	var contact models.Contact

	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		contact, err = models.New(tx).FindContact(ctx, models.FindContactParams{
			WorkspaceID: session.WorkspaceID, ID: session.ContactID,
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("find widget contact: %w", err)
	}

	return &ContactOutput{Body: fhttp.NewResource(resources.NewWidgetProfile(contact))}, nil
}

/*
Patch writes what the lead tells about themselves.
*/
func (c *ContactController) Patch(
	ctx context.Context, in *requests.UpdateWidgetContactRequest,
) (*ContactOutput, error) {
	session, _ := actions.CurrentSession(ctx)

	contact, err := c.Update.Handle(ctx, session, actions.ContactChange{
		Name: in.Body.Name, Email: in.Body.Email, Phone: in.Body.Phone, CustomAttributes: in.Body.CustomAttributes,
	})
	if err != nil {
		return nil, err
	}

	return &ContactOutput{Body: fhttp.NewResource(resources.NewWidgetProfile(contact))}, nil
}
