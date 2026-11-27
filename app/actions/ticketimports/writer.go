/*
|--------------------------------------------------------------------------
| Ticket Writer
|--------------------------------------------------------------------------
|
| The part of an import that writes, whichever tool the tickets came
| from: it maps a catalog of types, fields and states onto the
| workspace's, then brings tickets in through that mapping, replaying
| their states at their own times and setting nothing off. It runs in
| its caller's transaction and hands every record it makes or finds to
| a ledger, so an import from Intercom or Zendesk and a migration
| through the import format both keep what they touched their own way.
|
*/

package ticketimports

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Kept is one record a writer made or found: its kind (type, field,
state, ticket or link), its id in the other tool, its row here, and
whether the writer made it.
*/
type Kept struct {
	Kind     string
	External string
	ID       ids.UUID
	Created  bool
}

/*
Ledger keeps the records a writer made or found.
*/
type Ledger interface {
	// Keep keeps one record, inside the writer's transaction.
	Keep(ctx context.Context, q *models.Queries, k Kept) error
}

/*
Writer writes tickets of another tool into one workspace. Actor names
who the timeline says made the changes, and ActorID makes that a member
of the workspace. Mapping is filled by MapSetup, or by the caller from
an earlier run.
*/
type Writer struct {
	Clock       clock.Clock
	Events      *events.Dispatcher
	Tickets     *tickets.Tickets
	WorkspaceID ids.UUID
	Actor       string
	ActorID     *ids.UUID
	Ledger      Ledger
	Mapping     Mapping
	added       counts
}

/*
add adds what another step added.
*/
func (c *counts) add(o counts) {
	c.types += o.types
	c.fields += o.fields
	c.states += o.states
}

/*
ensureMapping makes the mapping ready to fill.
*/
func (w *Writer) ensureMapping() {
	if w.Mapping.Types == nil {
		w.Mapping.Types = map[string]models.TicketType{}
	}

	if w.Mapping.States == nil {
		w.Mapping.States = map[string]ids.UUID{}
	}
}

/*
keep hands the record to the ledger, when there is one.
*/
func (w *Writer) keep(ctx context.Context, q *models.Queries, t touched) error {
	if w.Ledger == nil {
		return nil
	}

	return w.Ledger.Keep(ctx, q, Kept{Kind: t.kind, External: t.external, ID: t.id, Created: t.created})
}
