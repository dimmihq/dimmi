/*
|--------------------------------------------------------------------------
| User Controller
|--------------------------------------------------------------------------
|
| Signing up: POST /users creates the account, optionally its first
| workspace, and signs the person in, answering like a sign-in does. An
| install that publishes legal terms refuses a sign-up that did not
| accept them.
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
	"github.com/dimmihq/dimmi/config"
)

/*
UserController serves the user collection. Terms is the legal terms of
the install, asked of every sign-up when it publishes them.
*/
type UserController struct {
	RegisterUser *auth.RegisterUser
	Terms        config.TermsConfig
	Cookies      appauth.Cookies
	Server       Server
}

/*
Store registers a user and answers with their picture.
*/
func (c *UserController) Store(ctx context.Context, in *requests.StoreUserRequest) (*MeOutput, error) {
	if c.Terms.SiteURL != "" {
		if err := in.TermsAccepted(ctx); err != nil {
			return nil, err
		}
	}

	signedIn, err := c.RegisterUser.Handle(ctx, auth.RegisterUserInput{
		Name:            in.Body.Name,
		Email:           in.Body.Email,
		Password:        in.Body.Password,
		WorkspaceName:   in.Body.WorkspaceName,
		InvitationToken: in.Body.InvitationToken,
		CaptchaToken:    in.Body.CaptchaToken,
		TermsAccepted:   in.Body.TermsAccepted,
		Client:          clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &MeOutput{
		SetCookie: deviceCookies(c.Cookies, signedIn.DeviceToken),
		Body:      fhttp.NewResource(resources.NewMe(ctx, signedIn.Me, c.Server.instance())),
	}, nil
}
