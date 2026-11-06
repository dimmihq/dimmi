/*
|--------------------------------------------------------------------------
| Settling An Answer
|--------------------------------------------------------------------------
|
| Whatever the protocol, an identity provider's answer that passed its
| checks ends the same way: a test is recorded on the connection, or the
| person signs in. A failure ends the same way too: the reason is kept on
| the connection for its owners, and the person sees one answer.
|
*/

package ssologin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
settling finishes answers: the database, the clock, and the sign-in.
*/
type settling struct {
	DB       *database.DB
	Clock    clock.Clock
	Complete *CompleteSSOLogin
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
func (a settling) finish(ctx context.Context, ans answered) (Result, error) {
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
fail keeps the reason on the connection for its owners and returns the
answer every failure gets.
*/
func (a settling) fail(ctx context.Context, conn sso.Connection, code string) error {
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
