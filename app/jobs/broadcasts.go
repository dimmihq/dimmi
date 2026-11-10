/*
|--------------------------------------------------------------------------
| Broadcast Jobs
|--------------------------------------------------------------------------
|
| A broadcast leaves in three steps. Starting freezes the audience into
| recipients; dispatching hands due recipients to senders at the pace
| the channel allows and comes back until nobody is left; sending
| reaches one person or channel. Every step can run twice without
| sending twice, and the hourly sweep restarts any broadcast whose job
| was lost.
|
*/

package jobs

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
StartBroadcastArgs freezes one broadcast's audience.
*/
type StartBroadcastArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	BroadcastID ids.UUID `json:"broadcast_id"`
}

/*
Kind names the job.
*/
func (StartBroadcastArgs) Kind() string { return "start_broadcast" }

/*
Options keep one start per broadcast waiting at a time.
*/
func (StartBroadcastArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 10, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
DispatchBroadcastArgs hands one broadcast's due recipients to senders.
*/
type DispatchBroadcastArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	BroadcastID ids.UUID `json:"broadcast_id"`
}

/*
Kind names the job.
*/
func (DispatchBroadcastArgs) Kind() string { return "dispatch_broadcast" }

/*
Options keep one dispatcher per broadcast waiting at a time.
*/
func (DispatchBroadcastArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 25, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
SendBroadcastRecipientArgs reaches one recipient of a broadcast.
*/
type SendBroadcastRecipientArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RecipientID ids.UUID `json:"recipient_id"`
}

/*
Kind names the job.
*/
func (SendBroadcastRecipientArgs) Kind() string { return "send_broadcast_recipient" }

/*
Options retry a passing failure a few times; the dispatcher's lease
brings a lost send back.
*/
func (SendBroadcastRecipientArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 5, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
SweepBroadcastsArgs names the hourly sweep that restarts broadcasts
and checks goals; it takes no arguments.
*/
type SweepBroadcastsArgs struct{}

/*
Kind names the job.
*/
func (SweepBroadcastsArgs) Kind() string { return "sweep_broadcasts" }

/*
Options run the sweep once an hour at most.
*/
func (SweepBroadcastsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByPeriod: time.Hour}}
}

/*
TallyBroadcastArgs counts one broadcast's results again and tells open
screens.
*/
type TallyBroadcastArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	BroadcastID ids.UUID `json:"broadcast_id"`
}

/*
Kind names the job.
*/
func (TallyBroadcastArgs) Kind() string { return "tally_broadcast" }

/*
Options keep one count per broadcast at a time, however much happens to
it meanwhile; a count that sees changes land while it runs is run
again.
*/
func (TallyBroadcastArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}
