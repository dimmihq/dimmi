/*
|--------------------------------------------------------------------------
| End Session
|--------------------------------------------------------------------------
|
| A user ends one of their own browser sessions: the one they are using,
| or one they see in their list, such as a lost laptop's. A session id
| that is not the user's own is not found, so nobody learns which
| sessions exist by guessing. Ending a session is final for that
| browser: its next request is answered as signed out.
|
*/

package auth

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EndSessionInput names the caller and the session to end, which is the
caller's own when SessionID is the caller's credential.
*/
type EndSessionInput struct {
	Principal *appauth.Principal
	SessionID ids.UUID
	Client    appauth.Client
}

/*
EndSession ends one of the caller's own browser sessions.

Rules:
  - Ending the current session signs the browser out and clears its
    cookie.
  - Another session must be the caller's own.

Events: [appevents.SessionRevoked].

Errors: [exceptions.ErrNotFound] for a session that is not the
caller's, or a wrapped error.
*/
type EndSession struct {
	Sessions    *session.Manager
	Events      *events.Dispatcher
	SecurityLog *appauth.SecurityLog
}

/*
Handle ends the session.
*/
func (a *EndSession) Handle(ctx context.Context, in EndSessionInput) error {
	current := in.SessionID == in.Principal.CredentialID

	end := a.endOther
	if current {
		end = endCurrent
	}

	if err := end(ctx, in); err != nil {
		return err
	}

	event, metadata := appauth.EventSessionRevoked, map[string]any{
		"session_id": models.PublicID(models.PrefixSession, in.SessionID),
	}
	if current {
		event = appauth.EventLogout
	}

	err := a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: in.Principal.User.ID, Type: event, Client: in.Client, Metadata: metadata,
	})
	if err != nil {
		return err
	}

	return a.Events.Dispatch(ctx, nil, appevents.SessionRevoked{UserID: in.Principal.User.ID, SessionID: in.SessionID})
}

/*
endCurrent ends the request's own session, which also clears the
browser's cookie.
*/
func endCurrent(ctx context.Context, _ EndSessionInput) error {
	s, ok := session.From(ctx)
	if !ok {
		return errNoSession
	}

	s.Destroy()

	return nil
}

/*
endOther ends another of the caller's sessions.
*/
func (a *EndSession) endOther(ctx context.Context, in EndSessionInput) error {
	err := a.Sessions.Revoke(ctx, in.Principal.User.ID, in.SessionID)
	if errors.Is(err, session.ErrNotFound) {
		return exceptions.ErrNotFound
	}

	return err
}
