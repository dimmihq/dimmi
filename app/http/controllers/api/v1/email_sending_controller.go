/*
|--------------------------------------------------------------------------
| Email Sending Controller
|--------------------------------------------------------------------------
|
| GET /email-sending answers whether the workspace's bulk email is
| paused and what it is judged by, and POST /email-sending/resumptions
| lets an owner resume it.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
EmailSendingOutput is the workspace's standing as a sender.
*/
type EmailSendingOutput struct {
	Body fhttp.Resource[resources.EmailSending]
}

/*
EmailSendingController serves the workspace's standing as a sender.
*/
type EmailSendingController struct {
	Reputation *email.Reputation
}

/*
Show answers the workspace's standing as a sender.
*/
func (c *EmailSendingController) Show(ctx context.Context, _ *requests.NoInput) (*EmailSendingOutput, error) {
	status, err := c.Reputation.Status(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &EmailSendingOutput{Body: fhttp.NewResource(resources.NewEmailSending(status))}, nil
}

/*
StoreResumption resumes the workspace's paused sending.
*/
func (c *EmailSendingController) StoreResumption(
	ctx context.Context, _ *requests.NoInput,
) (*EmailSendingOutput, error) {
	status, err := c.Reputation.Resume(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &EmailSendingOutput{Body: fhttp.NewResource(resources.NewEmailSending(status))}, nil
}
