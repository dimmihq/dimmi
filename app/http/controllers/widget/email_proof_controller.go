/*
|--------------------------------------------------------------------------
| Widget Email Proof Controller
|--------------------------------------------------------------------------
|
| An anonymous visitor proves they own an email address: a code goes to
| it, and the code comes back from the messenger's card. The answers
| repeat the address only masked.
|
*/

package widget

import (
	"context"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
EmailProofController sends and checks email codes.
*/
type EmailProofController struct {
	Proofs *actions.EmailProofs
}

/*
EmailCodeSent is a code on its way to the masked address, when it stops
working and when another may be asked for.
*/
type EmailCodeSent struct {
	Object      string    `json:"object"`
	Email       string    `json:"email"`
	ExpiresAt   time.Time `json:"expires_at"`
	ResendAfter time.Time `json:"resend_after"`
}

/*
EmailCodeSentOutput answers a code sent.
*/
type EmailCodeSentOutput struct {
	Status int
	Body   fhttp.Resource[EmailCodeSent]
}

/*
EmailProven is the session's proof: the masked address and when.
*/
type EmailProven struct {
	Object     string     `json:"object"`
	Email      string     `json:"email"`
	VerifiedAt *time.Time `json:"verified_at"`
}

/*
EmailProvenOutput answers a good code.
*/
type EmailProvenOutput struct {
	Body fhttp.Resource[EmailProven]
}

/*
Store mails a code to the address.
*/
func (c *EmailProofController) Store(ctx context.Context, in *requests.SendWidgetEmailCodeRequest,
) (*EmailCodeSentOutput, error) {
	sent, err := c.Proofs.Send(ctx, authenticated(ctx), in.Body.Email)
	if err != nil {
		return nil, err
	}
	return &EmailCodeSentOutput{Status: fhttp.StatusAccepted, Body: fhttp.NewResource(EmailCodeSent{
		Object: "email_verification", Email: sent.Email, ExpiresAt: sent.ExpiresAt, ResendAfter: sent.ResendAfter,
	})}, nil
}

/*
Check takes the code the visitor typed.
*/
func (c *EmailProofController) Check(ctx context.Context, in *requests.CheckWidgetEmailCodeRequest,
) (*EmailProvenOutput, error) {
	session, err := c.Proofs.Check(ctx, authenticated(ctx), in.Body.Code)
	if err != nil {
		return nil, err
	}
	email := ""
	if session.VerifiedEmail != nil {
		email = actions.MaskEmail(*session.VerifiedEmail)
	}
	return &EmailProvenOutput{Body: fhttp.NewResource(EmailProven{
		Object: "email_verification", Email: email, VerifiedAt: session.EmailVerifiedAt,
	})}, nil
}
