/*
|--------------------------------------------------------------------------
| User Confirmation Controller
|--------------------------------------------------------------------------
|
| The caller's proof of who they are, for sensitive steps: GET says
| whether a recent one holds, POST makes a new one.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
ConfirmationOutput answers with the caller's confirmation.
*/
type ConfirmationOutput struct {
	Body fhttp.Resource[resources.Confirmation]
}

/*
UserConfirmationController serves the caller's confirmation.
*/
type UserConfirmationController struct {
	Check   *auth.CheckConfirmation
	Confirm *auth.ConfirmUser
	Cookies appauth.Cookies
}

/*
Show answers with the confirmation that holds now.
*/
func (c *UserConfirmationController) Show(ctx context.Context, _ *requests.NoInput) (*ConfirmationOutput, error) {
	confirmation, err := c.Check.Handle(ctx, caller(ctx))
	if err != nil {
		return nil, err
	}

	return &ConfirmationOutput{Body: fhttp.NewResource(resources.NewConfirmation(confirmation))}, nil
}

/*
Store confirms the caller with the method.
*/
func (c *UserConfirmationController) Store(
	ctx context.Context, in *requests.StoreUserConfirmationRequest,
) (*ConfirmationOutput, error) {
	confirmation, err := c.Confirm.Handle(ctx, auth.ConfirmUserInput{
		Principal: caller(ctx), Method: in.Body.Method, Password: in.Body.Password, Code: in.Body.Code,
		Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &ConfirmationOutput{Body: fhttp.NewResource(resources.NewConfirmation(confirmation))}, nil
}
