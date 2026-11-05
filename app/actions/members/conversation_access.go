/*
|--------------------------------------------------------------------------
| Conversation Access Changes
|--------------------------------------------------------------------------
|
| An admin decides how much of the workspace's conversations a member
| sees. Nobody changes their own access, so a restricted admin cannot lift
| their own limit, and owners always see everything: they are the last
| people able to reach any conversation, so making someone an owner gives
| them full sight and empties their list of teams.
|
| A change takes effect at once and is announced only when something
| really changed, because every announcement closes the member's realtime
| connections so that they subscribe again under the new rule.
|
*/

package members

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxAccessTeams is the most teams one member's access may name.
*/
const MaxAccessTeams = 200

/*
AccessChange is a requested change of a member's conversation access:
each field is nil when it stays as it is. TeamIDs replaces the list.
*/
type AccessChange struct {
	Mode       *enums.ConversationAccess
	Unassigned *bool
	TeamIDs    *[]ids.UUID
}

/*
planAccess checks the requested access change and works out the access
the member ends with, nil when it changes nothing.
*/
func planAccess(
	ctx context.Context, s subject, in UpdateMemberInput,
) (*appevents.ConversationAccessState, error) {
	if in.Access == nil {
		return nil, nil
	}

	if in.Membership.Member.ID == s.target.ID || !in.Membership.Can("members:manage_access") {
		return nil, exceptions.ErrForbidden
	}

	if appauth.IsOwnerRole(enums.Role(s.target.Role)) {
		return nil, exceptions.ErrOwnerAccessUnrestricted
	}

	from, err := currentAccess(ctx, s.q, s.target)
	if err != nil {
		return nil, err
	}

	to := nextAccess(from, in.Access)

	if err := checkAccessTeams(ctx, s.q, s.target.WorkspaceID, to); err != nil {
		return nil, err
	}

	if accessEqual(from, to) {
		return nil, nil
	}

	return &to, nil
}

/*
currentAccess returns the member's access as it is.
*/
func currentAccess(
	ctx context.Context, q *models.Queries, m models.WorkspaceMember,
) (appevents.ConversationAccessState, error) {
	teams, err := q.ListMemberAccessTeamIDs(ctx, models.ListMemberAccessTeamIDsParams{
		WorkspaceID: m.WorkspaceID, MemberID: m.ID,
	})
	if err != nil {
		return appevents.ConversationAccessState{}, fmt.Errorf("load access teams: %w", err)
	}

	return appevents.ConversationAccessState{
		Mode: m.ConversationAccess, Unassigned: m.ConversationAccessUnassigned, TeamIDs: teams,
	}, nil
}

/*
nextAccess returns the access the change leaves the member with. A mode
that reads no list empties it, unless the change names teams itself,
which checkAccessTeams then refuses.
*/
func nextAccess(from appevents.ConversationAccessState, c *AccessChange) appevents.ConversationAccessState {
	to := from

	if c.Mode != nil {
		to.Mode = string(*c.Mode)
	}

	if c.Unassigned != nil {
		to.Unassigned = *c.Unassigned
	}

	if c.TeamIDs != nil {
		to.TeamIDs = *c.TeamIDs
	} else if !enums.ConversationAccess(to.Mode).UsesTeamList() {
		to.TeamIDs = nil
	}

	return to
}

/*
checkAccessTeams refuses a list a mode cannot use, one too long and one
naming a team the workspace does not have.
*/
func checkAccessTeams(
	ctx context.Context, q *models.Queries, workspaceID ids.UUID, to appevents.ConversationAccessState,
) error {
	if len(to.TeamIDs) == 0 {
		return nil
	}

	if !enums.ConversationAccess(to.Mode).UsesTeamList() {
		return exceptions.ErrAccessTeamsNotUsed
	}

	if len(to.TeamIDs) > MaxAccessTeams {
		return exceptions.ErrTooManyAccessTeams
	}

	for _, id := range to.TeamIDs {
		_, err := q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: workspaceID, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrAccessTeamNotFound
		}

		if err != nil {
			return fmt.Errorf("find access team: %w", err)
		}
	}

	return nil
}

