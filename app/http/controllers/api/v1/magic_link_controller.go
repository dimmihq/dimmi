/*
|--------------------------------------------------------------------------
| Magic Link Controller
|--------------------------------------------------------------------------
|
| POST /magic-links mails a sign-in link and code. It always answers the
| same, so it never tells whether the address has an account.
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
MessageOutput answers with a message for a person, under the status
the action decided on.
*/
type MessageOutput struct {
	Status int
	Body   fhttp.Resource[resources.Message]
}

/*
accepted answers that the request was taken, with the message under
the key.
*/
func accepted(ctx context.Context, key string) *MessageOutput {
	return &MessageOutput{
		Status: fhttp.StatusAccepted,
		Body:   fhttp.NewResource(resources.Message{Message: i18n.T(ctx, key, nil)}),
	}
}

/*
MagicLinkController serves the magic link collection.
*/
type MagicLinkController struct {
	SendMagicLink *auth.SendMagicLink
	Cookies       appauth.Cookies
}

/*
Store mails a sign-in link when the address may sign in.
*/
func (c *MagicLinkController) Store(ctx context.Context, in *requests.StoreMagicLinkRequest) (*MessageOutput, error) {
	err := c.SendMagicLink.Handle(ctx, auth.SendMagicLinkInput{
		Email: in.Body.Email, Name: in.Body.Name, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return accepted(ctx, "auth.magic_link_sent"), nil
}
