/*
|--------------------------------------------------------------------------
| Workspace Repair Command
|--------------------------------------------------------------------------
|
| dimmi workspace:repair [slug] puts one workspace, or every workspace
| not in the trash, back in order: each workspace's defaults are created
| again where missing and its organization is brought back and renamed
| after it when needed. It is safe to run at any time and as often as
| needed; a second run changes nothing.
|
*/

package commands

import (
	"context"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

/*
WorkspaceRepairCommand repairs workspaces through the application's
action.
*/
type WorkspaceRepairCommand struct {
	App *app.App
}

var _ foundation.Command = WorkspaceRepairCommand{}

/*
Name returns the command's name.
*/
func (WorkspaceRepairCommand) Name() string {
	return "workspace:repair"
}

/*
Description returns the line shown in the command list.
*/
func (WorkspaceRepairCommand) Description() string {
	return "Recreate missing workspace defaults and restore their organizations"
}

/*
Flags declares the optional slug of the one workspace to repair.
*/
func (WorkspaceRepairCommand) Flags(flags *console.Flags) {
	flags.OptionalArgument("slug", "the workspace to repair; every workspace when left out")
}

/*
Handle repairs the workspaces and lists each one it went over.
*/
func (c WorkspaceRepairCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	repaired, err := c.App.Actions.RepairWorkspaces.Handle(ctx, in.Argument("slug"))

	for _, r := range repaired {
		organization := "in order"
		if r.Organization {
			organization = "restored"
		}

		out.Line("%s: defaults checked, organization %s", r.Workspace.Slug, organization)
	}

	if err != nil {
		return err
	}

	out.Info("Repaired %d workspace(s).", len(repaired))

	return nil
}
