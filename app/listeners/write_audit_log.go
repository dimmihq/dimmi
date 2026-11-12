/*
|--------------------------------------------------------------------------
| Write Audit Log
|--------------------------------------------------------------------------
|
| Changes to a workspace and its members, its access tokens,
| invitations and teams, and its trash reach the audit log through their
| domain events, inside the transaction that dispatched them: the action
| never writes the log itself, and no change commits without its
| entry. A workspace update carries only the names of the fields that
| changed, so its entry records the values they changed to, read from
| the workspace as it now is.
|
*/

package listeners

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The kinds of subject the entries written here are about.
*/
const (
	subjectWorkspace  = "workspace"
	subjectMember     = "member"
	subjectToken      = "token"
	subjectInvitation = "invitation"
	subjectTeam       = "team"
	subjectRole       = "role"
	subjectConnection = "sso_connection"
	subjectDomain     = "organization_domain"
)

/*
WriteAuditLog records the audit entries of workspace and member events.
*/
type WriteAuditLog struct {
	DB     *database.DB
	Record *audit.RecordAuditLog
}

/*
Listen registers the listener for every event it records, in the
dispatching transaction.
*/
func (l *WriteAuditLog) Listen(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.workspaceCreated)
	events.Listen(d, events.InTx, l.workspaceUpdated)
	events.Listen(d, events.InTx, l.accessPolicyChanged)
	events.Listen(d, events.InTx, l.memberJoined)
	events.Listen(d, events.InTx, l.memberRoleChanged)
	events.Listen(d, events.InTx, l.memberDeactivated)
	events.Listen(d, events.InTx, l.memberReactivated)
	events.Listen(d, events.InTx, l.memberAccessChanged)
	l.listenTrash(d)
	l.listenAccess(d)
	l.listenTeams(d)
	l.listenRoles(d)
	l.listenSSO(d)
	l.listenAvailability(d)
	l.listenWidgets(d)
	l.listenSignInHelp(d)
	l.listenRedaction(d)
	l.listenErasures(d)
	l.listenMerges(d)
	l.listenDataExports(d)
}

/*
workspaceCreated records a new workspace, made by its first owner.
*/
func (l *WriteAuditLog) workspaceCreated(ctx context.Context, e appevents.WorkspaceCreated) error {
	q := models.New(l.DB)

	w, err := q.FindWorkspace(ctx, e.WorkspaceID)
	if err != nil {
		return fmt.Errorf("find audited workspace: %w", err)
	}

	owner, err := q.FindAuditActorByUser(ctx, models.FindAuditActorByUserParams{
		WorkspaceID: e.WorkspaceID, UserID: e.CreatedBy,
	})
	if err != nil {
		return fmt.Errorf("find workspace creator: %w", err)
	}

	return l.record(ctx, audit.Entry{
		WorkspaceID: w.ID, Event: audit.EventWorkspaceCreated, ActorID: owner.ID,
		Subject: audit.Subject{Type: subjectWorkspace, ID: w.ID, Label: w.Name},
		Changes: map[string]any{"name": map[string]any{"to": w.Name}, "slug": map[string]any{"to": w.Slug}},
	})
}

/*
workspaceUpdated records the new values of the fields that changed.
*/
func (l *WriteAuditLog) workspaceUpdated(ctx context.Context, e appevents.WorkspaceUpdated) error {
	w, err := models.New(l.DB).FindWorkspace(ctx, e.WorkspaceID)
	if err != nil {
		return fmt.Errorf("find audited workspace: %w", err)
	}

	values := map[string]string{"name": w.Name, "slug": w.Slug, "timezone": w.Timezone, "locale": w.Locale}
	changes := map[string]any{}

	for _, field := range e.Changes {
		changes[field] = map[string]any{"to": values[field]}
	}

	return l.record(ctx, audit.Entry{
		WorkspaceID: w.ID, Event: audit.EventWorkspaceUpdated, ActorID: e.UpdatedBy,
		Subject: audit.Subject{Type: subjectWorkspace, ID: w.ID, Label: w.Name}, Changes: changes,
	})
}

/*
accessPolicyChanged records a change to a workspace's rules with the
values it changed from and to, by the member who made it or, when an
operator did at the server, by the system.
*/
func (l *WriteAuditLog) accessPolicyChanged(ctx context.Context, e appevents.WorkspaceAccessPolicyChanged) error {
	w, err := models.New(l.DB).FindWorkspace(ctx, e.WorkspaceID)
	if err != nil {
		return fmt.Errorf("find audited workspace: %w", err)
	}

	return l.record(ctx, audit.Entry{
		WorkspaceID: w.ID, Event: audit.EventAccessPolicyChanged, ActorID: e.ChangedBy,
		Subject: audit.Subject{Type: subjectWorkspace, ID: w.ID, Label: w.Name}, Changes: e.Changes,
	})
}

