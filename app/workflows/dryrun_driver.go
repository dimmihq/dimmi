/*
|--------------------------------------------------------------------------
| Dry Run Drivers
|--------------------------------------------------------------------------
|
| What a dry run asks of its caller: to keep what the workflow says, to
| play the customer's answer to what it asks, and to play the agent's
| turns in a step that lets dimmi handle the conversation.
|
*/

package workflows

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
DryDriver plays what a dry run does not decide itself.
*/
type DryDriver interface {
	/*
		Said keeps what the workflow said to the customer.
	*/
	Said(ctx context.Context, m DryMessage)
	/*
		Customer returns the customer's answer to what the workflow
		asked.
	*/
	Customer(ctx context.Context, ask DryAsk) (DryAnswer, error)
	/*
		Handle plays the agent's turns of a step that lets dimmi handle
		the conversation, and says how they ended.
	*/
	Handle(ctx context.Context, h DryHandle) (DryHandled, error)
	/*
		Stepped is told of each step once it ran, for whoever watches.
	*/
	Stepped(ctx context.Context, s DryStep)
	/*
		Words returns what the customer wrote so far, oldest first.
	*/
	Words() []string
}

/*
DryMessage is one message the workflow sends: the step, its words and
the buttons it offers.
*/
type DryMessage struct {
	StepID  string   `json:"step_id"`
	Text    string   `json:"text"`
	Choices []string `json:"choices,omitempty"`
}

/*
DryAsk is what the workflow waits on from the customer: the step and
its kind, the question, the buttons, whether only a button will do,
and whether the wait can run out. At a wait step, Waiting says how
long the workflow waits: the customer may write before then, which
interrupts it, or let the time pass.
*/
type DryAsk struct {
	StepID      string
	Kind        string
	Question    string
	Choices     []string
	ChoicesOnly bool
	CanWait     bool
	Waiting     string
}

/*
DryAnswer is the customer's move: what they wrote or the button they
tapped, by its number counting from one, or no answer at all.
*/
type DryAnswer struct {
	Text   string
	Choice int
	Silent bool
}

/*
DryHandle is a step that lets dimmi handle the conversation: the step
and the agent it names, if any.
*/
type DryHandle struct {
	StepID  string
	AgentID *ids.UUID
}

/*
DryHandled is how the agent's turns ended: resolved, handed over or
quiet, and why.
*/
type DryHandled struct {
	Outcome string
	Reason  string
}
