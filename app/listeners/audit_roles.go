/*
|--------------------------------------------------------------------------
| Role Audit Entries
|--------------------------------------------------------------------------
|
| Custom roles decide what their holders may do, so every role made,
| edited or deleted lands in the audit log, edits with the permissions
| added and removed, and so does every member given or relieved of one.
|
*/

package listeners

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
listenRoles registers the entries of custom roles.
*/
func (l *WriteAuditLog) listenRoles(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.roleCreated)
	events.Listen(d, events.InTx, l.roleUpdated)
	events.Listen(d, events.InTx, l.roleDeleted)
	events.Listen(d, events.InTx, l.memberCustomRoleChanged)
}

/*
roleCreated records a new custom role with what it holds.
*/
func (l *WriteAuditLog) roleCreated(ctx context.Context, e appevents.RoleCreated) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventRoleCreated, ActorID: e.CreatedBy,
		Subject: audit.Subject{Type: subjectRole, ID: e.RoleID, Label: e.Name},
		Changes: map[string]any{
			"name": map[string]any{"to": e.Name}, "base_role": map[string]any{"to": e.BaseRole},
			"permissions": map[string]any{"added": e.Permissions, "removed": []string{}},
		},
	})
}

/*
roleUpdated records what changed on a custom role.
*/
func (l *WriteAuditLog) roleUpdated(ctx context.Context, e appevents.RoleUpdated) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventRoleUpdated, ActorID: e.UpdatedBy,
		Subject: audit.Subject{Type: subjectRole, ID: e.RoleID, Label: e.Name}, Changes: e.Changes,
	})
}

/*
roleDeleted records a custom role deleted.
*/
func (l *WriteAuditLog) roleDeleted(ctx context.Context, e appevents.RoleDeleted) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventRoleDeleted, ActorID: e.DeletedBy,
		Subject: audit.Subject{Type: subjectRole, ID: e.RoleID, Label: e.Name},
	})
}

/*
memberCustomRoleChanged records a member given a custom role or relieved
of one, naming the roles by their public ids.
*/
func (l *WriteAuditLog) memberCustomRoleChanged(ctx context.Context, e appevents.MemberCustomRoleChanged) error {
	return l.member(ctx, e.WorkspaceID, e.MemberID, audit.Entry{
		Event: audit.EventMemberCustomRole, ActorID: e.ChangedBy,
		Changes: map[string]any{
			"custom_role_id": map[string]any{"from": rolePublicID(e.From), "to": rolePublicID(e.To)},
		},
	})
}

/*
rolePublicID returns the role's public id, nil for none.
*/
func rolePublicID(id *ids.UUID) *string {
	if id == nil {
		return nil
	}

	public := models.PublicID(models.PrefixRole, *id)

	return &public
}
