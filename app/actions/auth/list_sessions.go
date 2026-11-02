/*
|--------------------------------------------------------------------------
| List Sessions
|--------------------------------------------------------------------------
|
| A user sees every browser they are signed in on, with how each signed
| in and when it was last used, so they can spot one they do not
| recognize and end it.
|
*/

package auth

import (
	"context"

	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
)

/*
BrowserSession is one of the user's live browser sessions as listed to
them. Current marks the one the request came from.
*/
type BrowserSession struct {
	Record  session.Record
	SignIn  appauth.SignIn
	Current bool
}

/*
ListSessions lists the caller's live browser sessions, the most recently
active first. Ended and idle sessions are left out.

Errors: a wrapped error.
*/
type ListSessions struct {
	Sessions *session.Manager
}

/*
Handle lists the sessions.
*/
func (a *ListSessions) Handle(ctx context.Context, p *appauth.Principal) ([]BrowserSession, error) {
	records, err := a.Sessions.Sessions(ctx, p.User.ID)
	if err != nil {
		return nil, err
	}

	listed := make([]BrowserSession, 0, len(records))
	for _, r := range records {
		listed = append(listed, BrowserSession{
			Record: r, SignIn: appauth.SignInFrom(r), Current: r.ID == p.CredentialID,
		})
	}

	return listed, nil
}
