/*
|--------------------------------------------------------------------------
| OAuth Redirect Controller
|--------------------------------------------------------------------------
|
| GET /auth/{provider}/redirect sends the browser to the provider. What
| the attempt is for travels in a sealed cookie the callback reads back:
| signing in, signing in afresh on the browser's own session, or linking
| the provider to the signed-in user. Linking adds a way into the
| account, so it takes a browser session that confirmed who it is
| recently; a bearer token never gets that far. A provider the install
| does not offer answers as if it did not exist.
|
*/

package web

import (
	"cmp"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
OAuthRedirectController starts sign-ins at identity providers.
ConfirmationTTL is how recently a session must have confirmed who it is
to link a provider.
*/
type OAuthRedirectController struct {
	Providers       providers.ProviderResolver
	Attempts        appauth.OAuthAttempts
	Cookies         appauth.Cookies
	Clock           clock.Clock
	ConfirmationTTL time.Duration
	Errors          fhttp.Errors
}

/*
Invoke starts the attempt and sends the browser to the provider.
*/
func (c *OAuthRedirectController) Invoke(w fhttp.ResponseWriter, r *fhttp.Request) {
	ctx := r.Context()

	provider, err := c.Providers.Find(ctx, r.PathValue("provider"))
	if err != nil {
		c.Errors.Render(w, r, exceptions.ErrNotFound)

		return
	}

	attempt, err := c.attempt(r)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	authorization, err := provider.AuthorizeURL(ctx)
	if err != nil {
		redirect(w, withError(failurePage(attempt.Intent), "oauth_unavailable"))

		return
	}

	attempt.Provider, attempt.State = provider.Key(), authorization.State

	sealed, err := c.Attempts.Seal(attempt)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	setCookie(w, c.Cookies.OAuthCookie(sealed, appauth.OAuthAttemptLifetime))
	redirect(w, authorization.URL)
}

/*
attempt reads what the attempt is for from the query. A fresh sign-in
without a session is a plain one; a link needs a session that confirmed
who it is within the window.
*/
func (c *OAuthRedirectController) attempt(r *fhttp.Request) (appauth.OAuthAttempt, error) {
	query := r.URL.Query()
	attempt := appauth.OAuthAttempt{
		Intent:     cmp.Or(query.Get("intent"), appauth.IntentLogin),
		Invitation: query.Get("invitation"),
		Redirect:   appauth.SafeRedirect(query.Get("redirect")),
	}

	userID := sessionUser(r)
	signedIn := userID != ids.Nil()

	switch attempt.Intent {
	case appauth.IntentLogin:
		return attempt, nil
	case appauth.IntentReauth:
		if !signedIn {
			attempt.Intent = appauth.IntentLogin

			return attempt, nil
		}
	case appauth.IntentLink:
		if !signedIn {
			return appauth.OAuthAttempt{}, fauth.ErrUnauthenticated
		}

		if s, _ := session.From(r.Context()); c.Clock.Now().Sub(s.ConfirmedAt()) > c.ConfirmationTTL {
			return appauth.OAuthAttempt{}, fauth.ErrConfirmationRequired
		}
	default:
		return appauth.OAuthAttempt{}, exceptions.ErrNotFound
	}

	attempt.UserID = userID

	return attempt, nil
}

/*
failurePage is the page a failed attempt lands on: the security
settings for a link, the sign-in page otherwise.
*/
func failurePage(intent string) string {
	if intent == appauth.IntentLink {
		return "/settings/security"
	}

	return "/login"
}

/*
sessionUser returns the user the request's session is signed in as, or
the nil id. Browser routes always run the session middleware.
*/
func sessionUser(r *fhttp.Request) ids.UUID {
	s, _ := session.From(r.Context())
	userID, _ := s.UserID()

	return userID
}
