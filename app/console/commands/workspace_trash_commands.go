/*
|--------------------------------------------------------------------------
| Workspace Trash Commands
|--------------------------------------------------------------------------
|
| dimmi workspace:restore takes a workspace out of the trash during its
| grace period, for owners who cannot do it themselves. dimmi
| workspace:purge deletes a workspace in the trash for good, now. A
| purge cannot be undone, so it asks for --force and refuses any
| workspace that is not already in the trash.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
ErrForceRequired refuses a purge without --force.
*/
var ErrForceRequired = errors.New("a purge cannot be undone; run again with --force")

/*
WorkspaceRestoreCommand takes a workspace out of the trash.
*/
type WorkspaceRestoreCommand struct {
	App *app.App
}

var _ foundation.Command = WorkspaceRestoreCommand{}

/*
Name returns workspace:restore.
*/
func (WorkspaceRestoreCommand) Name() string {
	return "workspace:restore"
}

/*
Description says what the command does.
*/
func (WorkspaceRestoreCommand) Description() string {
	return "Take a workspace out of the trash during its grace period"
}

/*
Flags declares the slug of the workspace.
*/
func (WorkspaceRestoreCommand) Flags(flags *console.Flags) {
	flags.Argument("slug", "the workspace to restore")
}

/*
Handle restores the workspace.
*/
func (c WorkspaceRestoreCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	restore := &workspaces.OperatorRestoreWorkspace{
		DB: c.App.DB, Events: c.App.Events, Clock: c.App.Clock, Mail: c.App.Mail, Links: c.App.Links,
	}

	w, err := restore.Handle(ctx, in.Argument("slug"))
	if err != nil {
		return err
	}

	out.Info("Workspace %s is restored and its owners are told.", w.Slug)

	return nil
}

/*
WorkspacePurgeCommand deletes a workspace in the trash for good.
*/
type WorkspacePurgeCommand struct {
	App *app.App
}

var _ foundation.Command = WorkspacePurgeCommand{}

/*
Name returns workspace:purge.
*/
func (WorkspacePurgeCommand) Name() string {
	return "workspace:purge"
}

/*
Description says what the command does.
*/
func (WorkspacePurgeCommand) Description() string {
	return "Delete a workspace in the trash for good, now"
}

/*
Flags declares the slug of the workspace and the switch that confirms
the purge.
*/
func (WorkspacePurgeCommand) Flags(flags *console.Flags) {
	flags.Argument("slug", "the workspace to purge; it must be in the trash")
	flags.Bool("force", "confirm that the workspace is deleted for good")
}

/*
Handle purges the workspace.
*/
func (c WorkspacePurgeCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	if !in.Bool("force") {
		return ErrForceRequired
	}

	disk, err := c.App.Storage.Default()
	if err != nil {
		return fmt.Errorf("workspace disk: %w", err)
	}

	purge := &workspaces.PurgeTrashedWorkspace{DB: c.App.DB, Clock: c.App.Clock, Purger: &jobs.PurgeWorkspace{
		DB: c.App.DB, Clock: c.App.Clock, Disk: disk, Retention: c.App.Retention, Log: c.App.Log,
	}}

	w, err := purge.Handle(ctx, in.Argument("slug"))
	if err != nil {
		return err
	}

	out.Info("Workspace %s is deleted for good.", w.Slug)

	return nil
}
