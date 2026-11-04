/*
|--------------------------------------------------------------------------
| Authenticate Widget Session
|--------------------------------------------------------------------------
|
| Every visitor request after the first names its session by a bearer token
| alone. The token's hash is looked up before any workspace is known, through
| a resolver that returns only ids; the session is then read inside its
| workspace like any other row. A revoked session never works again, an
| expired one only where a fresh identity token can revive it, and an
| anonymous visitor's session slides on as long as they keep coming back,
| written down at most once a minute.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
activityThrottle is how often a session's last visit is written down.
*/
const activityThrottle = time.Minute

/*
AuthenticateWidgetSessionInput is the bearer token a request sent, and
whether an expired identified session is admitted, which only the route
that revives it with a fresh identity token asks for.
*/
type AuthenticateWidgetSessionInput struct {
	Token        string
	AdmitExpired bool
}

/*
Authenticated is a visitor's session with its widget.
*/
type Authenticated struct {
	Session  models.WidgetSession
	Resolved Resolved
}

/*
AuthenticateWidgetSession finds the session a bearer token names.
AnonymousTTL is how long an anonymous session lives after the last visit.
*/
type AuthenticateWidgetSession struct {
	DB           *database.DB
	Clock        clock.Clock
	Directory    *Directory
	AnonymousTTL time.Duration
}

/*
Handle returns the live session and its widget, or why there is none:
[exceptions.ErrSessionInvalid], [exceptions.ErrSessionRevoked] or
[exceptions.ErrSessionExpired].
*/
func (a *AuthenticateWidgetSession) Handle(
	ctx context.Context, in AuthenticateWidgetSessionInput,
) (Authenticated, error) {
	if !WellFormedSessionToken(in.Token) {
		return Authenticated{}, exceptions.ErrSessionInvalid
	}

	row, err := models.New(a.DB).ResolveWidgetSession(ctx, HashSessionToken(in.Token))
	if errors.Is(err, database.ErrNotFound) {
		return Authenticated{}, exceptions.ErrSessionInvalid
	}

	if err != nil {
		return Authenticated{}, fmt.Errorf("resolve widget session: %w", err)
	}

	session, err := a.load(ctx, row.WorkspaceID, row.ID)
	if err != nil {
		return Authenticated{}, err
	}

	if err := a.usable(session, in); err != nil {
		return Authenticated{}, err
	}

	resolved, err := a.Directory.Find(ctx, session.WidgetSettingsID)
	if errors.Is(err, exceptions.ErrAppNotFound) {
		return Authenticated{}, exceptions.ErrSessionRevoked
	}

	if err != nil {
		return Authenticated{}, err
	}

	session, err = a.slide(ctx, session)

	return Authenticated{Session: session, Resolved: resolved}, err
}

/*
load reads the session inside its workspace.
*/
func (a *AuthenticateWidgetSession) load(ctx context.Context, workspaceID, id ids.UUID) (models.WidgetSession, error) {
	var session models.WidgetSession

	err := a.DB.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error
		session, err = models.New(tx).FindWidgetSession(ctx, models.FindWidgetSessionParams{
			WorkspaceID: workspaceID, ID: id,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return session, exceptions.ErrSessionInvalid
	}

	if err != nil {
		return session, fmt.Errorf("find widget session: %w", err)
	}

	return session, nil
}

/*
usable returns why the session cannot be used, nil when it can.
*/
func (a *AuthenticateWidgetSession) usable(s models.WidgetSession, in AuthenticateWidgetSessionInput) error {
	if s.RevokedAt != nil {
		return exceptions.ErrSessionRevoked
	}

	expired := !a.Clock.Now().Before(s.ExpiresAt)
	if expired && (!in.AdmitExpired || s.Kind != string(enums.WidgetSessionIdentified)) {
		return exceptions.ErrSessionExpired
	}

	return nil
}

/*
slide moves an anonymous session's expiry on from now, writing it down at
most once a minute. An identified session lives only as long as its last
identity token allows, so it never slides.
*/
func (a *AuthenticateWidgetSession) slide(
	ctx context.Context, s models.WidgetSession,
) (models.WidgetSession, error) {
	now := a.Clock.Now()
	if s.Kind != string(enums.WidgetSessionAnonymous) || now.Sub(s.LastSeenAt) < activityThrottle {
		return s, nil
	}

	s.ExpiresAt, s.LastSeenAt = now.Add(a.AnonymousTTL), now

	scoped := tenancy.With(ctx, s.WorkspaceID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).UpdateWidgetSessionActivity(ctx, models.UpdateWidgetSessionActivityParams{
			ExpiresAt: s.ExpiresAt, Now: now, WorkspaceID: s.WorkspaceID, ID: s.ID,
		})
	})
	if err != nil {
		return s, fmt.Errorf("slide widget session: %w", err)
	}

	return s, nil
}
