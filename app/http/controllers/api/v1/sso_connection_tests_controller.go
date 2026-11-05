/*
|--------------------------------------------------------------------------
| SSO Connection Test Controller
|--------------------------------------------------------------------------
|
| POST /sso-connections/{connection}/tests starts an owner's test sign-in
| and answers with the identity provider's address to open in a new tab.
| The browser gets the attempt cookie with the answer, so the identity
| provider's reply comes back to this browser only.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ssologin"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SSOConnectionTest is where the owner signs in to test a connection.
*/
type SSOConnectionTest struct {
	Object string `json:"object"`
	URL    string `json:"url"`
}

/*
SSOConnectionTestOutput answers with the address, and hands the browser
its attempt cookie.
*/
type SSOConnectionTestOutput struct {
	SetCookie []fhttp.Cookie `header:"Set-Cookie"`
	Body      fhttp.Resource[SSOConnectionTest]
}

/*
SSOConnectionTestController starts test sign-ins.
*/
type SSOConnectionTestController struct {
	Start   *ssologin.StartTest
	IDs     *ids.Registry
	Cookies appauth.Cookies
}

/*
Store starts a test sign-in through the connection.
*/
func (c *SSOConnectionTestController) Store(
	ctx context.Context, in *requests.ShowSSOConnectionRequest,
) (*SSOConnectionTestOutput, error) {
	id, err := decodeID(c.IDs, models.PrefixSSOConnection, in.Connection)
	if err != nil {
		return nil, err
	}

	started, err := c.Start.Handle(ctx, ssologin.StartTestInput{
		Membership: membership(ctx), UserID: caller(ctx).User.ID, ConnectionID: id,
	})
	if err != nil {
		return nil, err
	}

	return &SSOConnectionTestOutput{
		SetCookie: []fhttp.Cookie{c.Cookies.SSOCookie(started.Browser, appauth.SSOAttemptLifetime)},
		Body:      fhttp.NewResource(SSOConnectionTest{Object: "sso_connection_test", URL: started.URL}),
	}, nil
}
