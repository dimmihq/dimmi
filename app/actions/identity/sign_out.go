/*
|--------------------------------------------------------------------------
| Sign A Customer Out Everywhere
|--------------------------------------------------------------------------
|
| When a customer's account is compromised, or they ask, a teammate can
| end every messenger session the customer has open, on every device, at
| once. Each open messenger is told and disconnected; the customer signs
| in again with a fresh token from the host app.
|
*/

package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SignOutContact ends every live messenger session of the contact and
returns how many ended.
*/
func (s *Secrets) SignOutContact(ctx context.Context, m authz.Membership, contactID ids.UUID) (int, error) {
	if err := RequireManage(m); err != nil {
		return 0, err
	}
	var ended []Ended
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		_, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: m.Workspace.ID, ID: contactID})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find contact: %w", err)
		}
		rows, err := q.RevokeContactWidgetSessions(ctx, models.RevokeContactWidgetSessionsParams{
			Now: s.Clock.Now(), WorkspaceID: m.Workspace.ID, ContactID: contactID,
		})
		if err != nil {
			return fmt.Errorf("end contact sessions: %w", err)
		}
		for _, r := range rows {
			ended = append(ended, Ended{SessionID: r.ID, ContactID: r.ContactID})
		}
		return nil
	})
	if err != nil || len(ended) == 0 || s.Streams == nil {
		return len(ended), err
	}
	return len(ended), s.Streams.CloseSessions(ctx, ended)
}
