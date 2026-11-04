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
| client to drop what it showed: whatever it showed of the workspace is
| no longer theirs to see. The same goes for a credential that ends: a
| session signed out or revoked and a token revoked close the connections
| opened with them, and a reset or a taken-back account closes all of the
| user's. A change of which conversations someone sees closes their
| connections too: the client comes back at once and every channel is
| authorized again under the new rule.
|
*/

package broadcasting

import (
	"context"
	"fmt"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Disconnector closes connections of a user, on every node.
*/
type Disconnector interface {
	/*
		Disconnect closes every connection of the user.
	*/
	Disconnect(ctx context.Context, userID string) error

	/*
		DisconnectCredential closes the user's connections opened with the
		credential.
	*/
	DisconnectCredential(ctx context.Context, userID, credential string) error
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
Disconnects closes the connections of users who lose access and of
credentials that end.
*/
type Disconnects struct {
	Server Disconnector
}

/*
Listen makes the disconnects hear what ends access, once it commits.
*/
func (d *Disconnects) Listen(dispatcher *events.Dispatcher) {
	events.Listen(dispatcher, events.AfterCommit, func(ctx context.Context, e appevents.MemberDeactivated) error {
		return d.user(ctx, e.UserID)
	})
	events.Listen(dispatcher, events.AfterCommit, func(ctx context.Context, e appevents.MemberAccessChanged) error {
		return d.user(ctx, e.UserID)
	})
	events.Listen(dispatcher, events.AfterCommit, func(ctx context.Context, e appevents.UserSignedOutEverywhere) error {
		return d.user(ctx, e.UserID)
	})
	events.Listen(dispatcher, events.AfterCommit, func(ctx context.Context, e appevents.UserDeleted) error {
		return d.user(ctx, e.UserID)
	})
	events.Listen(dispatcher, events.AfterCommit, func(ctx context.Context, e appevents.SessionRevoked) error {
		return d.credential(ctx, e.UserID, appauth.KindSession, e.SessionID)
	})
	events.Listen(dispatcher, events.AfterCommit, func(ctx context.Context, e appevents.TokenRevoked) error {
		return d.credential(ctx, e.UserID, appauth.KindToken, e.TokenID)
	})
}

/*
user closes every connection of the user.
*/
func (d *Disconnects) user(ctx context.Context, userID ids.UUID) error {
	if err := d.Server.Disconnect(ctx, models.PublicID(models.PrefixUser, userID)); err != nil {
		return fmt.Errorf("disconnect user: %w", err)
	}

	return nil
}

/*
credential closes the user's connections opened with the session or
token.
*/
func (d *Disconnects) credential(ctx context.Context, userID ids.UUID, kind string, id ids.UUID) error {
	err := d.Server.DisconnectCredential(ctx, models.PublicID(models.PrefixUser, userID), Credential(kind, id))
	if err != nil {
		return fmt.Errorf("disconnect ended credential: %w", err)
	}

	return nil
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
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberAccessChanged) error {
		return b.publish(ctx, []string{MemberChannel(e.MemberID)}, fbroadcasting.Message{
			ID: ids.Encode("evt", ids.New()), Event: "member.access_changed",
			Data: map[string]string{"from": e.From.Mode, "to": e.To.Mode},
		})
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.UserUpdated) error {
		for _, m := range e.Memberships {
			data := changed{ID: models.PublicID(models.PrefixMember, m.MemberID), Object: "member", Changes: e.Changes}

			if err := b.account(ctx, m.WorkspaceID, "user.updated", data); err != nil {
				return err
			}
		}

		return nil
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