/*
accessEqual reports whether two accesses show the same conversations,
whatever order their teams are named in.
*/
func accessEqual(a, b appevents.ConversationAccessState) bool {
	if a.Mode != b.Mode || a.Unassigned != b.Unassigned || len(a.TeamIDs) != len(b.TeamIDs) {
		return false
	}

	for _, id := range a.TeamIDs {
		if !slices.Contains(b.TeamIDs, id) {
			return false
		}
	}

	return true
}

/*
changeAccess writes the member's new access and announces it.
*/
func (w memberWriter) changeAccess(
	ctx context.Context, m models.WorkspaceMember, to appevents.ConversationAccessState,
) error {
	q := models.New(w.tx)

	from, err := currentAccess(ctx, q, m)
	if err != nil {
		return err
	}

	_, err = q.SetMemberConversationAccess(ctx, models.SetMemberConversationAccessParams{
		WorkspaceID: m.WorkspaceID, ID: m.ID, ConversationAccess: to.Mode,
		ConversationAccessUnassigned: to.Unassigned, Now: w.now,
	})
	if err != nil {
		return fmt.Errorf("change conversation access: %w", err)
	}

	err = q.ClearMemberAccessTeams(ctx, models.ClearMemberAccessTeamsParams{WorkspaceID: m.WorkspaceID, MemberID: m.ID})
	if err != nil {
		return fmt.Errorf("clear access teams: %w", err)
	}

	err = q.AddMemberAccessTeams(ctx, models.AddMemberAccessTeamsParams{
		WorkspaceID: m.WorkspaceID, MemberID: m.ID, TeamIds: to.TeamIDs, Now: w.now,
	})
	if err != nil {
		return fmt.Errorf("add access teams: %w", err)
	}

	return w.events.Dispatch(ctx, w.tx, appevents.MemberAccessChanged{
		WorkspaceID: m.WorkspaceID, MemberID: m.ID, UserID: m.UserID, From: from, To: to, ChangedBy: w.by,
	})
}

/*
liftAccess gives a member who just became an owner sight of everything,
when they did not have it.
*/
func (w memberWriter) liftAccess(ctx context.Context, m models.WorkspaceMember) error {
	if enums.ConversationAccess(m.ConversationAccess) == enums.ConversationAccessAll {
		return nil
	}

	return w.changeAccess(ctx, m, appevents.ConversationAccessState{
		Mode: string(enums.ConversationAccessAll),
	})
}

/*
ListAccessTeamsInput is who asks and about which members.
*/
type ListAccessTeamsInput struct {
	Membership authz.Membership
	Members    []models.WorkspaceMember
}

/*
ListAccessTeams returns the teams the conversation access of each of the
members names.

Rules:
  - A caller without members:read learns nothing: a token that may only
    change a member gets no list back.
  - Members whose mode reads no list cost no query.

Errors: a wrapped database error.
*/
type ListAccessTeams struct {
	DB *database.DB
}

/*
Handle returns the teams by member.
*/
func (a *ListAccessTeams) Handle(ctx context.Context, in ListAccessTeamsInput) (map[ids.UUID][]ids.UUID, error) {
	memberIDs := make([]ids.UUID, 0, len(in.Members))
	for _, m := range in.Members {
		if enums.ConversationAccess(m.ConversationAccess).UsesTeamList() {
			memberIDs = append(memberIDs, m.ID)
		}
	}

	out := map[ids.UUID][]ids.UUID{}
	if len(memberIDs) == 0 || !in.Membership.Can("members:read") {
		return out, nil
	}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ListAccessTeamIDsOfMembers(ctx, models.ListAccessTeamIDsOfMembersParams{
			WorkspaceID: in.Membership.Workspace.ID, MemberIds: memberIDs,
		})
		if err != nil {
			return fmt.Errorf("list access teams: %w", err)
		}

		for _, row := range rows {
			out[row.MemberID] = append(out[row.MemberID], row.TeamID)
		}

		return nil
	})

	return out, err
}
