/*
|--------------------------------------------------------------------------
| Email Doctor Command
|--------------------------------------------------------------------------
|
| dimmi email:doctor <domain> explains, in plain words, what stands
| between the install and mail that arrives: the customer mailer, the
| inbound domain and the domain's own records. It exits with a failure
| when something blocks mail outright, so a deploy check can run it.
|
*/

package commands

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/email"
)

/*
ErrEmailBlocked fails the run when something blocks mail outright.
*/
var ErrEmailBlocked = errors.New("email doctor: mail is blocked")

/*
EmailDoctorCommand reports on the install's mail and one domain.
*/
type EmailDoctorCommand struct {
	App *app.App
}

var _ foundation.Command = EmailDoctorCommand{}

/*
Name returns the command's name.
*/
func (EmailDoctorCommand) Name() string {
	return "email:doctor"
}

/*
Description returns the line shown in the command list.
*/
func (EmailDoctorCommand) Description() string {
	return "Explain what stands between a domain and mail that arrives"
}

/*
Flags declares the domain to check.
*/
func (EmailDoctorCommand) Flags(flags *console.Flags) {
	flags.Argument("domain", "the sending domain to check, such as yourcompany.com")
}

/*
Handle prints one line per finding.
*/
func (c EmailDoctorCommand) Handle(
	ctx context.Context, fa *foundation.Application, in *console.Input, out *console.Output,
) error {
	doctor := &email.DiagnoseEmailDomain{
		Domains: c.App.EmailDomains, Lang: fa.Lang, Production: fa.Env == foundation.EnvProduction,
	}

	findings, err := doctor.Handle(ctx, in.Argument("domain"))
	if err != nil {
		return err
	}

	blocked := false

	for _, finding := range findings {
		out.Line("[%s] %s", finding.Level, finding.Message)
		blocked = blocked || finding.Level == email.FindingFail
	}

	if blocked {
		return ErrEmailBlocked
	}

	return nil
}
