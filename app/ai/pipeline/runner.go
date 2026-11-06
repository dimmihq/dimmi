/*
|--------------------------------------------------------------------------
| Pipeline Runner
|--------------------------------------------------------------------------
|
| One customer message in, one decision out. A run happens in three
| moves so no database transaction ever waits on a model: a short
| transaction loads the conversation, checks the gate and opens the run
| row; the model calls happen outside any transaction, each step
| written as it completes; a last short transaction locks the
| conversation, makes sure nothing newer landed and commits the reply.
|
| A run never ends in silence when the customer is talking to the
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
Runner runs the answer pipeline for one conversation. Timeouts bound
each provider call; StorePayloads keeps request and response text on
the run's steps. Publisher, when set, shows the customer the agent is
typing while it writes.
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
	Timeouts      config.AITimeoutsConfig
	StorePayloads bool

	redactor *pii.Redactor
	guard    *linkguard.Guard
}

/*
Args is what one run needs: the conversation and the customer message
that triggered it.
*/
type Args struct {
	WorkspaceID      ids.UUID
	ConversationID   ids.UUID
	TriggerMessageID ids.UUID
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
Run answers one trigger message and returns the outcome. An error
means the infrastructure failed and the job should retry; a model
failure is an outcome, never an error.
*/
func (r *Runner) Run(ctx context.Context, args Args) (*Result, error) {
	r.prepare()

	st, early, err := r.open(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("pipeline open: %w", err)
	}
	if early != nil {
		return early, nil
	}

	d := st.gate
	if d == nil {
		d = r.decide(ctx, st)
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
decide walks the stages that may call a model: the human fast path,
classify, route, retrieve, generate and post-process.
*/
func (r *Runner) decide(ctx context.Context, st *run) *decision {
	if d := r.fastPath(st); d != nil {
		return d
	}

	r.classify(ctx, st)
	if d := r.route(st); d != nil {
		return d
	}

	if !st.skipRetrieval {
		r.retrieve(ctx, st)
		if d := r.answerability(st); d != nil {
			return d
		}
	}

	return r.answer(ctx, st)
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
