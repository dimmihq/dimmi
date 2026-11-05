/*
|--------------------------------------------------------------------------
| Import Members
|--------------------------------------------------------------------------
|
| Admins of a large workspace change members in bulk from a
| spreadsheet: roles, teams and status by email address, and invitations
| for addresses that are not members yet. Each row goes through the same
| actions as a change made by hand, in a transaction of its own, so
| events and the audit log follow as usual and one bad row never stops
| the others. A dry run reads the file the same way and reports what
| would happen without changing anything.
|
| An owner is never made, changed or removed by a file: ownership moves
| only by hand, with the care that takes.
|
*/

package memberimports

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/teams"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
importsPerHour is how many imports a workspace may run an hour.
*/
const importsPerHour = 5

/*
The results of one row: a new invitation, a changed member, nothing to
change, and a row that could not apply.
*/
const (
	ResultCreated   = "created"
	ResultUpdated   = "updated"
	ResultUnchanged = "unchanged"
	ResultFailed    = "failed"
)

/*
ImportMembersInput is who imports, as a signed-in person and through
their membership, the file, and whether this is a dry run.
*/
type ImportMembersInput struct {
	Principal  *appauth.Principal
	Membership authz.Membership
	File       io.Reader
	DryRun     bool
}

/*
RowReport is what became of one row: its line in the file, its address,
the result, the fields it changed, and why it failed.
*/
type RowReport struct {
	Row     int
	Email   string
	Result  string
	Changes []string
	Err     error
}

/*
Report is what became of a whole file, counted by result, row by row.
*/
type Report struct {
	DryRun    bool
	Created   int
	Updated   int
	Unchanged int
	Failed    int
	Rows      []RowReport
}

/*
ImportMembers applies a member import file to the caller's workspace.

Rules:
  - The caller needs members:import.
  - A workspace imports at most 5 times an hour, dry runs included.
  - A file holds at most 1,000 rows under a header naming known columns,
    email among them.
  - An empty cell leaves the field as it is; a new person joins as a
    member unless the row says admin.
  - A known address changes the member's role, status and teams; an
    unknown one is invited with the role and teams.
  - No row makes, changes or removes an owner, deactivates an unknown
    address, or names a team the workspace does not have.

Errors: [exceptions.ErrForbidden], [exceptions.ErrTooManyImports], the
file errors of [exceptions.ErrImportUnreadable] and its group, or a
wrapped error. Row failures are reported, not returned.
*/
type ImportMembers struct {
	DB      *database.DB
	Limiter *ratelimit.Limiter
	Update  *members.UpdateMember
	Add     *teams.AddTeamMembers
	Remove  *teams.RemoveTeamMember
	Invite  *invitations.CreateInvitations
}

/*
Handle reads the file and applies, or with DryRun only checks, each row.
*/
func (a *ImportMembers) Handle(ctx context.Context, in ImportMembersInput) (Report, error) {
	if !in.Membership.Can("members:import") {
		return Report{}, exceptions.ErrForbidden
	}

	if err := a.throttle(ctx, in.Membership); err != nil {
		return Report{}, err
	}

	rows, err := readFile(in.File)
	if err != nil {
		return Report{}, err
	}

	teamIDs, err := a.teamsByName(ctx, in.Membership.Workspace.ID)
	if err != nil {
		return Report{}, err
	}

	report := Report{DryRun: in.DryRun, Rows: make([]RowReport, 0, len(rows))}
	seen := map[string]bool{}

	for _, r := range rows {
		outcome, err := a.row(ctx, in, plan{row: r, teams: teamIDs, seen: seen})
		if err != nil {
			return Report{}, err
		}

		report.add(outcome)
	}

	return report, nil
}

/*
add counts the row's result and keeps its report.
*/
func (r *Report) add(row RowReport) {
	switch row.Result {
	case ResultCreated:
		r.Created++
	case ResultUpdated:
		r.Updated++
	case ResultUnchanged:
		r.Unchanged++
	default:
		r.Failed++
	}

	r.Rows = append(r.Rows, row)
}

/*
throttle counts the import against the workspace's hourly allowance.
*/
func (a *ImportMembers) throttle(ctx context.Context, m authz.Membership) error {
	res, err := a.Limiter.Attempt(ctx, "member_import:"+m.Workspace.ID.String(), importsPerHour, time.Hour)
	if err != nil {
		return appauth.Unavailable(ctx, err)
	}

	if !res.Allowed {
		return exceptions.ErrTooManyImports.With("retry_after", max(int(res.RetryAfter.Seconds()), 1))
	}

	return nil
}

/*
teamsByName returns the workspace's live teams keyed by their name in
lower case.
*/
func (a *ImportMembers) teamsByName(ctx context.Context, workspaceID ids.UUID) (map[string]ids.UUID, error) {
	out := map[string]ids.UUID{}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ListTeams(ctx, models.ListTeamsParams{
			WorkspaceID: workspaceID, RowLimit: MaxRows * 10,
		})
		for _, row := range rows {
			out[strings.ToLower(row.Team.Name)] = row.Team.ID
		}

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}

	return out, nil
}

/*
failed reports a row that could not apply, keeping row failures apart
from errors that end the import.
*/
func failed(r row, err error) (RowReport, error) {
	if !isRowFailure(err) {
		return RowReport{}, err
	}

	return RowReport{Row: r.number, Email: r.cell(columnEmail), Result: ResultFailed, Err: err}, nil
}

/*
isRowFailure reports whether the error refuses one row only: anything
the actions refuse on the row's account, as opposed to an outage.
*/
func isRowFailure(err error) bool {
	var refused interface{ Code() string }

	return errors.As(err, &refused)
}
