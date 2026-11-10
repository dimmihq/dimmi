/*
|--------------------------------------------------------------------------
| Broadcast Feedback
|--------------------------------------------------------------------------
|
| A broadcast email that bounces or draws a complaint is recorded on the
| sent email like any other, and also on the broadcast's recipient, so
| the broadcast's report counts it and a send that bounces too much
| pauses itself. Only the address the email went to is ever affected.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
broadcastFailed marks the broadcast recipient of a sent broadcast email
bounced or complained, and tells the broadcast to count again.
*/
func (run reportRun) broadcastFailed(ctx context.Context, o outcome) error {
	failed := o.overall == string(enums.OutboundEmailStatusBounced) ||
		o.overall == string(enums.OutboundEmailStatusComplained)
	if run.sent.Kind != string(enums.OutboundEmailKindBroadcast) || !failed {
		return nil
	}

	r, err := run.q.FindBroadcastRecipientByOutboundEmail(ctx, models.FindBroadcastRecipientByOutboundEmailParams{
		WorkspaceID: run.sent.WorkspaceID, OutboundEmailID: &run.sent.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find broadcast recipient: %w", err)
	}

	reason := bounceReason(o)

	if _, err := run.q.MarkBroadcastRecipientFailed(ctx, models.MarkBroadcastRecipientFailedParams{
		WorkspaceID: r.WorkspaceID, ID: r.ID, Status: o.overall, Reason: &reason, Now: run.a.Clock.Now(),
	}); err != nil && !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("fail broadcast recipient: %w", err)
	}

	return run.a.Events.Dispatch(ctx, run.tx, appevents.BroadcastRecipientChanged{
		Base:        appevents.Base{ID: ids.New(), WorkspaceID: r.WorkspaceID, OccurredAt: run.a.Clock.Now()},
		BroadcastID: r.BroadcastID, RecipientID: r.ID,
	})
}

/*
bounceReason names why a broadcast email did not land, in the words its
report shows: the address does not exist, the mailbox refused it, or
the person reported it as spam.
*/
func bounceReason(o outcome) string {
	switch {
	case o.overall == string(enums.OutboundEmailStatusComplained):
		return "spam_complaint"
	case addressGone(o.status.Code):
		return "address_not_found"
	}

	return "mailbox_refused"
}
