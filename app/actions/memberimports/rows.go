/*
|--------------------------------------------------------------------------
| Member Import Rows
|--------------------------------------------------------------------------
|
| One row of an import asks for a person by address to hold a role, a
| status and a set of teams. Before anything changes, the row is read
| into what it wants and checked on its own; then it changes the member,
| invites the address, or reports why it cannot.
|
*/

package memberimports

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	fmail "github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
plan is one row to run with what every row shares: the workspace's
teams by lower-case name and the addresses earlier rows named.
*/
type plan struct {
	row   row
	teams map[string]ids.UUID
	seen  map[string]bool
}

/*
wanted is what a row asks for. A nil field, or nil teams, leaves the
member's own as it is.
*/
type wanted struct {
	email  string
	role   *enums.Role
	status *enums.MemberStatus
	teams  []ids.UUID
}

/*
row runs one row: changes the member with the address, or invites it.
*/
func (a *ImportMembers) row(ctx context.Context, in ImportMembersInput, p plan) (RowReport, error) {
	want, err := p.check()
	if err != nil {
		return failed(p.row, err)
	}

	member, err := a.find(ctx, in, want.email)
	if errors.Is(err, database.ErrNotFound) {
		return a.inviteAddress(ctx, job{in: in, row: p.row, want: want})
	}

	if err != nil {
		return RowReport{}, err
	}

	return a.change(ctx, job{in: in, row: p.row, want: want}, member)
}

/*
check reads the row into what it wants, refusing values that cannot
apply whoever the address belongs to.
*/
func (p plan) check() (wanted, error) {
	email := p.row.cell(columnEmail)
	if parsed, err := fmail.ParseAddress(email); err != nil || parsed.Email != email {
		return wanted{}, exceptions.ErrInvalidInvitationEmail
	}

	if p.seen[strings.ToLower(email)] {
		return wanted{}, exceptions.ErrImportDuplicateEmail
	}

	p.seen[strings.ToLower(email)] = true

	if p.row.cell(columnCustomRole) != "" {
		return wanted{}, exceptions.ErrImportCustomRole
	}

	want := wanted{email: email}

	if err := p.readRole(&want); err != nil {
		return wanted{}, err
	}

	if err := p.readStatus(&want); err != nil {
		return wanted{}, err
	}

	err := p.readTeams(&want)

	return want, err
}

/*
readRole reads the role cell; owner is never allowed.
*/
func (p plan) readRole(want *wanted) error {
	cell := strings.ToLower(p.row.cell(columnRole))

	switch enums.Role(cell) {
	case "":
		return nil
	case enums.RoleOwner:
		return exceptions.ErrImportRoleNotAllowed
	case enums.RoleAdmin, enums.RoleMember:
		role := enums.Role(cell)
		want.role = &role

		return nil
	default:
		return exceptions.ErrImportInvalidRole
	}
}

/*
readStatus reads the status cell.
*/
func (p plan) readStatus(want *wanted) error {
	cell := enums.MemberStatus(strings.ToLower(p.row.cell(columnStatus)))
	if cell == "" {
		return nil
	}

	if !slices.Contains(enums.MemberStatuses(), cell) {
		return exceptions.ErrImportInvalidStatus
	}

	want.status = &cell

	return nil
}

/*
readTeams reads the team names, separated by semicolons, into ids.
*/
func (p plan) readTeams(want *wanted) error {
	cell := p.row.cell(columnTeams)
	if cell == "" {
		return nil
	}

	want.teams = []ids.UUID{}

	for name := range strings.SplitSeq(cell, ";") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		id, ok := p.teams[strings.ToLower(name)]
		if !ok {
			return exceptions.ErrImportUnknownTeam.With("team", name)
		}

		if !slices.Contains(want.teams, id) {
			want.teams = append(want.teams, id)
		}
	}

	return nil
}

/*
find returns the workspace's member with the address, or
database.ErrNotFound.
*/
func (a *ImportMembers) find(
	ctx context.Context, in ImportMembersInput, email string,
) (models.WorkspaceMember, error) {
	var member models.WorkspaceMember

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		member, err = models.New(tx).FindWorkspaceMemberByEmail(ctx, models.FindWorkspaceMemberByEmailParams{
			WorkspaceID: in.Membership.Workspace.ID, Email: email,
		})

		return err
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return member, fmt.Errorf("find member: %w", err)
	}

	return member, err
}
