/*
|--------------------------------------------------------------------------
| Export Members
|--------------------------------------------------------------------------
|
| Those who manage a workspace's members take the list to a spreadsheet
| to review who has access: roles, teams, second factors and when each
| person last came by. The file follows the same filters as the list on
| screen and is read a page at a time, so a large workspace never holds
| the database. No cell can start a spreadsheet formula, since names and
| team names are typed by people.
|
| Every export is recorded once the file ends, with what it was narrowed
| to and how many members it held.
|
*/

package members

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/csvsafe"
)

/*
EventMembersExported is the audit event of a members export, and
exportPage how many members each read takes.
*/
const (
	EventMembersExported = "members.exported"
	exportPage           = 500
)

/*
errExportReaderGone means the reader of the file went away before its
end.
*/
var errExportReaderGone = errors.New("members export reader went away")

/*
MemberExportColumns are the columns of an exported member list, in
order. Custom roles do not exist yet, so that column stays empty.
*/
func MemberExportColumns() []string {
	return []string{
		"name", "email", "role", "custom_role", "status", "teams", "two_factor", "last_active_at", "joined_at",
	}
}

/*
ExportMembersInput is the list to export, narrowed like the list on
screen, and the filters as they were asked for, kept in the export's
audit entry.
*/
type ExportMembersInput struct {
	List  ListMembersInput
	Asked map[string][]string
}

/*
ExportMembers streams the members of the caller's workspace as CSV.

Rules:
  - The caller needs members:write.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ExportMembers struct {
	List   *ListMembers
	DB     *database.DB
	Record *audit.RecordAuditLog
}

/*
Filename checks the export may run and names its file, before anything
is written.
*/
func (a *ExportMembers) Filename(in ExportMembersInput) (string, error) {
	if !in.List.Membership.Can("members:write") {
		return "", exceptions.ErrForbidden
	}

	return fmt.Sprintf("dimmi-members-%s.csv", in.List.Membership.Workspace.Slug), nil
}

/*
Write streams the members to out by name, then records the export. A
reader that goes away ends the stream and is not an error; a list that
cannot be read is, after the entry is written.
*/
func (a *ExportMembers) Write(ctx context.Context, in ExportMembersInput, out io.Writer) error {
	if _, err := a.Filename(in); err != nil {
		return err
	}

	rows, err := a.stream(ctx, in, csvsafe.NewWriter(out))

	recordErr := a.record(context.WithoutCancel(ctx), in, rows, err == nil)
	if err != nil && !errors.Is(err, errExportReaderGone) {
		return err
	}

	return recordErr
}

/*
stream writes the header and every member, a page at a time, and
returns how many members the reader took.
*/
func (a *ExportMembers) stream(ctx context.Context, in ExportMembersInput, w *csvsafe.Writer) (int, error) {
	if err := sendRows(w, [][]string{MemberExportColumns()}); err != nil {
		return 0, err
	}

	list := in.List
	list.Sort, list.Offset, list.Limit = SortByName, 0, exportPage
	rows := 0

	for {
		page, err := a.List.Handle(ctx, list)
		if err != nil {
			return rows, exportReadFailure(ctx, err)
		}

		names, err := a.teamNames(ctx, list, page.Members)
		if err != nil {
			return rows, exportReadFailure(ctx, err)
		}

		batch := make([][]string, 0, len(page.Members))
		for _, row := range page.Members {
			batch = append(batch, memberRecord(row, names[row.WorkspaceMember.ID]))
		}

		if err := sendRows(w, batch); err != nil {
			return rows, err
		}

		rows += len(batch)
		if !page.More {
			return rows, nil
		}

		list.Offset += exportPage
	}
}

/*
teamNames returns the names of each listed member's teams, keyed by
member id.
*/
func (a *ExportMembers) teamNames(
	ctx context.Context, list ListMembersInput, rows []models.ListWorkspaceMembersRow,
) (map[ids.UUID][]string, error) {
	memberIDs := make([]ids.UUID, 0, len(rows))
	for _, row := range rows {
		memberIDs = append(memberIDs, row.WorkspaceMember.ID)
	}

	names := make(map[ids.UUID][]string, len(rows))

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		teams, err := models.New(tx).ListMemberTeams(ctx, models.ListMemberTeamsParams{
			WorkspaceID: list.Membership.Workspace.ID, MemberIds: memberIDs,
		})
		for _, team := range teams {
			names[team.WorkspaceMemberID] = append(names[team.WorkspaceMemberID], team.Team.Name)
		}

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list member teams: %w", err)
	}

	return names, nil
}

/*
memberRecord is the member's row of the file.
*/
func memberRecord(row models.ListWorkspaceMembersRow, teams []string) []string {
	m, u := row.WorkspaceMember, row.User
	lastActive := ""

	if m.LastAccessedAt != nil {
		lastActive = m.LastAccessedAt.UTC().Format(time.RFC3339)
	}

	return []string{
		u.Name, u.Email, m.Role, "", m.Status, strings.Join(teams, ";"),
		strconv.FormatBool(u.TwoFactorConfirmedAt != nil), lastActive, m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

/*
sendRows writes the records and hands them to the reader.
*/
func sendRows(w *csvsafe.Writer, records [][]string) error {
	for _, record := range records {
		if err := w.Write(record); err != nil {
			return fmt.Errorf("%w: %w", errExportReaderGone, err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("%w: %w", errExportReaderGone, err)
	}

	return nil
}

/*
exportReadFailure tells a reader who went away, which cancels the read,
from a list that could not be read.
*/
func exportReadFailure(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %w", errExportReaderGone, ctx.Err())
	}

	return err
}

/*
record writes the export's audit entry: what it was narrowed to, how
many members it held and whether the reader took them all.
*/
func (a *ExportMembers) record(ctx context.Context, in ExportMembersInput, rows int, completed bool) error {
	filters := map[string]any{}
	for name, values := range in.Asked {
		filters[name] = values
	}

	m := in.List.Membership
	_, err := a.Record.Handle(ctx, audit.Entry{
		WorkspaceID: m.Workspace.ID, Event: EventMembersExported, ActorID: m.Member.ID,
		Subject: audit.Subject{Type: "workspace", ID: m.Workspace.ID, Label: m.Workspace.Name},
		Changes: map[string]any{"filters": filters, "rows": rows, "completed": completed},
	})

	return err
}
