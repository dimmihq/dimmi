/*
|--------------------------------------------------------------------------
| A Person's Grants
|--------------------------------------------------------------------------
|
| Every app a person connected is a grant they can see and take back. A
| grant lists the app, the tenant it acts in, the access it holds and
| when it was made and last renewed. Taking one back ends it and every
| token issued under it at once, and only the person it acts for may.
|
*/

package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Grant is an app a person connected: the grant's id, the app's id and
name, the tenant it acts in, empty for a first-party app, the scopes it
holds, when it was made and when a token was last issued under it.
*/
type Grant struct {
	ID         ids.UUID
	ClientID   string
	ClientName string
	Tenant     string
	Scopes     []string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

/*
Grants returns the person's live grants, newest first.
*/
func (s *Server) Grants(ctx context.Context, user ids.UUID) ([]Grant, error) {
	sql := "SELECT g.id, g.client_id, c.name, COALESCE(g.tenant_id, ''), g.scopes, g.created_at, " +
		"COALESCE((SELECT max(t.created_at) FROM oauth_tokens t WHERE t.grant_id = g.id), g.created_at) " +
		"FROM oauth_grants g JOIN oauth_clients c ON c.id = g.client_id " +
		"WHERE g.user_id = $1 AND g.revoked_at IS NULL ORDER BY g.created_at DESC, g.id DESC"

	rows, err := s.db.Query(ctx, sql, user)
	if err != nil {
		return nil, fmt.Errorf("list grants: %w", err)
	}

	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Grant, error) {
		var g Grant
		err := row.Scan(&g.ID, &g.ClientID, &g.ClientName, &g.Tenant, &g.Scopes, &g.CreatedAt, &g.LastUsedAt)

		return g, err
	})
	if err != nil {
		return nil, fmt.Errorf("list grants: %w", err)
	}

	return out, nil
}

/*
RevokeUserGrant ends one of the person's grants and every token issued
under it. A grant that is not the person's, or is already ended, is
ErrGrantNotFound.
*/
func (s *Server) RevokeUserGrant(ctx context.Context, user, id ids.UUID) error {
	return s.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var owner ids.UUID

		err := tx.QueryRow(ctx, "SELECT user_id FROM oauth_grants WHERE id = $1 AND revoked_at IS NULL FOR UPDATE",
			id).Scan(&owner)
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, database.ErrNotFound) || (err == nil && owner != user) {
			return ErrGrantNotFound
		}

		if err != nil {
			return fmt.Errorf("find grant: %w", err)
		}

		return revokeGrant(ctx, tx, id, s.clock.Now())
	})
}
