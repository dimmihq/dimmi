/*
|--------------------------------------------------------------------------
| AI Doctor Command
|--------------------------------------------------------------------------
|
| dimmi ai:doctor explains, in plain words, what stands between the
| install and an agent that answers: the managed upstreams, the
| prompt templates, the price table and the subscription connect.
| It exits with a failure when something blocks answers outright.
|
*/

package commands

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/ai"
)

/*
ErrAIBlocked fails the run when something blocks answers outright.
*/
var ErrAIBlocked = errors.New("ai doctor: the agent is blocked")

/*
AIDoctorCommand reports on the install's agent.
*/
type AIDoctorCommand struct {
	App *app.App
}

var _ foundation.Command = AIDoctorCommand{}

/*
Name returns the command's name.
*/
func (AIDoctorCommand) Name() string { return "ai:doctor" }

/*
Description returns the line shown in the command list.
*/
func (AIDoctorCommand) Description() string {
	return "Explain what stands between the install and an agent that answers"
}

/*
Flags takes none; every check runs.
*/
func (AIDoctorCommand) Flags(_ *console.Flags) {}

/*
Handle prints one line per check.
*/
func (c AIDoctorCommand) Handle(
	ctx context.Context, _ *foundation.Application, _ *console.Input, out *console.Output,
) error {
	doctor := &ai.AiDoctor{
		Config: c.App.Config.AI, Prompts: c.App.AI.Prompts, Prices: c.App.AI.Prices,
	}

	blocked := false
	for _, check := range doctor.Handle(ctx) {
		out.Line("[%s] %s", check.Status, check.Message)
		blocked = blocked || check.Status == ai.AICheckFail
	}

	if blocked {
		return ErrAIBlocked
	}
	return nil
}
