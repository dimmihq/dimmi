/*
|--------------------------------------------------------------------------
| Consume SAML Answer
|--------------------------------------------------------------------------
|
| Reads the answer a company's identity provider posts back. It must
| answer the attempt this browser started, carry a valid signature, and
| be used once. An answer nobody asked for is never trusted to sign in:
| a valid one restarts the sign-in from here. Every failure looks the
| same to the person; the reason is kept on the connection for its
| owners, never the answer itself.
|
*/

package ssologin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/saml"

	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
assertionSkew is how long past its expiry an assertion stays remembered,
covering the clock skew the check allows.
*/
const assertionSkew = 3 * time.Minute

/*
ConsumeSAMLInput is the connection the answer names, the answer, the
relay state, the browser's attempt cookie, and the client.
*/
type ConsumeSAMLInput struct {
	ConnectionID ids.UUID
	Response     string
	RelayState   string
	Browser      string
	Client       appauth.Client
}

/*
Result is what an answer led to: a sign-in, a test recorded, or a
sign-in to restart from here; ReturnTo is where to land.
*/
type Result struct {
	SignedIn auth.SignedIn
	Tested   bool
	Restart  bool
	ReturnTo string
}

/*
ConsumeSAML reads a SAML answer.

Errors: [exceptions.ErrSSOFailed] for any answer that cannot sign in,
[exceptions.ErrAuthBusy] when attempts cannot be checked, or what
[CompleteSSOLogin] refuses.
*/
type ConsumeSAML struct {
	DB       *database.DB
	Clock    clock.Clock
	Attempts appauth.SSOAttempts
	SAML     sso.SAMLProviders
	Complete *CompleteSSOLogin
}

/*
Handle checks the answer and acts on it.
*/
func (a *ConsumeSAML) Handle(ctx context.Context, in ConsumeSAMLInput) (Result, error) {
	conn, err := findConnection(ctx, a.DB, in.ConnectionID)
	if err != nil || conn.Connection.Protocol != sso.ProtocolSAML {
		return Result{}, exceptions.ErrSSOFailed
	}

	provider, err := a.SAML.For(ctx, conn)
	if err != nil {
		return Result{}, a.fail(ctx, conn, "connection_unreadable")
	}

	attempt, found, err := a.Attempts.Claim(ctx, in.RelayState, in.Browser)
	if err != nil {
		return Result{}, busy(err)
	}

	if !found {
		return a.unsolicited(ctx, conn, provider, in.Response)
	}

	if attempt.ConnectionID != conn.Connection.ID {
		return Result{}, a.fail(ctx, conn, "wrong_connection")
	}

	assertion, err := provider.Consume(in.Response, attempt.RequestID)
	if err != nil {
		return Result{}, a.fail(ctx, conn, reason(err))
	}

	first, err := a.Attempts.FirstUse(ctx, conn.Connection.ID, assertion.AssertionID,
		assertion.NotOnOrAfter.Sub(a.Clock.Now())+assertionSkew)
	if err != nil {
		return Result{}, busy(err)
	}

	if !first {
		return Result{}, a.fail(ctx, conn, "replayed")
	}

	identity := identityOf(conn.Config, assertion)
	if identity.Email == "" {
		return Result{}, a.fail(ctx, conn, "missing_email")
	}

	return a.finish(ctx, answered{conn: conn, attempt: attempt, identity: identity, client: in.Client})
}

/*
answered is an answer that passed every check: the connection, the
attempt it answered, the person it vouches for, and their client.
*/
type answered struct {
	conn     sso.Connection
	attempt  appauth.SSOAttempt
	identity Identity
	client   appauth.Client
}

/*
finish records a test, or signs the person in.
*/
func (a *ConsumeSAML) finish(ctx context.Context, ans answered) (Result, error) {
	conn := ans.conn
	if ans.attempt.Intent == appauth.SSOIntentTest {
		now := a.Clock.Now()
		if _, err := models.New(a.DB).MarkSSOConnectionTested(ctx, models.MarkSSOConnectionTestedParams{
			Now: &now, ID: conn.Connection.ID,
		}); err != nil {
			return Result{}, fmt.Errorf("record connection test: %w", err)
		}

		return Result{Tested: true}, nil
	}

	signedIn, err := a.Complete.Handle(ctx, CompleteSSOLoginInput{
		Connection: conn, Identity: ans.identity, Client: ans.client,
	})
	if err != nil {
		return Result{}, err
	}

	return Result{SignedIn: signedIn, ReturnTo: ans.attempt.ReturnTo}, nil
}

/*
unsolicited checks an answer no attempt of this browser asked for. A
valid one restarts the sign-in from here, never signs in by itself.
*/
func (a *ConsumeSAML) unsolicited(
	ctx context.Context, conn sso.Connection, provider *saml.Provider, response string,
) (Result, error) {
	_, err := provider.Consume(response, "")
	if errors.Is(err, saml.ErrUnsolicited) && conn.Connection.Status == sso.StatusActive {
		return Result{Restart: true}, nil
	}

	if err == nil {
		err = saml.ErrUnsolicited
	}

	return Result{}, a.fail(ctx, conn, reason(err))
}

/*
fail keeps the reason on the connection for its owners and returns the
answer every failure gets.
*/
func (a *ConsumeSAML) fail(ctx context.Context, conn sso.Connection, code string) error {
	raw, err := json.Marshal(map[string]any{"code": code, "at": a.Clock.Now()})
	if err != nil {
		return fmt.Errorf("encode sso failure: %w", err)
	}

	if err := models.New(a.DB).RecordSSOConnectionError(ctx, models.RecordSSOConnectionErrorParams{
		LastError: raw, Now: a.Clock.Now(), ID: conn.Connection.ID,
	}); err != nil {
		return fmt.Errorf("record sso failure: %w", err)
	}

	return exceptions.ErrSSOFailed
}

/*
identityOf reads the person from the assertion through the connection's
attribute names, falling back to an email name id for the address.
*/
func identityOf(cfg sso.Config, assertion saml.Assertion) Identity {
	first := func(name string) string {
		if values := assertion.Attributes[name]; name != "" && len(values) > 0 {
			return strings.TrimSpace(values[0])
		}

		return ""
	}

	email := first(cfg.AttributeMap.Email)
	if email == "" && assertion.NameIDFormat == sso.NameIDEmail {
		email = assertion.NameID
	}

	name := first(cfg.AttributeMap.DisplayName)
	if name == "" {
		name = strings.TrimSpace(first(cfg.AttributeMap.FirstName) + " " + first(cfg.AttributeMap.LastName))
	}

	var groups []string
	if cfg.AttributeMap.Groups != "" {
		groups = assertion.Attributes[cfg.AttributeMap.Groups]
	}

	return Identity{Subject: assertion.NameID, Email: email, Name: name, Groups: groups}
}

/*
reason names why the identity provider's answer was refused, for the
connection's owners.
*/
func reason(err error) string {
	for _, known := range []struct {
		err  error
		code string
	}{
		{saml.ErrInvalidSignature, "invalid_signature"},
		{saml.ErrExpired, "expired"},
		{saml.ErrWrongAudience, "wrong_audience"},
		{saml.ErrWrongDestination, "wrong_destination"},
		{saml.ErrWrongIssuer, "wrong_issuer"},
		{saml.ErrInResponseTo, "wrong_request"},
		{saml.ErrUnsolicited, "unsolicited"},
		{saml.ErrStatusNotSuccess, "identity_provider_refused"},
	} {
		if errors.Is(err, known.err) {
			return known.code
		}
	}

	return "malformed"
}
