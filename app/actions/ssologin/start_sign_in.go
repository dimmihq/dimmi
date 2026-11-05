/*
|--------------------------------------------------------------------------
| Start Sign-In
|--------------------------------------------------------------------------
|
| Sends a person to their company's identity provider. Only an active
| connection signs people in; an owner may try out any connection that
| is not disabled. The attempt is remembered with the request sent, so
| the answer can be matched to it and nothing else.
|
*/

package ssologin

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
StartSignInInput is the connection, why the sign-in starts, who is
signed in already, nil for nobody, and where to land after.
*/
type StartSignInInput struct {
	ConnectionID ids.UUID
	Intent       string
	UserID       ids.UUID
	ReturnTo     string
}

/*
Started is where to send the browser, and the value its attempt cookie
holds.
*/
type Started struct {
	URL     string
	Browser string
}

/*
StartSignIn starts a sign-in through a connection.

Errors: [exceptions.ErrNotFound] for an unknown connection or one that
may not sign in now, [exceptions.ErrAuthBusy] when attempts cannot be
stored, or a wrapped error.
*/
type StartSignIn struct {
	DB       *database.DB
	Attempts appauth.SSOAttempts
	SAML     sso.SAMLProviders
	OIDC     OIDCProviders
}

/*
Handle remembers the attempt and returns the identity provider's
address.
*/
func (a *StartSignIn) Handle(ctx context.Context, in StartSignInInput) (Started, error) {
	conn, err := findConnection(ctx, a.DB, in.ConnectionID)
	if err != nil {
		return Started{}, err
	}

	if !startable(conn.Connection.Status, in.Intent) {
		return Started{}, exceptions.ErrNotFound
	}

	relay, browser := a.Attempts.Relay()
	attempt := appauth.SSOAttempt{
		ConnectionID: conn.Connection.ID, Intent: in.Intent, UserID: in.UserID, ReturnTo: in.ReturnTo,
		Browser: browser,
	}

	url, err := a.address(ctx, conn, relay, &attempt)
	if err != nil {
		return Started{}, err
	}

	if err := a.Attempts.Remember(ctx, relay, attempt); err != nil {
		return Started{}, busy(err)
	}

	return Started{URL: url, Browser: browser}, nil
}

/*
address returns the identity provider's sign-in address for the
attempt, recording the request id it must answer.
*/
func (a *StartSignIn) address(
	ctx context.Context, conn sso.Connection, relay string, attempt *appauth.SSOAttempt,
) (string, error) {
	if conn.Connection.Protocol == sso.ProtocolOIDC {
		return a.OIDC.AuthURL(ctx, conn, relay, attempt)
	}

	provider, err := a.SAML.For(ctx, conn)
	if err != nil {
		return "", err
	}

	url, requestID, err := provider.AuthURL(relay)
	if err != nil {
		return "", fmt.Errorf("build saml request: %w", err)
	}

	attempt.RequestID = requestID

	return url, nil
}

/*
startable reports whether a connection in the status may start a
sign-in for the intent.
*/
func startable(status, intent string) bool {
	if intent == appauth.SSOIntentTest {
		return status != sso.StatusDisabled
	}

	return intent == appauth.SSOIntentLogin && status == sso.StatusActive
}

/*
findConnection returns the connection with the id, whichever company it
belongs to.
*/
func findConnection(ctx context.Context, db *database.DB, id ids.UUID) (sso.Connection, error) {
	row, err := models.New(db).FindSSOConnectionByID(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return sso.Connection{}, exceptions.ErrNotFound
	}

	if err != nil {
		return sso.Connection{}, fmt.Errorf("find connection: %w", err)
	}

	return sso.Decode(row)
}

/*
busy turns an unreachable attempt store into the answer that says to try
again, keeping any other error.
*/
func busy(err error) error {
	if errors.Is(err, appauth.ErrSSOStoreDown) {
		return exceptions.ErrAuthBusy
	}

	return err
}
