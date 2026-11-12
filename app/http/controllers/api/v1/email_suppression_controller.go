/*
|--------------------------------------------------------------------------
| Email Suppression Controllers
|--------------------------------------------------------------------------
|
| GET /email-suppressions?filter[contact]= lists what stops mail to a
| customer; DELETE /email-suppressions/{email_suppression} lifts one.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailSuppressionsOutput is a contact's suppressions.
*/
type EmailSuppressionsOutput struct {
	Body fhttp.Collection[resources.EmailSuppression]
}

/*
EmailSuppressionController serves suppressed addresses.
*/
type EmailSuppressionController struct {
	List *email.ListContactSuppressions
	Lift *email.LiftSuppression
	IDs  *ids.Registry
}

/*
Index lists the suppressions on the contact's address.
*/
func (c *EmailSuppressionController) Index(
	ctx context.Context, in *requests.ListEmailSuppressionsRequest,
) (*EmailSuppressionsOutput, error) {
	contact, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	rows, err := c.List.Handle(ctx, membership(ctx), contact)
	if err != nil {
		return nil, err
	}

	items := resources.NewEmailSuppressions(rows)
	page := pagination.Page[resources.EmailSuppression]{Items: items, Limit: len(items)}

	return &EmailSuppressionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Destroy lifts a suppression.
*/
func (c *EmailSuppressionController) Destroy(
	ctx context.Context, in *requests.EmailSuppressionRequest,
) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixEmailSuppression), in.Suppression)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	if err := c.Lift.Handle(ctx, membership(ctx), id); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}
