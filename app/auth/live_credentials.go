/*
|--------------------------------------------------------------------------
| Live Credentials
|--------------------------------------------------------------------------
|
| A request proves its credential afresh every time, but a realtime
| connection proved it once, when it opened, and may stay open for days.
| This answers the question it has to keep asking: does what opened this
| connection still work? A session is gone once it is signed out, expired
| or replaced. A token is gone once revoked or expired, or once its member
| is no longer active. Either is gone with a deleted account.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
LiveCredentials tells whether a caller's credential still works.
*/
type LiveCredentials struct {
	DB       *database.DB
	Clock    clock.Clock
	Sessions *session.Manager
}

/*
Alive reports whether the principal's account and credential still
work. A first-party app holds a grant the OAuth server keeps, which is
checked there, so it counts as alive here. A connected app's grant is
checked there too; here only its person must still be an active member.
*/
func (c *LiveCredentials) Alive(ctx context.Context, p *Principal) (bool, error) {
	_, err := liveUser(ctx, c.DB, p.User.ID)
	if errors.Is(err, fauth.ErrNoCredentials) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	switch p.Kind {
	case KindSession:
		return c.sessionAlive(ctx, p)
	case KindToken:
		if p.Connected {
			return c.connectedAlive(ctx, p)
		}

		return c.tokenAlive(ctx, p)
	default:
		return true, nil
	}
}

/*
sessionAlive reports whether the session is among the user's live ones.
*/
func (c *LiveCredentials) sessionAlive(ctx context.Context, p *Principal) (bool, error) {
	live, err := c.Sessions.Sessions(ctx, p.User.ID)
	if err != nil {
		return false, fmt.Errorf("list sessions to check one: %w", err)
	}

	return slices.ContainsFunc(live, func(r session.Record) bool { return r.ID == p.CredentialID }), nil
}

/*
connectedAlive reports whether a connected app's person is still an
active member of the workspace it was connected to.
*/
func (c *LiveCredentials) connectedAlive(ctx context.Context, p *Principal) (bool, error) {
	_, err := grantMember(ctx, c.DB, p.WorkspaceID, p.User.ID)
	if errors.Is(err, fauth.ErrInvalidToken) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

/*
tokenAlive reports whether the token still works and its member is
still active, reading both inside the token's workspace.
*/
func (c *LiveCredentials) tokenAlive(ctx context.Context, p *Principal) (bool, error) {
	alive := false
	scoped := tenancy.With(ctx, p.WorkspaceID.String())

	err := c.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		tok, err := q.FindWorkspaceToken(ctx, models.FindWorkspaceTokenParams{
			WorkspaceID: p.WorkspaceID, ID: p.CredentialID,
		})
		if err != nil {
			return err
		}

		member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: p.WorkspaceID, ID: tok.OwnerID,
		})
		if err != nil {
			return err
		}

		unexpired := tok.ExpiresAt == nil || tok.ExpiresAt.After(c.Clock.Now())
		alive = tok.RevokedAt == nil && unexpired && member.Status == string(enums.MemberStatusActive)

		return nil
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("check token: %w", err)
	}

	return alive, nil
}

/*
Refreshed returns a copy of the caller as their session now stands, for
a credential that stays open while the person keeps working: someone
active over the API since a connection opened is not idle, and someone
who signed in again is not expired. A session that ended leaves the
caller as they were, for Alive to refuse.
*/
func (c *LiveCredentials) Refreshed(ctx context.Context, p *Principal) (*Principal, error) {
	if p.Kind != KindSession {
		return p, nil
	}

	records, err := c.Sessions.Sessions(ctx, p.User.ID)
	if err != nil {
		return nil, fmt.Errorf("list sessions to refresh one: %w", err)
	}

	fresh := *p

	for _, record := range records {
		if record.ID == p.CredentialID {
			fresh.AuthenticatedAt, fresh.LastActivityAt = record.AuthenticatedAt, WorkedAt(record)
			fresh.SignIn = SignInFrom(record)
			fresh.SSOConnectionID, fresh.AuthDomain = record.ConnectionID, record.AuthDomain
		}
	}

	return &fresh, nil
}
