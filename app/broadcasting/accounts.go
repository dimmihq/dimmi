/*
|--------------------------------------------------------------------------
| Workspace And Member Broadcasts
|--------------------------------------------------------------------------
|
| Teammates see the workspace change under them: its name, who joined,
| whose role changed, who left. These broadcasts only say what changed,
| by id, and screens fetch the rest, since what a member may see of
| another depends on their own permissions.
|
| Losing access ends it everywhere at once. A deactivated member's
| connections are closed on every node, with a reason that tells the
| client not to reconnect on its own: whatever it showed of the
| workspace is no longer theirs to see.
|
*/

package broadcasting

import (
	"context"
	"fmt"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Disconnector closes every connection of a user, on every node.
*/
type Disconnector interface {
	/*
		Disconnect closes the user's connections for good.
	*/
	Disconnect(ctx context.Context, userID string) error
}

/*
changed is the data of a workspace or member broadcast: which record,
and for a workspace the names of the fields that changed.
*/
type changed struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Changes []string `json:"changes,omitempty"`
}

/*
Disconnects closes the connections of members who lose access.
*/
type Disconnects struct {
	Server Disconnector
}

/*
Listen makes the disconnects hear deactivations, once they commit.
*/
func (d *Disconnects) Listen(dispatcher *events.Dispatcher) {
	events.Listen(dispatcher, events.AfterCommit, func(ctx context.Context, e appevents.MemberDeactivated) error {
		if err := d.Server.Disconnect(ctx, models.PublicID(models.PrefixUser, e.UserID)); err != nil {
			return fmt.Errorf("disconnect deactivated member: %w", err)
		}

		return nil
	})
}

/*
listenAccounts broadcasts the workspace and member events on the
workspace's channel.
*/
func (b *Broadcaster) listenAccounts(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.WorkspaceUpdated) error {
		data := changed{
			ID: models.PublicID(models.PrefixWorkspace, e.WorkspaceID), Object: "workspace", Changes: e.Changes,
		}

		return b.account(ctx, e.WorkspaceID, "workspace.updated", data)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberJoined) error {
		return b.member(ctx, e.WorkspaceID, "member.joined", e.MemberID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberRoleChanged) error {
		return b.member(ctx, e.WorkspaceID, "member.role_changed", e.MemberID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberDeactivated) error {
		return b.member(ctx, e.WorkspaceID, "member.deactivated", e.MemberID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberReactivated) error {
		return b.member(ctx, e.WorkspaceID, "member.reactivated", e.MemberID)
	})
}

func (b *Broadcaster) member(ctx context.Context, workspaceID ids.UUID, event string, memberID ids.UUID) error {
	id := models.PublicID(models.PrefixMember, memberID)

	return b.account(ctx, workspaceID, event, changed{ID: id, Object: "member"})
}

func (b *Broadcaster) account(ctx context.Context, workspaceID ids.UUID, event string, data changed) error {
	m := fbroadcasting.Message{ID: ids.Encode("evt", ids.New()), Event: event, Data: data}

	return b.publish(ctx, []string{WorkspaceChannel(workspaceID)}, m)
}
