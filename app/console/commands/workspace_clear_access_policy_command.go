/*
|--------------------------------------------------------------------------
| Workspace Clear Access Policy Command
|--------------------------------------------------------------------------
|
| dimmi workspace:clear-access-policy <slug> [--only ip_allowlist] is the
| way back in for a company its own rules shut out. It prints what it
| cleared, so whoever ran it has the same record the audit log keeps.
|
*/

package commands

import (
	"context"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/accesspolicy"
)

/*
WorkspaceClearAccessPolicyCommand clears a workspace's rules through the
application's action.
*/
type WorkspaceClearAccessPolicyCommand struct {
	App *app.App
}

var _ foundation.Command = WorkspaceClearAccessPolicyCommand{}

/*
Name returns the command's name.
*/
func (WorkspaceClearAccessPolicyCommand) Name() string {
	return "workspace:clear-access-policy"
}

/*
Description returns the line shown in the command list.
*/
func (WorkspaceClearAccessPolicyCommand) Description() string {
	return "Clear a workspace's sign-in rules, for a company its own rules shut out"
}

/*
Flags declares the workspace's slug and how much to clear.
*/
func (WorkspaceClearAccessPolicyCommand) Flags(flags *console.Flags) {
	flags.Argument("slug", "the workspace to clear")
	flags.String("only", "", "clear only this part of the rules: ip_allowlist")
}

/*
Handle clears the rules and prints what changed.
*/
func (c WorkspaceClearAccessPolicyCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	changes, err := c.App.Actions.ClearAccessPolicy.Handle(ctx, accesspolicy.ClearAccessPolicyInput{
		Slug: in.Argument("slug"), Only: in.String("only"),
	})
	if err != nil {
		return err
	}

	if len(changes) == 0 {
		out.Line("Nothing to clear.")

		return nil
	}

	cleared := make([]string, 0, len(changes))
	for key := range changes {
		cleared = append(cleared, key)
	}

	slices.Sort(cleared)
	out.Line("Cleared %s. Recorded in the audit log as the system.", strings.Join(cleared, ", "))

	return nil
}
