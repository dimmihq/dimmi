/*
|--------------------------------------------------------------------------
| List Members
|--------------------------------------------------------------------------
|
| Everyone in a workspace can see who else works in it: names, emails,
| roles and whether each member is still active. Lists come a page at a
| time, by name, newest first or most recently active first, narrowed by
| status, role, team or a search of names and emails. Whether a member
| has a second factor is only for those who manage members, so only they
| may narrow by it.
|
*/

package members

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The orders a list of members comes in.
*/
const (
	SortByName       = "name"
	SortNewestFirst  = "created_at"
	SortLastActivity = "last_active_at"
)

/*
ListMembersInput is who asks, through their membership, and which page
of which members they want. Statuses, Roles and TeamIDs narrow the
list, none meaning all; TwoFactor, when set, keeps members with or
without a second factor; Search matches names and emails; Sort is one
of the Sort constants, by name when empty.
*/
type ListMembersInput struct {
	Membership authz.Membership
	Statuses   []string
	Roles      []string
	TeamIDs    []ids.UUID
	TwoFactor  *bool
	Search     string
	Sort       string
	Offset     int
	Limit      int
}

/*
MemberPage is one page of members, and whether more follow.
*/
type MemberPage struct {
	Members []models.ListWorkspaceMembersRow
	More    bool
}

/*
ListMembers lists the members of the caller's workspace.

Rules:
  - The caller needs members:read.
  - Narrowing by second factor needs members:write.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListMembers struct {
	DB *database.DB
}

/*
Handle returns the page of members.
*/
func (a *ListMembers) Handle(ctx context.Context, in ListMembersInput) (MemberPage, error) {
	if !in.Membership.Can("members:read") || (in.TwoFactor != nil && !in.Membership.Can("members:write")) {
		return MemberPage{}, exceptions.ErrForbidden
	}

	var rows []models.ListWorkspaceMembersRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		rows, err = models.New(tx).ListWorkspaceMembers(ctx, models.ListWorkspaceMembersParams{
			WorkspaceID: in.Membership.Workspace.ID, Statuses: in.Statuses, Roles: in.Roles, TeamIds: in.TeamIDs,
			TwoFactor: in.TwoFactor, Search: in.Search, Sort: in.Sort,
			RowOffset: int64(in.Offset), RowLimit: int64(in.Limit + 1),
		})

		return err
	})
	if err != nil {
		return MemberPage{}, fmt.Errorf("list members: %w", err)
	}

	if len(rows) > in.Limit {
		return MemberPage{Members: rows[:in.Limit], More: true}, nil
	}

	return MemberPage{Members: rows}, nil
}

/*
ShowMemberInput is who asks, through their membership, and the member
they ask about.
*/
type ShowMemberInput struct {
	Membership authz.Membership
	MemberID   ids.UUID
}

/*
ShowMember returns one member of the caller's workspace with their
account.

Rules:
  - The caller needs members:read.
  - Members of other workspaces are not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowMember struct {
	DB *database.DB
}

/*
Handle returns the member.
*/
func (a *ShowMember) Handle(ctx context.Context, in ShowMemberInput) (models.FindWorkspaceMemberWithUserRow, error) {
	if !in.Membership.Can("members:read") {
		return models.FindWorkspaceMemberWithUserRow{}, exceptions.ErrForbidden
	}

	var row models.FindWorkspaceMemberWithUserRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		row, err = findWithUser(ctx, models.New(tx), in.Membership.Workspace.ID, in.MemberID)

		return err
	})

	return row, err
}

/*
findWithUser returns the member with their account, or
exceptions.ErrNotFound.
*/
func findWithUser(
	ctx context.Context, q *models.Queries, workspaceID, id ids.UUID,
) (models.FindWorkspaceMemberWithUserRow, error) {
	row, err := q.FindWorkspaceMemberWithUser(ctx, models.FindWorkspaceMemberWithUserParams{
		WorkspaceID: workspaceID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find member: %w", err)
	}

	return row, nil
}
