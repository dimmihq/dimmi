/*
|--------------------------------------------------------------------------
| Erasing What Readers And Recipients Left
|--------------------------------------------------------------------------
|
| A customer leaves traces outside their conversations: notes on whether
| an article helped, changelog subscriptions under their address, and
| the addresses broadcasts were delivered to. Their rows point at the
| contact only loosely, so deleting the contact would leave the words and
| addresses behind. This eraser forgets them; counts built on the rows
| stay as they were. Running it again finds nothing left.
|
*/

package erasures

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
TraceEraser forgets an erased customer's article notes, changelog
subscriptions and broadcast addresses.
*/
type TraceEraser struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Name names what the eraser removes.
*/
func (e *TraceEraser) Name() string {
	return "reader_and_recipient_traces"
}

/*
Erase scrubs the contact's rating notes and broadcast addresses and
deletes their changelog subscriptions, in one transaction.
*/
func (e *TraceEraser) Erase(ctx context.Context, workspaceID, contactID ids.UUID) error {
	scoped := tenancy.With(ctx, workspaceID.String())

	return e.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q, now := models.New(tx), e.Clock.Now()

		if _, err := q.ScrubErasedRatingComments(ctx, models.ScrubErasedRatingCommentsParams{
			Now: now, WorkspaceID: workspaceID, ContactID: &contactID,
		}); err != nil {
			return fmt.Errorf("scrub rating notes: %w", err)
		}

		if _, err := q.DeleteErasedChangelogSubscriptions(ctx, models.DeleteErasedChangelogSubscriptionsParams{
			WorkspaceID: workspaceID, ContactID: contactID,
		}); err != nil {
			return fmt.Errorf("delete changelog subscriptions: %w", err)
		}

		if _, err := q.ScrubErasedBroadcastRecipients(ctx, models.ScrubErasedBroadcastRecipientsParams{
			Now: now, WorkspaceID: workspaceID, ContactID: &contactID,
		}); err != nil {
			return fmt.Errorf("scrub broadcast recipients: %w", err)
		}

		return nil
	})
}
