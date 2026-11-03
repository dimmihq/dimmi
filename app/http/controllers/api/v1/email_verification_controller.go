/*
|--------------------------------------------------------------------------
| Email Verification Controller
|--------------------------------------------------------------------------
|
| POST /email-verifications mails the signed-in user a new link.
| PATCH /email-verifications/{verification} redeems a link from any
| device, signed in or not.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
EmailVerificationOutput answers with the verified address.
*/
type EmailVerificationOutput struct {
	Body fhttp.Resource[resources.EmailVerification]
}

/*
EmailVerificationController serves the email verification collection.
*/
type EmailVerificationController struct {
	Resend  *auth.ResendEmailVerification
	Verify  *auth.VerifyEmail
	Cookies appauth.Cookies
}

/*
Store mails a new link, or says the address needs none.
*/
func (c *EmailVerificationController) Store(ctx context.Context, _ *requests.NoInput) (*MessageOutput, error) {
	sent, err := c.Resend.Handle(ctx, caller(ctx), clientOf(ctx, c.Cookies))
	if err != nil {
		return nil, err
	}

	if !sent {
		return &MessageOutput{
			Status: fhttp.StatusOK,
			Body:   fhttp.NewResource(resources.Message{Message: i18n.T(ctx, "auth.already_verified", nil)}),
		}, nil
	}

	return accepted(ctx, "auth.verification_sent"), nil
}

/*
Update verifies the address the link names.
*/
func (c *EmailVerificationController) Update(
	ctx context.Context, in *requests.UpdateEmailVerificationRequest,
) (*EmailVerificationOutput, error) {
	verification, err := c.Verify.Handle(ctx, auth.VerifyEmailInput{
		ID: in.Verification, Token: in.Body.Token, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &EmailVerificationOutput{Body: fhttp.NewResource(resources.NewEmailVerification(verification))}, nil
}
