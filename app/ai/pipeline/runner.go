/*
|--------------------------------------------------------------------------
| Pipeline Runner
|--------------------------------------------------------------------------
|
| One customer message in, one turn of the agent out. A turn happens in
| three moves so no database transaction ever waits on a model: a short
| transaction loads the conversation, checks the policy gate and opens
| the run row; the agent's tool loop and the draft's check run outside
| any transaction, each step written as it completes; a last short
| transaction locks the conversation, makes sure nothing newer landed
| and commits the reply and any handoff together.
|
| A turn never ends in silence when the customer is talking to the
| agent: a provider failure still hands the conversation to the team.
|
*/

package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	fevents "github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/support/aiprices"
	"github.com/dimmihq/dimmi/app/support/linkguard"
	"github.com/dimmihq/dimmi/app/support/pii"
	"github.com/dimmihq/dimmi/app/support/prompts"
	"github.com/dimmihq/dimmi/config"
)

/*
Runner runs the agent for one conversation. Timeouts bound each
provider call; StorePayloads keeps request and response text on the
run's steps. Publisher, when set, shows the customer the agent is
typing while it writes; Working, when set, hears the same on every other
channel, so a chat app can show it. Actions, when set, gives the agent
the workspace's own actions as tools.
*/
type Runner struct {
	DB            *database.DB
	Clock         clock.Clock
	LLM           *managers.LLMManager
	Knowledge     contracts.Knowledge
	Prompts       *prompts.Loader
	Prices        *aiprices.Table
	Events        *fevents.Dispatcher
	Publisher     appbroadcasting.Publisher
	Working       contracts.AgentWorking
	Actions       *actionkit.Kit
	Timeouts      config.AITimeoutsConfig
	StorePayloads bool

	redactor *pii.Redactor
	guard    *linkguard.Guard
}

/*
Args is what one run needs: the conversation and the customer message
that triggered it, or for a playground question the member asking.
*/
type Args struct {
	WorkspaceID      ids.UUID
	ConversationID   ids.UUID
	TriggerMessageID ids.UUID
	MemberID         *ids.UUID
	Kind             string
	QueuedAt         time.Time
}

/*
Result is how the run ended. RunID is zero when the run was skipped
before a row was worth writing: a duplicate, a debounced message, or
a trigger that is gone.
*/
type Result struct {
	Outcome       string
	SkipReason    string
	HandoffReason string
	AnswerID      *ids.UUID
	RunID         ids.UUID
}

/*
Run answers one trigger message with the agent's own settings.
*/
func (r *Runner) Run(ctx context.Context, args Args) (*Result, error) {
	return r.RunConversationTurn(ctx, args, nil)
}

/*
RunConversationTurn runs one agent turn for the trigger message. Nil
settings means the agent's own; a workflow step passes its overrides.
An error means the infrastructure failed and the job should retry; a
model failure is an outcome, never an error.
*/
func (r *Runner) RunConversationTurn(ctx context.Context, args Args, settings *TurnSettings) (*Result, error) {
	r.prepare()

	st, early, err := r.open(ctx, args, settings)
	if err != nil {
		return nil, fmt.Errorf("pipeline open: %w", err)
	}
	if early != nil {
		return early, nil
	}

	d := st.gate
	if d == nil {
		d = r.turn(ctx, st)
	}

	res, err := r.close(ctx, st, d)
	if err != nil {
		return nil, fmt.Errorf("pipeline commit: %w", err)
	}
	return res, nil
}

/*
prepare fills the defaults a zero-configured runner lacks.
*/
func (r *Runner) prepare() {
	if r.Clock == nil {
		r.Clock = clock.System()
	}
	if r.redactor == nil {
		r.redactor = pii.New()
	}
	if r.guard == nil {
		r.guard = linkguard.New()
	}
}

/*
turn is the agent's turn: the tool loop writes the reply, and the
review turns it into a decision.
*/
func (r *Runner) turn(ctx context.Context, st *run) *decision {
	llm, err := r.LLM.ForKey(ctx, *st.env.AnswerKey)
	if err != nil {
		return failed("provider_unavailable", err)
	}

	stopTyping := r.typing(ctx, st.env)
	defer stopTyping()

	st.loop = &loop{llm: llm, system: r.render(st.env, "answer.system.md", r.answerVars(st))}
	st.loop.messages = r.conversationMessages(st.env)
	r.offerActions(ctx, st)

	draft, err := r.agentLoop(ctx, st)
	if err != nil {
		return failed("provider_error", err)
	}
	return r.review(ctx, st, draft)
}

/*
timeout derives a context that ends after d on the runner's clock; a
zero d leaves the context alone.
*/
func (r *Runner) timeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	ctx, cancel := context.WithCancelCause(ctx)
	timer := r.Clock.AfterFunc(d, func() { cancel(context.DeadlineExceeded) })
	return ctx, func() {
		timer.Stop()
		cancel(context.Canceled)
	}
}