/*
memberJoined records a member joining, as their own act.
*/
func (l *WriteAuditLog) memberJoined(ctx context.Context, e appevents.MemberJoined) error {
	return l.member(ctx, e.WorkspaceID, e.MemberID, audit.Entry{
		Event: audit.EventMemberJoined, ActorID: e.MemberID,
		Changes: map[string]any{"joined_via": e.JoinedVia},
	})
}

/*
memberRoleChanged records a new role, and the handover when the role
is owner.
*/
func (l *WriteAuditLog) memberRoleChanged(ctx context.Context, e appevents.MemberRoleChanged) error {
	entry := audit.Entry{
		Event: audit.EventMemberRoleChanged, ActorID: e.ChangedBy,
		Changes: map[string]any{"role": map[string]any{"from": e.From, "to": e.To}},
	}

	if err := l.member(ctx, e.WorkspaceID, e.MemberID, entry); err != nil {
		return err
	}

	if !appauth.IsOwnerRole(enums.Role(e.To)) {
		return nil
	}

	entry.Event = audit.EventOwnershipTransferred

	return l.member(ctx, e.WorkspaceID, e.MemberID, entry)
}

/*
memberAccessChanged records which conversations a member sees now and
what they saw before.
*/
func (l *WriteAuditLog) memberAccessChanged(ctx context.Context, e appevents.MemberAccessChanged) error {
	return l.member(ctx, e.WorkspaceID, e.MemberID, audit.Entry{
		Event: audit.EventMemberAccessChanged, ActorID: e.ChangedBy,
		Changes: map[string]any{"conversation_access": map[string]any{
			"from": auditedAccess(e.From), "to": auditedAccess(e.To),
		}},
	})
}

/*
auditedAccess is how the audit log writes one state of a member's
conversation access.
*/
func auditedAccess(s appevents.ConversationAccessState) map[string]any {
	teams := make([]string, len(s.TeamIDs))
	for i, id := range s.TeamIDs {
		teams[i] = models.PublicID(models.PrefixTeam, id)
	}

	return map[string]any{"mode": s.Mode, "unassigned": s.Unassigned, "team_ids": teams}
}

/*
memberDeactivated records a member losing access: deactivated by
someone else, or leaving on their own.
*/
func (l *WriteAuditLog) memberDeactivated(ctx context.Context, e appevents.MemberDeactivated) error {
	entry := audit.Entry{
		Event: audit.EventMemberDeactivated, ActorID: e.DeactivatedBy,
		Changes: map[string]any{"status": map[string]any{"from": "active", "to": "deactivated"}},
	}

	if e.DeactivatedBy == e.MemberID {
		entry.Event = audit.EventMemberLeft
	}

	if e.ReassignTo != ids.Nil() {
		entry.Changes["reassign_to_member_id"] = models.PublicID(models.PrefixMember, e.ReassignTo)
	}

	return l.member(ctx, e.WorkspaceID, e.MemberID, entry)
}

/*
memberReactivated records a member coming back, with the role they
came back with.
*/
func (l *WriteAuditLog) memberReactivated(ctx context.Context, e appevents.MemberReactivated) error {
	return l.member(ctx, e.WorkspaceID, e.MemberID, audit.Entry{
		Event: audit.EventMemberReactivated, ActorID: e.ReactivatedBy,
		Changes: map[string]any{
			"status": map[string]any{"from": "deactivated", "to": "active"}, "role": map[string]any{"to": e.Role},
		},
	})
}

/*
member records the entry about the member, labeled with their name,
read inside the member's workspace.
*/
func (l *WriteAuditLog) member(ctx context.Context, workspaceID, memberID ids.UUID, entry audit.Entry) error {
	var subject models.FindAuditActorRow

	err := l.DB.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error
		subject, err = models.New(tx).FindAuditActor(ctx, models.FindAuditActorParams{
			WorkspaceID: workspaceID, ID: memberID,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("find audited member: %w", err)
	}

	entry.WorkspaceID = workspaceID
	entry.Subject = audit.Subject{Type: subjectMember, ID: memberID, Label: subject.Name}

	return l.record(ctx, entry)
}

/*
record writes the entry.
*/
func (l *WriteAuditLog) record(ctx context.Context, entry audit.Entry) error {
	_, err := l.Record.Handle(ctx, entry)

	return err
}
