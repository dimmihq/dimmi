/*
|--------------------------------------------------------------------------
| User Controller
|--------------------------------------------------------------------------
|
| Signing up: POST /users creates the account, optionally its first
| workspace, and signs the person in, answering like a sign-in does.
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
UserController serves the user collection.
*/
type UserController struct {
	RegisterUser *auth.RegisterUser
	Cookies      appauth.Cookies
	Server       Server
}

/*
Store registers a user and answers with their picture.
*/
func (c *UserController) Store(ctx context.Context, in *requests.StoreUserRequest) (*MeOutput, error) {
	signedIn, err := c.RegisterUser.Handle(ctx, auth.RegisterUserInput{
		Name:            in.Body.Name,
		Email:           in.Body.Email,
		Password:        in.Body.Password,
		WorkspaceName:   in.Body.WorkspaceName,
		InvitationToken: in.Body.InvitationToken,
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
