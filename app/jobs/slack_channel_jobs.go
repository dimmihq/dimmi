/*
|--------------------------------------------------------------------------
| Slack Channel Jobs
|--------------------------------------------------------------------------
|
| Work on Slack that follows something done in the inbox, run after the
| change committed so the inbox never waits on Slack: the closed mark on
| a thread, the buttons taken off an answer once a teammate replied, a
| deleted reply deleted in Slack too. And the list of channels the bot
| is in, refreshed when a connection is made or asked for.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
Limits of the Slack jobs: how often each is tried, and how soon a room
sync may run again for the same connection.
*/
const (
	slackEffectAttempts = 5
	roomSyncAttempts    = 5
	roomSyncPeriod      = time.Minute
)

/*
The effects an inbox change has on Slack.
*/
const (
	SlackEffectClosed   = "closed"
	SlackEffectReopened = "reopened"
	SlackEffectAnswered = "answered"
	SlackEffectDeleted  = "deleted"
)

/*
ApplySlackEffectArgs names the change: what it does on Slack, the
conversation, and the message it is about when it is about one.
*/
type ApplySlackEffectArgs struct {
	WorkspaceID    ids.UUID  `json:"workspace_id"`
	ConversationID ids.UUID  `json:"conversation_id"`
	MessageID      *ids.UUID `json:"message_id,omitempty"`
	Effect         string    `json:"effect"`
}

/*
Kind names the job.
*/
func (ApplySlackEffectArgs) Kind() string {
	return "apply_slack_effect"
}

/*
Options try the job a few times.
*/
func (ApplySlackEffectArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: slackEffectAttempts}
}

/*
SlackEffects applies an inbox change on Slack.
*/
type SlackEffects interface {
	/*
		Apply applies the change the args name.
	*/
	Apply(ctx context.Context, args ApplySlackEffectArgs) error
}

/*
ApplySlackEffect runs the Slack side of inbox changes.
*/
type ApplySlackEffect struct {
	Effects SlackEffects
}

/*
Handle applies the change.
*/
func (w *ApplySlackEffect) Handle(ctx context.Context, job *queue.Job[ApplySlackEffectArgs]) error {
	return w.Effects.Apply(ctx, job.Args)
}

/*
SyncSlackRoomsArgs names the connection whose rooms to refresh.
*/
type SyncSlackRoomsArgs struct {
	WorkspaceID  ids.UUID `json:"workspace_id"`
	ConnectionID ids.UUID `json:"connection_id"`
}

/*
Kind names the job.
*/
func (SyncSlackRoomsArgs) Kind() string {
	return "sync_channel_rooms"
}

/*
Options run one sync per connection a minute at most.
*/
func (SyncSlackRoomsArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: roomSyncAttempts, Unique: queue.UniqueOpts{ByArgs: true, ByPeriod: roomSyncPeriod},
	}
}

/*
RoomSyncer refreshes a connection's rooms.
*/
type RoomSyncer interface {
	/*
		SyncRooms refreshes the rooms of the connection.
	*/
	SyncRooms(ctx context.Context, workspaceID, connectionID ids.UUID) error
}

/*
SyncSlackRooms runs room syncs.
*/
type SyncSlackRooms struct {
	Syncer RoomSyncer
}

/*
Handle refreshes the rooms.
*/
func (w *SyncSlackRooms) Handle(ctx context.Context, job *queue.Job[SyncSlackRoomsArgs]) error {
	return w.Syncer.SyncRooms(ctx, job.Args.WorkspaceID, job.Args.ConnectionID)
}
