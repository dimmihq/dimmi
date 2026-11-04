/*
|--------------------------------------------------------------------------
| Audit Log: Teams
|--------------------------------------------------------------------------
|
| Teams decide who sees and answers which conversations, so making,
| renaming and deleting one, and who joins or leaves it, are kept in the
| audit log. A team is named as it was called at the time, deleted or
| not.
|
*/

package listeners

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
listenTeams registers the entries of teams and their members.
*/
func (l *WriteAuditLog) listenTeams(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.teamCreated)
	events.Listen(d, events.InTx, l.teamUpdated)
	events.Listen(d, events.InTx, l.teamDeleted)
	events.Listen(d, events.InTx, l.teamMemberAdded)
	events.Listen(d, events.InTx, l.teamMemberRemoved)
}

/*
teamCreated records a new team.
*/
func (l *WriteAuditLog) teamCreated(ctx context.Context, e appevents.TeamCreated) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventTeamCreated, ActorID: e.CreatedBy,
		Subject: audit.Subject{Type: subjectTeam, ID: e.TeamID, Label: e.Name},
		Changes: map[string]any{"name": map[string]any{"to": e.Name}},
	})
}

/*
teamUpdated records a team's new name.
*/
func (l *WriteAuditLog) teamUpdated(ctx context.Context, e appevents.TeamUpdated) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventTeamUpdated, ActorID: e.UpdatedBy,
		Subject: audit.Subject{Type: subjectTeam, ID: e.TeamID, Label: e.To},
		Changes: map[string]any{"name": map[string]any{"from": e.From, "to": e.To}},
	})
}

/*
teamDeleted records a team deleted.
*/
func (l *WriteAuditLog) teamDeleted(ctx context.Context, e appevents.TeamDeleted) error {
	name, err := l.teamName(ctx, e.WorkspaceID, e.TeamID)
	if err != nil {
		return err
	}

	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventTeamDeleted, ActorID: e.DeletedBy,
		Subject: audit.Subject{Type: subjectTeam, ID: e.TeamID, Label: name},
	})
}

/*
teamMemberAdded records a member joining a team.
*/
func (l *WriteAuditLog) teamMemberAdded(ctx context.Context, e appevents.TeamMemberAdded) error {
	return l.teamMember(ctx, e.TeamID, e.MemberID, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventTeamMemberAdded, ActorID: e.AddedBy,
	})
}

/*
teamMemberRemoved records a member leaving a team.
*/
func (l *WriteAuditLog) teamMemberRemoved(ctx context.Context, e appevents.TeamMemberRemoved) error {
	return l.teamMember(ctx, e.TeamID, e.MemberID, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventTeamMemberRemoved, ActorID: e.RemovedBy,
	})
}

/*
teamMember records the entry about the member, naming the team. The
entry names the workspace.
*/
func (l *WriteAuditLog) teamMember(ctx context.Context, teamID, memberID ids.UUID, entry audit.Entry) error {
	name, err := l.teamName(ctx, entry.WorkspaceID, teamID)
	if err != nil {
		return err
	}

	entry.Changes = map[string]any{
		"team": map[string]any{"id": models.PublicID(models.PrefixTeam, teamID), "name": name},
	}

	return l.member(ctx, entry.WorkspaceID, memberID, entry)
}

/*
teamName returns what the team is called, deleted or not.
*/
func (l *WriteAuditLog) teamName(ctx context.Context, workspaceID, teamID ids.UUID) (string, error) {
	teams, err := models.New(l.DB).TeamsByIDs(ctx, models.TeamsByIDsParams{
		WorkspaceID: workspaceID, Ids: []ids.UUID{teamID},
	})
	if err != nil {
		return "", fmt.Errorf("find audited team: %w", err)
	}

	if len(teams) == 0 {
		return "", fmt.Errorf("find audited team: %s: not found", teamID)
	}

	return teams[0].Name, nil
}
