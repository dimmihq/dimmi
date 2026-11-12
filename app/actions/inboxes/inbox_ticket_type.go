/*
|--------------------------------------------------------------------------
| Inbox Ticket Type
|--------------------------------------------------------------------------
|
| An inbox can turn every new conversation into a ticket of one customer
| type, such as email support where each message is a request. The
| change rides on an inbox update, inside its transaction, so the inbox
| comes back as it now is.
|
*/

package inboxes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketTypeChoice is the ticket type new conversations become, ID nil
for none.
*/
type TicketTypeChoice struct {
	ID *ids.UUID
}

/*
typeChange is a chosen type and when it was chosen.
*/
type typeChange struct {
	TicketTypeChoice
	Now time.Time
}

/*
setTicketType points the inbox's new conversations at the chosen type,
which must be a live customer type of the workspace, and returns the
inbox as it now is.
*/
func setTicketType(
	ctx context.Context, q *models.Queries, inbox models.Inbox, choice typeChange,
) (models.Inbox, error) {
	if sameType(inbox.TicketTypeID, choice.ID) {
		return inbox, nil
	}

	if choice.ID != nil {
		t, err := q.FindTicketType(ctx, models.FindTicketTypeParams{WorkspaceID: inbox.WorkspaceID, ID: *choice.ID})
		if errors.Is(err, database.ErrNotFound) {
			return inbox, exceptions.ErrInboxTicketTypeInvalid
		}

		if err != nil {
			return inbox, fmt.Errorf("find ticket type: %w", err)
		}

		if t.Kind != "customer" || t.ArchivedAt != nil {
			return inbox, exceptions.ErrInboxTicketTypeInvalid
		}
	}

	err := q.SetInboxTicketType(ctx, models.SetInboxTicketTypeParams{
		TicketTypeID: choice.ID, Now: choice.Now, WorkspaceID: inbox.WorkspaceID, ID: inbox.ID,
	})
	if err != nil {
		return inbox, fmt.Errorf("set inbox ticket type: %w", err)
	}

	inbox.TicketTypeID, inbox.UpdatedAt = choice.ID, choice.Now

	return inbox, nil
}

/*
sameType reports whether two optional type ids name the same type.
*/
func sameType(a, b *ids.UUID) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}
