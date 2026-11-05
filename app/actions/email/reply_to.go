/*
|--------------------------------------------------------------------------
| Reply-To
|--------------------------------------------------------------------------
|
| A customer's mail can ask for answers to go to another address than
| the one it came from, as every mail client honors. The person stays
| the contact their From address names; only where replies go changes,
| following their latest ordinary email in the conversation. Mail from
| lists and senders nobody answers is left out, and so is an address
| of the workspace's own.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
askedRecipients returns who a reply that names nobody goes to: the
address the customer's latest email asked answers to go to, else the
customer's own.
*/
func askedRecipients(ctx context.Context, q *models.Queries, out *outbound, own []string) ([]string, error) {
	customer := out.msg.Recipient.IdentityValue
	if customer == "" {
		return nil, nil
	}

	asked, err := q.FindContactReplyTo(ctx, models.FindContactReplyToParams{
		WorkspaceID: out.msg.WorkspaceID, ConversationID: out.msg.ConversationID,
		ContactID: &out.msg.Recipient.ContactID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("find reply-to address: %w", err)
	}

	if asked = lower(asked); asked == "" || slices.Contains(own, asked) {
		return []string{customer}, nil
	}

	return []string{asked}, nil
}
