/*
|--------------------------------------------------------------------------
| OAuth Callback Controller
|--------------------------------------------------------------------------
|
| GET /auth/{provider}/callback is where a provider sends the browser
| back. It only counts with the sealed cookie the redirect set, for the
| same provider, in a browser still signed in as the user a link or
| fresh sign-in was started for. The cookie is spent whatever happens,
| so a callback works once.
|
| A sign-in lands on the page the attempt asked for, or, when the
| attempt carried an invitation, on the invitation page, which accepts
| it or says it was sent to another address. A failure the person can
| act on lands on the sign-in page, or the security settings for a
| link, with its error code.
|
*/

package web

import (
	"errors"
	"net/url"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
OAuthCallbackController finishes sign-ins at identity providers.
*/
type OAuthCallbackController struct {
	Providers providers.ProviderResolver
	Attempts  appauth.OAuthAttempts
	Cookies   appauth.Cookies
	Callback  *auth.HandleOAuthCallback
	Errors    fhttp.Errors
}

/*
Invoke finishes the attempt and sends the browser on.
*/
func (c *OAuthCallbackController) Invoke(w fhttp.ResponseWriter, r *fhttp.Request) {
	ctx := r.Context()
	setCookie(w, c.Cookies.OAuthCookie("", 0))

	provider, err := c.Providers.Find(ctx, r.PathValue("provider"))
	if err != nil {
		c.Errors.Render(w, r, exceptions.ErrNotFound)

		return
	}

	attempt, ok := c.opened(r, provider.Key())
	if !ok {
		redirect(w, withError("/login", "oauth_state"))

		return
	}

	identity, err := provider.Callback(ctx, attempt.State, r.URL.Query())
	if err != nil {
		redirect(w, withError(failurePage(attempt.Intent), providerFailure(err)))

		return
	}

	result, err := c.Callback.Handle(ctx, auth.OAuthCallbackInput{
		Identity: identity, Intent: attempt.Intent, UserID: attempt.UserID, Invitation: attempt.Invitation,
		Client: clientOf(r, c.Cookies),
	})
	if code := fault.CodeOf(err); code != "" {
		redirect(w, withError(failurePage(attempt.Intent), code))

		return
	}

	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	if result.SignedIn.DeviceToken != "" {
		setCookie(w, c.Cookies.DeviceCookie(result.SignedIn.DeviceToken))
	}

	redirect(w, landing(attempt, result))
}

/*
opened returns the attempt sealed in the request's cookie, when it was
started for this provider by the user the session is still signed in
as.
*/
func (c *OAuthCallbackController) opened(r *fhttp.Request, provider string) (appauth.OAuthAttempt, bool) {
	cookie, err := r.Cookie(c.Cookies.OAuthName())
	if err != nil {
		return appauth.OAuthAttempt{}, false
	}

	attempt, ok := c.Attempts.Open(cookie.Value)
	if !ok || attempt.Provider != provider {
		return appauth.OAuthAttempt{}, false
	}

	if attempt.Intent != appauth.IntentLogin && attempt.UserID != sessionUser(r) {
		return appauth.OAuthAttempt{}, false
	}

	return attempt, true
}

/*
providerFailure returns the error code of a provider failure.
*/
func providerFailure(err error) string {
	switch {
	case errors.Is(err, providers.ErrState):
		return "oauth_state"
	case errors.Is(err, providers.ErrDenied):
		return "oauth_denied"
	default:
		return "oauth_unavailable"
	}
}

/*
landing returns where the browser goes once the callback succeeded.
*/
func landing(attempt appauth.OAuthAttempt, result auth.OAuthCallbackResult) string {
	if result.Linked {
		if attempt.Redirect == "/" {
			return "/settings/security"
		}

		return attempt.Redirect
	}

	if attempt.Invitation == "" {
		return attempt.Redirect
	}

	fragment := "#" + url.Values{"token": {attempt.Invitation}}.Encode()
	if result.InvitationMatches {
		return "/invitation" + fragment
	}

	return withError("/invitation", "email_mismatch") + fragment
}
