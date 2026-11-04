/*
|--------------------------------------------------------------------------
| Password Reset Controller
|--------------------------------------------------------------------------
|
| POST /password-resets mails a reset link, answering the same for every
| email. PATCH /password-resets/{reset} redeems the link with a new
| password and signs the user in.
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
PasswordResetController serves the password reset collection.
*/
type PasswordResetController struct {
	SendPasswordResetLink *auth.SendPasswordResetLink
	ResetPassword         *auth.ResetPassword
	Cookies               appauth.Cookies
	Server                Server
}

/*
Store mails a reset link when the email has an account.
*/
func (c *PasswordResetController) Store(
	ctx context.Context, in *requests.StorePasswordResetRequest,
) (*MessageOutput, error) {
	err := c.SendPasswordResetLink.Handle(ctx, auth.SendPasswordResetLinkInput{
		Email: in.Body.Email, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return accepted(ctx, "auth.password_reset_sent"), nil
}

/*
Update sets the new password and answers with the signed-in caller, or
with the second-factor challenge the account asks for.
*/
func (c *PasswordResetController) Update(
	ctx context.Context, in *requests.UpdatePasswordResetRequest,
) (*SignInOutput, error) {
	signedIn, err := c.ResetPassword.Handle(ctx, auth.ResetPasswordInput{
		ID: in.Reset, Token: in.Body.Token, Password: in.Body.Password, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return signInOutput(ctx, c.Cookies, c.Server, signedIn), nil
}
