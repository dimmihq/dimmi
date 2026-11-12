/*
|--------------------------------------------------------------------------
| Widget Conversation Tickets
|--------------------------------------------------------------------------
|
| A conversation that is a request shows its reference, title and the
| customer label of its state in the visitor's list and above its
| thread. Plain conversations carry nothing.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
withTickets adds the request each conversation carries to its shape;
out and found hold the same conversations in the same order.
*/
func (c *ConversationController) withTickets(
	ctx context.Context, out []resources.WidgetConversation, found []actions.VisibleConversation,
) error {
	if c.Tickets == nil || len(found) == 0 {
		return nil
	}

	convs := make([]ids.UUID, len(found))
	for i, f := range found {
		convs[i] = f.Conversation.ID
	}

	badges, err := c.Tickets.CustomerBadges(ctx, found[0].Conversation.WorkspaceID, convs)
	if err != nil {
		return err
	}

	for i, f := range found {
		if b, ok := badges[f.Conversation.ID]; ok {
			out[i].Ticket = resources.NewWidgetTicketBlock(b)
		}
	}

	return nil
}
