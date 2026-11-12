/*
|--------------------------------------------------------------------------
| Tickets For Automation
|--------------------------------------------------------------------------
|
| What service levels and workflows need from tickets: a writer acting
| as dimmi inside their own transaction, a ticket locked through it, and
| the ticket a member may read. Every change still goes through the
| writer, so the rules hold for automation as for people.
|
*/

package tickets

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AutomationWriter returns a writer acting as dimmi in the transaction.
*/
func (t *Tickets) AutomationWriter(ctx context.Context, tx *database.Tx, ws ids.UUID) (*Writer, error) {
	return t.systemWriter(ctx, tx, ws)
}

/*
Lock reads the ticket and holds its row until the transaction ends, for
the actions that work tickets beside this one.
*/
func (w *Writer) Lock(ctx context.Context, ws, id ids.UUID) (models.Ticket, error) {
	return w.lockRow(ctx, ws, id)
}

/*
Readable returns the live ticket the member may read, or
ErrTicketNotFound.
*/
func Readable(ctx context.Context, q *models.Queries, m authz.Membership, ref Ref) (models.Ticket, error) {
	if err := canRead(m); err != nil {
		return models.Ticket{}, err
	}

	return find(ctx, q, m, ref)
}
