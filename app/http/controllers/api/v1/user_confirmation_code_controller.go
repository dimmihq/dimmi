/*
|--------------------------------------------------------------------------
| User Confirmation Code Controller
|--------------------------------------------------------------------------
|
| POST /user/confirmation-codes mails a code to a user without a
| password, who confirms who they are with it.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
UserConfirmationCodeController serves the caller's confirmation codes.
*/
type UserConfirmationCodeController struct {
	SendConfirmCode *auth.SendConfirmCode
	Cookies         appauth.Cookies
}

/*
Store mails the caller a confirmation code.
*/
func (c *UserConfirmationCodeController) Store(ctx context.Context, _ *requests.NoInput) (*MessageOutput, error) {
	if err := c.SendConfirmCode.Handle(ctx, caller(ctx), clientOf(ctx, c.Cookies)); err != nil {
		return nil, err
	}

	return accepted(ctx, "auth.confirm_code_sent"), nil
}
