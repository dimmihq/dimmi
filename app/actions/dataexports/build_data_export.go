/*
|--------------------------------------------------------------------------
| Build a Data Export
|--------------------------------------------------------------------------
|
| The file is built as the member who asked for it sees the workspace
| when the build runs: rows they lost access to since are left out, and
| a member who left or lost the permission gets nothing. The file sits
| in the workspace's folder until the workspace's export period runs
| out, so a workspace purge removes it with everything else.
|
*/

package dataexports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BuildDataExport builds a requested export.
*/
type BuildDataExport struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Clock    clock.Clock
	Storage  *filesystem.Manager
	DiskName string
	Selector conversations.Selector
}

/*
Handle builds the export inside its workspace, records a failure before
returning it so the member sees it, and leaves an export already built
or gone alone.
*/
func (a *BuildDataExport) Handle(ctx context.Context, args BuildArgs) error {
	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	var row models.DataExport

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		row, err = models.New(tx).StartDataExport(ctx, models.StartDataExportParams{
			Now: a.Clock.Now(), WorkspaceID: args.WorkspaceID, ID: args.ExportID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("start export: %w", err)
	}

	if err := a.build(ctx, row); err != nil {
		return fmt.Errorf("build export: %w", errors.Join(err, a.fail(ctx, row, err)))
	}

	return nil
}

/*
fail records why the build failed, so the member sees it.
*/
func (a *BuildDataExport) fail(ctx context.Context, row models.DataExport, cause error) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).FailDataExport(ctx, models.FailDataExportParams{
			Error: new(cause.Error()), Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
	})
}

/*
build writes the file, stores it and marks the export ready.
*/
func (a *BuildDataExport) build(ctx context.Context, row models.DataExport) error {
	var (
		out  packed
		rows int
		ws   models.Workspace
	)

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		m, err := requester(ctx, models.New(tx), row)
		if err != nil {
			return err
		}

		ws = m.Workspace
		out, rows, err = a.render(ctx, tx, m, row)

		return err
	})
	if err != nil {
		return err
	}

	disk, err := a.Storage.Disk(a.DiskName)
	if err != nil {
		return fmt.Errorf("open disk %s: %w", a.DiskName, err)
	}

	path := "workspaces/" + row.WorkspaceID.String() + "/data-exports/" + row.ID.String() + "/" + out.name
	if err := disk.Put(ctx, path, out.data, filesystem.PutOptions{ContentType: out.contentType}); err != nil {
		return fmt.Errorf("store export: %w", err)
	}

	ttl := time.Duration(ws.Retention().ExportDays) * 24 * time.Hour

	return a.ready(ctx, row, finished{path: path, size: int64(len(out.data)), rows: rows, ttl: ttl})
}

/*
requester returns the membership of the member who asked, as it stands
now, refusing one who left or may no longer take the export.
*/
func requester(ctx context.Context, q *models.Queries, row models.DataExport) (authz.Membership, error) {
	if row.RequestedByMemberID == nil {
		return authz.Membership{}, exceptions.ErrForbidden
	}

	ws, err := q.FindWorkspace(ctx, row.WorkspaceID)
	if err != nil {
		return authz.Membership{}, fmt.Errorf("find workspace: %w", err)
	}

	member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
		WorkspaceID: row.WorkspaceID, ID: *row.RequestedByMemberID,
	})
	if err != nil {
		return authz.Membership{}, fmt.Errorf("find requester: %w", err)
	}

	role := enums.Role(member.Role)
	m := authz.NewMembership(ws, member, auth.RolePermissions(role))

	notes := !row.IncludeNotes || m.Can("conversations:moderate")
	if member.Status != string(enums.MemberStatusActive) || !m.Can(permissionOf(row.Kind)) || !notes {
		return m, exceptions.ErrForbidden
	}

	m.AccessTeams, err = authz.LoadAccessTeams(ctx, q, member)

	return m, err
}

/*
render reads the export's rows and writes its files.
*/
func (a *BuildDataExport) render(
	ctx context.Context, tx *database.Tx, m authz.Membership, row models.DataExport,
) (packed, int, error) {
	var (
		files []file
		rows  int
		err   error
	)

	if row.Kind == KindContacts {
		files, rows, err = renderContacts(ctx, models.New(tx), row)
	} else {
		files, rows, err = a.renderConversations(ctx, tx, m, row)
	}

	if err != nil {
		return packed{}, 0, err
	}

	out, err := pack(files)

	return out, rows, err
}

/*
renderConversations writes the conversations the member may see.
*/
func (a *BuildDataExport) renderConversations(
	ctx context.Context, tx *database.Tx, m authz.Membership, row models.DataExport,
) ([]file, int, error) {
	var sel conversations.Selection
	if err := json.Unmarshal(row.Selection, &sel); err != nil {
		return nil, 0, fmt.Errorf("read selection: %w", err)
	}

	loaded, err := a.Selector.Select(ctx, tx, conversations.Reading{
		Membership: m, Selection: sel, Limit: limits()[row.Kind][row.Format],
	})
	if err != nil {
		return nil, 0, err
	}

	files, err := writeConversations(ctx, models.New(tx), loaded, row)

	return files, len(loaded), err
}

/*
renderContacts writes the customers of the selection.
*/
func renderContacts(ctx context.Context, q *models.Queries, row models.DataExport) ([]file, int, error) {
	var sel ContactSelection
	if err := json.Unmarshal(row.Selection, &sel); err != nil {
		return nil, 0, fmt.Errorf("read selection: %w", err)
	}

	details, err := selectContacts(ctx, q, contactRead{
		ws: row.WorkspaceID, sel: sel, limit: limits()[row.Kind][row.Format],
	})
	if err != nil {
		return nil, 0, err
	}

	rows := contactRows(details)

	if row.Format == "csv" {
		files, err := contactSheet(rows)

		return files, len(rows), err
	}

	files, err := data("contacts.json", rows)

	return files, len(rows), err
}

/*
finished is where a built export was stored, what it holds and how long
it is kept.
*/
type finished struct {
	path string
	size int64
	rows int
	ttl  time.Duration
}

/*
ready marks the export ready to download until it expires and tells the
member who asked.
*/
func (a *BuildDataExport) ready(ctx context.Context, row models.DataExport, f finished) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		now := a.Clock.Now()

		if _, err := models.New(tx).FinishDataExport(ctx, models.FinishDataExportParams{
			Disk: &a.DiskName, Path: &f.path, SizeBytes: &f.size, RowCount: new(int64(f.rows)),
			Warnings: json.RawMessage(`[]`), ExpiresAt: new(now.Add(f.ttl)), Now: now,
			WorkspaceID: row.WorkspaceID, ID: row.ID,
		}); err != nil {
			return fmt.Errorf("finish export: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.DataExportReady{
			Base: appevents.Base{
				ID: ids.New(), WorkspaceID: row.WorkspaceID, OccurredAt: now,
				Actor: appevents.Actor{Type: "system", ID: ids.Nil()},
			},
			ExportID: row.ID, RequestedBy: *row.RequestedByMemberID, Rows: f.rows,
		})
	})
}
