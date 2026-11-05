/*
|--------------------------------------------------------------------------
| Role Broadcasts
|--------------------------------------------------------------------------
|
| Role pickers on open screens refresh when a custom role is made, edited
| or deleted, and a member's own screen reloads what they may do when
| their custom role changes. Only ids travel; screens fetch the rest.
|
*/

package broadcasting

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
listenRoles broadcasts the custom role events on the workspace's
channel.
*/
func (b *Broadcaster) listenRoles(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.RoleCreated) error {
		return b.role(ctx, e.WorkspaceID, "role.created", e.RoleID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.RoleUpdated) error {
		return b.role(ctx, e.WorkspaceID, "role.updated", e.RoleID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.RoleDeleted) error {
		return b.role(ctx, e.WorkspaceID, "role.deleted", e.RoleID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MemberCustomRoleChanged) error {
		return b.member(ctx, e.WorkspaceID, "member.custom_role_changed", e.MemberID)
	})
}

/*
role broadcasts an event about the custom role.
*/
func (b *Broadcaster) role(ctx context.Context, workspaceID ids.UUID, event string, roleID ids.UUID) error {
	return b.account(ctx, workspaceID, event, changed{ID: models.PublicID(models.PrefixRole, roleID), Object: "role"})
}
