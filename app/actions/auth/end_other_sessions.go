/*
|--------------------------------------------------------------------------
| End Other Sessions
|--------------------------------------------------------------------------
|
| "Sign out everywhere else": every browser session of the user ends
| except the one asking, which is why this needs a recent confirmation
| of who is asking.
|
*/

package auth

import (
	"context"

	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
)

/*
EndOtherSessions signs the caller out of every browser session but the
one they are using.

Errors: a wrapped error.
*/
type EndOtherSessions struct {
	Sessions    *session.Manager
	SecurityLog *appauth.SecurityLog
}

/*
Handle ends the other sessions and logs how many ended.
*/
func (a *EndOtherSessions) Handle(ctx context.Context, p *appauth.Principal, client appauth.Client) error {
	ended, err := a.Sessions.RevokeOthers(ctx, p.ID(), p.CredentialID)
	if err != nil {
		return err
	}

	return a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: p.ID(), Type: appauth.EventSessionsRevokedOthers, Client: client,
		Metadata: map[string]any{"count": ended},
	})
}
