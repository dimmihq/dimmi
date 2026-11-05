/*
|--------------------------------------------------------------------------
| Sign-In Answers
|--------------------------------------------------------------------------
|
| Every endpoint that signs someone in answers one of two ways: the
| signed-in caller, or, for an account protected by a second factor, the
| challenge to answer first, with the cookie that ties this browser to
| it. Answering the challenge drops that cookie again.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
SignInOutput answers a sign-in: the caller, or a challenge under 200,
with the cookies to set.
*/
type SignInOutput struct {
	Status    int
	SetCookie []fhttp.Cookie `header:"Set-Cookie"`
	Body      any
}

/*
signInOutput shapes the answer to a sign-in, the caller or a challenge,
both under 200. A challenge sets its cookie; a session drops a
challenge cookie the browser may still hold.
*/
func signInOutput(ctx context.Context, cookies appauth.Cookies, server Server, signedIn auth.SignedIn) *SignInOutput {
	set := deviceCookies(cookies, signedIn.DeviceToken)

	if c := signedIn.Challenge; c != nil {
		set = append(set, cookies.TwoFactorCookie(c.Token, c.ExpiresAt.Sub(server.Clock.Now())))

		return &SignInOutput{
			Status: fhttp.StatusOK, SetCookie: set, Body: fhttp.NewResource(resources.NewTwoFactorChallenge(*c)),
		}
	}

	return &SignInOutput{
		Status:    fhttp.StatusOK,
		SetCookie: append(set, cookies.TwoFactorCookie("", 0)),
		Body:      fhttp.NewResource(resources.NewMe(ctx, signedIn.Me, server.instance())),
	}
}
