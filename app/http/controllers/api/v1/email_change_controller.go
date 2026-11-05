/*
|--------------------------------------------------------------------------
| Email Change Controller
|--------------------------------------------------------------------------
|
| POST /email-changes mails a link to a new address. PATCH
| /email-changes/{change} redeems the link from any device, signed in or
| not: the token is the proof.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/users"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
EmailChangeOutput answers with the move to a new address.
*/
type EmailChangeOutput struct {
	Body fhttp.Resource[resources.EmailChange]
}

/*
EmailChangeController serves the email change collection.
*/
type EmailChangeController struct {
	Request *users.RequestEmailChange
	Confirm *users.ConfirmEmailChange
	Cookies appauth.Cookies
}

/*
Store starts a move to a new address.
*/
func (c *EmailChangeController) Store(
	ctx context.Context, in *requests.StoreEmailChangeRequest,
) (*EmailChangeOutput, error) {
	change, err := c.Request.Handle(ctx, users.RequestEmailChangeInput{
		User: caller(ctx).User, Email: in.Body.Email, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &EmailChangeOutput{Body: fhttp.NewResource(resources.NewEmailChange(change))}, nil
}

/*
Update finishes the move with the link's token.
*/
func (c *EmailChangeController) Update(
	ctx context.Context, in *requests.UpdateEmailChangeRequest,
) (*EmailChangeOutput, error) {
	change, err := c.Confirm.Handle(ctx, users.ConfirmEmailChangeInput{
		ID: in.Change, Token: in.Body.Token, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &EmailChangeOutput{Body: fhttp.NewResource(resources.NewEmailChange(change))}, nil
}
