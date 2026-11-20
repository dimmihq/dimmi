/*
|--------------------------------------------------------------------------
| Ticket Jobs
|--------------------------------------------------------------------------
|
| The work tickets do outside a request: telling a customer the state
| their ticket moved to, two minutes after the move and only once per
| ticket while it waits; resolving tickets that waited on the customer
| too long; warning about and breaching service level clocks;
| re-checking linked issues in case a webhook was missed; and looking
| for spikes of conversations about one thing.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
NotifyTicketStateArgs names a ticket whose customer may need telling.
Every move queues one; each sends only the state the ticket is in when
it runs and only once per state, so moves in quick succession send
only the last.
*/
type NotifyTicketStateArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	TicketID    ids.UUID `json:"ticket_id"`
}

/*
Kind names the job.
*/
func (NotifyTicketStateArgs) Kind() string { return "notify_ticket_state" }

/*
Kind names the job.
*/
func (SweepTicketsArgs) Kind() string { return "sweep_tickets" }

/*
Kind names the job.
*/
func (SweepSLAClocksArgs) Kind() string { return "sweep_sla_clocks" }

/*
Kind names the job.
*/
func (SyncTicketIssuesArgs) Kind() string { return "sync_ticket_issues" }

/*
TicketWork is what the ticket jobs run.
*/
type TicketWork interface {
	/*
		NotifyState tells a customer the state their ticket is in now.
	*/
	NotifyState(ctx context.Context, args NotifyTicketStateArgs) error

	/*
		Sweep resolves tickets that waited on their customer too long.
	*/
	Sweep(ctx context.Context) error
}

/*
NotifyTicketState tells a customer the state of their ticket.
*/
type NotifyTicketState struct {
	Work TicketWork
}

/*
Handle sends the notice.
*/
func (w *NotifyTicketState) Handle(ctx context.Context, job *queue.Job[NotifyTicketStateArgs]) error {
	return w.Work.NotifyState(ctx, job.Args)
}
