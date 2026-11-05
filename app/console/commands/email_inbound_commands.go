/*
|--------------------------------------------------------------------------
| Inbound Mail Commands
|--------------------------------------------------------------------------
|
| What an operator runs while setting up a receiving provider or
| recovering from an outage: the health checks that name what to fix, the
| Worker Cloudflare needs to hand mail over, and a backfill feeding through
| the mail Resend held on to while the install was down. A backfilled mail
| the install already has is recognized and kept once.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/integrations/email/cloudflareworker"
)

/*
ErrDoctorFailed means a health check found something that stops mail.
*/
var ErrDoctorFailed = errors.New("inbound mail has problems")

/*
EmailInboundDoctorCommand prints how mail reaches the install.
*/
type EmailInboundDoctorCommand struct {
	App *app.App
}

/*
Name returns email:inbound-doctor.
*/
func (c EmailInboundDoctorCommand) Name() string {
	return "email:inbound-doctor"
}

/*
Description says what the command does.
*/
func (c EmailInboundDoctorCommand) Description() string {
	return "Check how mail reaches this install and name what to fix"
}

/*
Flags declares nothing.
*/
func (c EmailInboundDoctorCommand) Flags(*console.Flags) {}

/*
Handle prints each check and fails when one failed.
*/
func (c EmailInboundDoctorCommand) Handle(
	ctx context.Context, _ *foundation.Application, _ *console.Input, out *console.Output,
) error {
	failed := false

	for _, check := range c.App.Actions.InboundDoctor.Handle(ctx) {
		out.Line("%-5s %s: %s", check.Status, check.ID, check.Message)
		failed = failed || check.Status == email.CheckFail
	}

	if failed {
		return ErrDoctorFailed
	}

	return nil
}

/*
EmailCloudflareWorkerCommand prints the Worker and its configuration.
*/
type EmailCloudflareWorkerCommand struct {
	App *app.App
}

/*
Name returns email:cloudflare-worker.
*/
func (c EmailCloudflareWorkerCommand) Name() string {
	return "email:cloudflare-worker"
}

/*
Description says what the command does.
*/
func (c EmailCloudflareWorkerCommand) Description() string {
	return "Print the Cloudflare Email Routing Worker that hands mail to this install"
}

/*
Flags declares nothing.
*/
func (c EmailCloudflareWorkerCommand) Flags(*console.Flags) {}

/*
Handle prints both files with this install's address filled in.
*/
func (c EmailCloudflareWorkerCommand) Handle(
	_ context.Context, a *foundation.Application, _ *console.Input, out *console.Output,
) error {
	files := cloudflareworker.Render(a.Config.App.URL)

	out.Line("// wrangler.toml\n%s", files.Config)
	out.Line("// worker.js\n%s", files.Script)

	return nil
}

/*
EmailResendBackfillCommand feeds through the mail Resend received.
*/
type EmailResendBackfillCommand struct {
	App *app.App
}

/*
Name returns email:resend-backfill.
*/
func (c EmailResendBackfillCommand) Name() string {
	return "email:resend-backfill"
}

/*
Description says what the command does.
*/
func (c EmailResendBackfillCommand) Description() string {
	return "Feed through the mail Resend received since a date"
}

/*
Flags declares the date to start from.
*/
func (c EmailResendBackfillCommand) Flags(flags *console.Flags) {
	flags.String("since", "", "the date to start from, as 2026-11-01")
}

/*
Handle walks Resend's mail and says how many it handed over.
*/
func (c EmailResendBackfillCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	since, err := time.Parse(time.DateOnly, strings.TrimSpace(in.String("since")))
	if err != nil {
		return fmt.Errorf("--since must be a date such as 2026-11-01: %w", err)
	}

	count, err := c.App.Actions.BackfillResend.Handle(ctx, since)
	if err != nil {
		return err
	}

	out.Info("Fed through %d emails.", count)

	return nil
}
