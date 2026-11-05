/*
|--------------------------------------------------------------------------
| Workspace Suspend Commands
|--------------------------------------------------------------------------
|
| dimmi workspace:suspend freezes a workspace, for abuse or an unpaid
| bill, and dimmi workspace:unsuspend thaws it. Members can still read a
| suspended workspace but change nothing in it. A reason is always
| given, so whoever looks at the workspace next knows why it is frozen.
|
*/

package commands

import (
	"context"
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
)

/*
ErrReasonRequired refuses a suspension without a reason.
*/
var ErrReasonRequired = errors.New("give a reason with --reason")

/*
WorkspaceSuspendCommand suspends a workspace.
*/
type WorkspaceSuspendCommand struct {
	App *app.App
}

var _ foundation.Command = WorkspaceSuspendCommand{}

/*
Name returns workspace:suspend.
*/
func (WorkspaceSuspendCommand) Name() string {
	return "workspace:suspend"
}

/*
Description says what the command does.
*/
func (WorkspaceSuspendCommand) Description() string {
	return "Stop every change to a workspace, keeping it readable"
}

/*
Flags declares the slug of the workspace and the reason.
*/
func (WorkspaceSuspendCommand) Flags(flags *console.Flags) {
	flags.Argument("slug", "the workspace to suspend")
	flags.String("reason", "", "why the workspace is suspended")
}

/*
Handle suspends the workspace.
*/
func (c WorkspaceSuspendCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	reason := strings.TrimSpace(in.String("reason"))
	if reason == "" {
		return ErrReasonRequired
	}

	suspend := &workspaces.SuspendWorkspace{DB: c.App.DB, Clock: c.App.Clock, Audit: c.App.Actions.RecordAuditLog}

	w, err := suspend.Handle(ctx, in.Argument("slug"), reason)
	if err != nil {
		return err
	}

	out.Info("Workspace %s is suspended: %s", w.Slug, reason)

	return nil
}

/*
WorkspaceUnsuspendCommand lifts a workspace's suspension.
*/
type WorkspaceUnsuspendCommand struct {
	App *app.App
}

var _ foundation.Command = WorkspaceUnsuspendCommand{}

/*
Name returns workspace:unsuspend.
*/
func (WorkspaceUnsuspendCommand) Name() string {
	return "workspace:unsuspend"
}

/*
Description says what the command does.
*/
func (WorkspaceUnsuspendCommand) Description() string {
	return "Let a suspended workspace be changed again"
}

/*
Flags declares the slug of the workspace.
*/
func (WorkspaceUnsuspendCommand) Flags(flags *console.Flags) {
	flags.Argument("slug", "the workspace to unsuspend")
}

/*
Handle unsuspends the workspace.
*/
func (c WorkspaceUnsuspendCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	unsuspend := &workspaces.UnsuspendWorkspace{DB: c.App.DB, Clock: c.App.Clock, Audit: c.App.Actions.RecordAuditLog}

	w, err := unsuspend.Handle(ctx, in.Argument("slug"))
	if err != nil {
		return err
	}

	out.Info("Workspace %s is active.", w.Slug)

	return nil
}
