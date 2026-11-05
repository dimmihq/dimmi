/*
|--------------------------------------------------------------------------
| Workspace List Command
|--------------------------------------------------------------------------
|
| dimmi workspace:list shows the operator every workspace on the server
| and whether it is active, suspended or in the trash, as a table for a
| person or as JSON for a script. Workspaces in the trash are left out
| unless asked for, since they are on their way out.
|
*/

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The states a workspace is listed in.
*/
const (
	workspaceActive    = "active"
	workspaceSuspended = "suspended"
	workspaceDeleted   = "deleted"
)

/*
listedWorkspace is one workspace as the JSON listing shows it.
*/
type listedWorkspace struct {
	ID              string     `json:"id"`
	Slug            string     `json:"slug"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	SuspendedReason *string    `json:"suspended_reason"`
	CreatedAt       time.Time  `json:"created_at"`
	DeletedAt       *time.Time `json:"deleted_at"`
	PurgeAfter      *time.Time `json:"purge_after"`
}

/*
WorkspaceListCommand lists the server's workspaces.
*/
type WorkspaceListCommand struct {
	App *app.App
}

var _ foundation.Command = WorkspaceListCommand{}

/*
Name returns workspace:list.
*/
func (WorkspaceListCommand) Name() string {
	return "workspace:list"
}

/*
Description says what the command does.
*/
func (WorkspaceListCommand) Description() string {
	return "List the workspaces on this server"
}

/*
Flags declares the switches for workspaces in the trash and for JSON.
*/
func (WorkspaceListCommand) Flags(flags *console.Flags) {
	flags.Bool("with-deleted", "include workspaces in the trash")
	flags.Bool("json", "print a JSON array instead of a table")
}

/*
Handle lists the workspaces.
*/
func (c WorkspaceListCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	q := models.New(c.App.DB)
	list := q.ListWorkspaces

	if in.Bool("with-deleted") {
		list = q.ListAllWorkspaces
	}

	all, err := list(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	if in.Bool("json") {
		return printWorkspaces(out, all)
	}

	rows := make([][]string, 0, len(all))
	for _, w := range all {
		rows = append(rows, []string{w.Slug, w.Name, workspaceStatus(w), w.CreatedAt.UTC().Format(time.DateTime)})
	}

	out.Table([]string{"Slug", "Name", "Status", "Created"}, rows)

	return nil
}

/*
printWorkspaces writes the workspaces as one JSON array.
*/
func printWorkspaces(out *console.Output, all []models.Workspace) error {
	listed := make([]listedWorkspace, 0, len(all))
	for _, w := range all {
		listed = append(listed, listedWorkspace{
			ID: models.PublicID(models.PrefixWorkspace, w.ID), Slug: w.Slug, Name: w.Name,
			Status: workspaceStatus(w), SuspendedReason: w.SuspendedReason, CreatedAt: w.CreatedAt,
			DeletedAt: w.DeletedAt, PurgeAfter: w.PurgeAfter,
		})
	}

	encoded, err := json.Marshal(listed)
	if err != nil {
		return fmt.Errorf("encode workspaces: %w", err)
	}

	out.Line("%s", encoded)

	return nil
}

/*
workspaceStatus names the state of the workspace: in the trash first,
then suspended, else active.
*/
func workspaceStatus(w models.Workspace) string {
	if w.DeletedAt != nil {
		return workspaceDeleted
	}

	if w.SuspendedAt != nil {
		return workspaceSuspended
	}

	return workspaceActive
}
