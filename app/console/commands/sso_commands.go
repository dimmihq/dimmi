/*
|--------------------------------------------------------------------------
| Single Sign-On Commands
|--------------------------------------------------------------------------
|
| What the operator of an install may do for a company's single sign-on
| from the server itself: vouch for a domain that public DNS cannot see,
| such as an internal one, and switch every connection of a company off
| when its identity provider is down and nobody can sign in. Both are
| recorded in the audit log as the system.
|
*/

package commands

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

/*
SSOVerifyDomainCommand marks a domain proven on the operator's word.
*/
type SSOVerifyDomainCommand struct {
	App *app.App
}

var _ foundation.Command = SSOVerifyDomainCommand{}

/*
Name returns the command's name.
*/
func (SSOVerifyDomainCommand) Name() string {
	return "sso:verify-domain"
}

/*
Description returns the line shown in the command list.
*/
func (SSOVerifyDomainCommand) Description() string {
	return "Verify a domain a workspace added, without a DNS record"
}

/*
Flags declares the domain and, when several workspaces added it, which.
*/
func (SSOVerifyDomainCommand) Flags(flags *console.Flags) {
	flags.Argument("domain", "the domain to verify")
	flags.String("workspace", "", "the slug of the workspace that added it, when several did")
}

/*
Handle verifies the domain and prints it.
*/
func (c SSOVerifyDomainCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	domain, err := c.App.Actions.SSOOperator.VerifyDomain(ctx, in.Argument("domain"), in.String("workspace"))
	if err != nil {
		return err
	}

	out.Line("Verified %s. Recorded in the audit log as the system.", domain.Domain)

	return nil
}

/*
SSODisableCommand switches off a company's connections.
*/
type SSODisableCommand struct {
	App *app.App
}

var _ foundation.Command = SSODisableCommand{}

/*
Name returns the command's name.
*/
func (SSODisableCommand) Name() string {
	return "sso:disable"
}

/*
Description returns the line shown in the command list.
*/
func (SSODisableCommand) Description() string {
	return "Switch off a workspace's single sign-on, for a company its identity provider shuts out"
}

/*
Flags declares the workspace's slug.
*/
func (SSODisableCommand) Flags(flags *console.Flags) {
	flags.Argument("slug", "the workspace whose connections to switch off")
}

/*
Handle switches the connections off and prints their names.
*/
func (c SSODisableCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	names, err := c.App.Actions.SSOOperator.Disable(ctx, in.Argument("slug"))
	if err != nil {
		return err
	}

	if len(names) == 0 {
		out.Line("No active connection to switch off.")

		return nil
	}

	out.Line("Switched off %s. Recorded in the audit log as the system.", strings.Join(names, ", "))

	return nil
}
