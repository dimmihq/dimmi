/*
|--------------------------------------------------------------------------
| Preview Retention
|--------------------------------------------------------------------------
|
| Before a workspace first sets a period for closed conversations, it is
| told how many would go to the trash, so nobody empties years of history
| by accident.
|
*/

package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
PreviewRetentionInput is who asks, and the period they consider.
*/
type PreviewRetentionInput struct {
	Membership             authz.Membership
	ClosedConversationDays int
}

/*
PreviewRetention counts the closed conversations a period would move.

Rules:
  - The caller needs settings:read.
  - The period is within 30 to 3650 days.

Errors: [exceptions.ErrForbidden], [exceptions.ErrInvalidRetention], or
a wrapped database error.
*/
type PreviewRetention struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle returns how many conversations the period would move today.
*/
func (a *PreviewRetention) Handle(ctx context.Context, in PreviewRetentionInput) (int64, error) {
	if !in.Membership.Can("settings:read") {
		return 0, exceptions.ErrForbidden
	}

	if in.ClosedConversationDays < 30 || in.ClosedConversationDays > 3650 {
		return 0, exceptions.ErrInvalidRetention.With("field", "closed_conversation_days")
	}

	var count int64

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		count, err = models.New(tx).CountStaleClosedConversations(ctx, models.CountStaleClosedConversationsParams{
			WorkspaceID: in.Membership.Workspace.ID,
			Cutoff:      a.Clock.Now().Add(-time.Duration(in.ClosedConversationDays) * day),
		})
		if err != nil {
			return fmt.Errorf("count stale conversations: %w", err)
		}

		return nil
	})

	return count, err
}
