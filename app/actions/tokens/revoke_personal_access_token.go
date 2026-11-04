/*
|--------------------------------------------------------------------------
| Revoke Personal Access Token
|--------------------------------------------------------------------------
|
| Revoking a token ends it at once: the guard reads the token on every
| request, so the very next call with it is refused, and connections it
| opened close. The row stays, with
| who ended it, so its owner still sees when it was last used. Revoking
| a token that already ended changes nothing.
|
*/

package tokens

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RevokePersonalAccessTokenInput is who asks, through their membership,
the token, and the client asking.
*/
type RevokePersonalAccessTokenInput struct {
	Membership authz.Membership
	TokenID    ids.UUID
	Client     appauth.Client
}

/*
RevokePersonalAccessToken ends a token.

Rules:
  - Only a token the caller may manage is found: their own, or any with
    tokens:manage_any.
  - The token's owner finds the revocation in their security log.

Errors: [exceptions.ErrNotFound], or a wrapped error.
*/
type RevokePersonalAccessToken struct {
	DB          *database.DB
	Clock       clock.Clock
	Events      *events.Dispatcher
	SecurityLog *appauth.SecurityLog
}

/*
Handle revokes the token.
*/
func (a *RevokePersonalAccessToken) Handle(ctx context.Context, in RevokePersonalAccessTokenInput) error {
	m := in.Membership

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		tok, err := findManaged(ctx, q, m, in.TokenID)
		if err != nil {
			return err
		}

		revoked, err := q.RevokeWorkspaceToken(ctx, models.RevokeWorkspaceTokenParams{
			WorkspaceID: m.Workspace.ID, ID: tok.ID, RevokedBy: m.Member.ID, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("revoke token: %w", err)
		}

		if revoked == 0 {
			return nil
		}

		err = a.SecurityLog.Record(ctx, appauth.Entry{
			UserID: tok.MemberUserID, Type: appauth.EventTokenRevoked, Client: in.Client,
			Metadata: map[string]any{
				"token_id": models.PublicID(models.PrefixPersonalToken, tok.ID), "name": tok.Name,
				"revoked_by": models.PublicID(models.PrefixMember, m.Member.ID),
			},
		})
		if err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.TokenRevoked{
			WorkspaceID: m.Workspace.ID, TokenID: tok.ID, MemberID: tok.OwnerID, UserID: tok.MemberUserID,
			RevokedBy: m.Member.ID,
		})
	})
}
