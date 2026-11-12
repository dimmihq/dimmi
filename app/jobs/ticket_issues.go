/*
|--------------------------------------------------------------------------
| Linked Issue Jobs
|--------------------------------------------------------------------------
|
| When a ticket's links change, the customer impact written on its
| linked issues is refreshed outside the request, since every tool is a
| network call away. One job per ticket waits at a time.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
RefreshTicketImpactArgs names a ticket whose issues need the current
impact.
*/
type RefreshTicketImpactArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	TicketID    ids.UUID `json:"ticket_id"`
}

/*
Kind names the job.
*/
func (RefreshTicketImpactArgs) Kind() string { return "refresh_ticket_impact" }

/*
Options keep one waiting job per ticket.
*/
func (RefreshTicketImpactArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
ImpactWriter writes a ticket's impact to its linked issues.
*/
type ImpactWriter interface {
	/*
		RefreshImpact writes the ticket's current impact to its issues.
	*/
	RefreshImpact(ctx context.Context, workspaceID, ticketID ids.UUID) error
}

/*
RefreshTicketImpact writes a ticket's impact to its linked issues.
*/
type RefreshTicketImpact struct {
	Issues ImpactWriter
}

/*
Handle writes the impact.
*/
func (w *RefreshTicketImpact) Handle(ctx context.Context, job *queue.Job[RefreshTicketImpactArgs]) error {
	return w.Issues.RefreshImpact(ctx, job.Args.WorkspaceID, job.Args.TicketID)
}
