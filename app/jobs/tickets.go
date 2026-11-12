/*
|--------------------------------------------------------------------------
| Ticket Jobs
|--------------------------------------------------------------------------
|
| The work tickets do outside a request: telling a customer the state
| their ticket moved to, two minutes after the move and only once per
| ticket while it waits; sending a tracker's message to every linked
| customer; resolving tickets that waited on the customer too long;
| warning about and breaching service level clocks; re-checking linked
| issues in case a webhook was missed; and looking for spikes of
| conversations about one thing.
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
SendTicketUpdateArgs names a tracker's mass message to send.
*/
type SendTicketUpdateArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	UpdateID    ids.UUID `json:"update_id"`
}

/*
Kind names the job.
*/
func (SendTicketUpdateArgs) Kind() string { return "send_ticket_update" }

/*
Options keep one job per message.
*/
func (SendTicketUpdateArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

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
Kind names the job.
*/
func (DetectIncidentSpikesArgs) Kind() string { return "detect_incident_spikes" }

/*
TicketWork is what the ticket jobs run.
*/
type TicketWork interface {
	/*
		NotifyState tells a customer the state their ticket is in now.
	*/
	NotifyState(ctx context.Context, args NotifyTicketStateArgs) error

	/*
		SendUpdate sends a tracker's message to its pending recipients.
	*/
	SendUpdate(ctx context.Context, args SendTicketUpdateArgs) error

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

/*
SendTicketUpdate sends a tracker's message to its linked customers.
*/
type SendTicketUpdate struct {
	Work TicketWork
}

/*
Handle sends the message.
*/
func (w *SendTicketUpdate) Handle(ctx context.Context, job *queue.Job[SendTicketUpdateArgs]) error {
	return w.Work.SendUpdate(ctx, job.Args)
}
