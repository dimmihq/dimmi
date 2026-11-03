/*
|--------------------------------------------------------------------------
| Member Changes
|--------------------------------------------------------------------------
|
| The writes behind every change to a member, shared by updating a
| member and leaving a workspace. Each write announces itself, and a
| change that could leave the workspace without an active owner counts
| the owners again before the transaction may commit.
|
| A deactivated member leaves every team and every token they minted
| stops working at once; their row stays, so their history keeps their
| name.
|
*/

package members

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
memberWriter writes changes to members inside one transaction, on
behalf of the member making them, at one moment.
*/
type memberWriter struct {
	tx     *database.Tx
	events *events.Dispatcher
	by     ids.UUID
	now    time.Time
}

/*
reactivate gives a member their access back with the role.
*/
func (w memberWriter) reactivate(
	ctx context.Context, m models.WorkspaceMember, role enums.Role,
) (models.WorkspaceMember, error) {
	member, err := models.New(w.tx).ReactivateWorkspaceMember(ctx, models.ReactivateWorkspaceMemberParams{
		WorkspaceID: m.WorkspaceID, ID: m.ID, Role: string(role), Now: w.now,
	})
	if err != nil {
		return member, fmt.Errorf("reactivate member: %w", err)
	}

	return member, w.events.Dispatch(ctx, w.tx, appevents.MemberReactivated{
		WorkspaceID: m.WorkspaceID, MemberID: m.ID, UserID: m.UserID, Role: member.Role, ReactivatedBy: w.by,
	})
}

/*
changeRole gives a member a new role.
*/
func (w memberWriter) changeRole(
	ctx context.Context, m models.WorkspaceMember, role enums.Role,
) (models.WorkspaceMember, error) {
	member, err := models.New(w.tx).UpdateWorkspaceMemberRole(ctx, models.UpdateWorkspaceMemberRoleParams{
		WorkspaceID: m.WorkspaceID, ID: m.ID, Role: string(role), Now: w.now,
	})
	if err != nil {
		return member, fmt.Errorf("change member role: %w", err)
	}

	return member, w.events.Dispatch(ctx, w.tx, appevents.MemberRoleChanged{
		WorkspaceID: m.WorkspaceID, MemberID: m.ID, UserID: m.UserID, From: m.Role, To: member.Role, ChangedBy: w.by,
	})
}

/*
deactivate ends a member's access: their row is marked, they leave
every team, and their tokens are revoked. The member named by
reassignTo takes their open work; ids.Nil() names nobody.
*/
func (w memberWriter) deactivate(ctx context.Context, m models.WorkspaceMember, reassignTo ids.UUID) error {
	q := models.New(w.tx)

	_, err := q.DeactivateWorkspaceMember(ctx, models.DeactivateWorkspaceMemberParams{
		WorkspaceID: m.WorkspaceID, ID: m.ID, DeactivatedBy: w.by, Now: w.now,
	})
	if err != nil {
		return fmt.Errorf("deactivate member: %w", err)
	}

	if err := w.leaveTeams(ctx, m); err != nil {
		return err
	}

	_, err = q.RevokeMemberTokens(ctx, models.RevokeMemberTokensParams{
		WorkspaceID: m.WorkspaceID, OwnerID: m.ID, RevokedBy: w.by, Now: w.now,
	})
	if err != nil {
		return fmt.Errorf("revoke member tokens: %w", err)
	}

	return w.events.Dispatch(ctx, w.tx, appevents.MemberDeactivated{
		WorkspaceID: m.WorkspaceID, MemberID: m.ID, UserID: m.UserID, DeactivatedBy: w.by, ReassignTo: reassignTo,
	})
}

/*
leaveTeams takes a member out of every team, announcing each team they
left. Reactivating them later does not put them back.
*/
func (w memberWriter) leaveTeams(ctx context.Context, m models.WorkspaceMember) error {
	teams, err := models.New(w.tx).RemoveMemberFromTeams(ctx, models.RemoveMemberFromTeamsParams{
		WorkspaceID: m.WorkspaceID, WorkspaceMemberID: m.ID,
	})
	if err != nil {
		return fmt.Errorf("remove member from teams: %w", err)
	}

	for _, team := range teams {
		err := w.events.Dispatch(ctx, w.tx, appevents.TeamMemberRemoved{
			WorkspaceID: m.WorkspaceID, TeamID: team, MemberID: m.ID, RemovedBy: w.by,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
keepsOwner refuses a change that left the workspace without an active
owner.
*/
func (w memberWriter) keepsOwner(ctx context.Context, workspaceID ids.UUID) error {
	owners, err := models.New(w.tx).CountActiveOwners(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("count owners: %w", err)
	}

	if owners == 0 {
		return exceptions.ErrLastOwner
	}

	return nil
}

/*
checkSuccessor refuses to hand ownership to a member who has not
verified their email, or who lacks the second factor the workspace
requires.
*/
func checkSuccessor(ctx context.Context, q *models.Queries, w models.Workspace, target models.WorkspaceMember) error {
	user, err := q.FindUser(ctx, target.UserID)
	if err != nil {
		return fmt.Errorf("load new owner: %w", err)
	}

	if !user.IsVerified() {
		return exceptions.ErrTargetUnverified
	}

	if requiresTwoFactor(w.Settings) && !user.TwoFactorEnabled() {
		return exceptions.ErrTargetTwoFactorRequired
	}

	return nil
}

/*
checkReassignee refuses to hand the target's open work to anyone but
another active member of the workspace.
*/
func checkReassignee(
	ctx context.Context, q *models.Queries, target models.WorkspaceMember, reassignee ids.UUID,
) error {
	member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
		WorkspaceID: target.WorkspaceID, ID: reassignee,
	})
	if err != nil || member.ID == target.ID || member.Status != string(enums.MemberStatusActive) {
		return exceptions.ErrReassignTargetInvalid
	}

	return nil
}

/*
requiresTwoFactor reports whether the workspace settings require every
member to use a second factor, under security.require_two_factor.
*/
func requiresTwoFactor(settings json.RawMessage) bool {
	var stored struct {
		Security struct {
			RequireTwoFactor bool `json:"require_two_factor"`
		} `json:"security"`
	}

	if err := json.Unmarshal(settings, &stored); err != nil {
		return false
	}

	return stored.Security.RequireTwoFactor
}
