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
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	fevents "github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/agents"
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
channel, so a chat app can show it. Drafts, when set, turns answers in
places where the team reviews the agent into drafts for a teammate to
send. Actions, when set, gives the agent the workspace's own actions as
tools. Delivery, when set, queues each reply on a channel that pushes.
Decider, when set, checks that a procedure the agent asks for fits the
request; DecisionProvider names it in a run's steps and DecisionPrice
is what one of its input tokens costs, in millionths of a dollar. ProcedureVectors, when set, ranks a long list of
procedures by meaning. It remembers each unpriced model it has logged,
so each is logged once.
*/
type Runner struct {
	DB               *database.DB
	Clock            clock.Clock
	LLM              *managers.LLMManager
	Knowledge        contracts.Knowledge
	Prompts          *prompts.Loader
	Prices           *aiprices.Table
	Events           *fevents.Dispatcher
	Publisher        appbroadcasting.Publisher
	Working          contracts.AgentWorking
	Drafts           contracts.AgentDrafts
	Delivery         *conversations.Delivery
	Actions          *actionkit.Kit
	Tickets          *tickets.Tickets
	Timeouts         config.AITimeoutsConfig
	StorePayloads    bool
	AnswerEffort     contracts.Effort
	Decider          contracts.Decider
	DecisionProvider string
	DecisionPrice    float64

	ProcedureVectors ProcedureVectors

	prepared sync.Once
	redactor *pii.Redactor
	guard    *linkguard.Guard

	unpricedSeen sync.Map
	stepWaits    sync.Map
}

/*
Args is what one run needs: the conversation and the customer message
that triggered it, or for a playground question the member asking.
Delayed marks a run the channel held back so a teammate could answer
first.
*/
type Args struct {
	WorkspaceID      ids.UUID
	ConversationID   ids.UUID
	TriggerMessageID ids.UUID
	MemberID         *ids.UUID
	Kind             string
	QueuedAt         time.Time
	Delayed          bool
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
	r.remember(ctx, st, res)
	if res.Outcome == outcomeTransferred {
		return r.RunConversationTurn(ctx, args, settings)
	}
	return res, nil
}

/*
prepare fills the defaults a zero-configured runner lacks, once. One
runner answers every conversation, so two turns starting together both
get here; filling the defaults unguarded let each write what the other
was reading.
*/
func (r *Runner) prepare() {
	r.prepared.Do(func() {
		if r.Clock == nil {
			r.Clock = clock.System()
		}
		if r.redactor == nil {
			r.redactor = pii.New()
		}
		if r.guard == nil {
			r.guard = linkguard.New()
		}
	})
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

	if st.sim == nil {
		stopTyping := r.typing(ctx, st.env)
		defer stopTyping()
	}

	st.loop = &loop{llm: llm, system: r.render(st.env, rootOf(st.env), r.answerVars(st))}
	r.shortlistProcedures(ctx, st)
	r.resumeProcedure(ctx, st)
	messages := r.conversationMessages(st.env)
	if st.sim != nil {
		if update := r.confirmPending(ctx, st); update != "" {
			messages = r.envelope(st.env, update)
		}
	}
	st.loop.messages = r.withMemory(st, messages)
	r.offerActions(ctx, st)
	r.openLayer(ctx, st)
	if st.proc != nil {
		r.ensureProcedureActions(ctx, st)
	}
	st.loop.newest = len(st.loop.messages) - 1
	r.searchAhead(ctx, st)

	draft, err := r.agentLoop(ctx, st)
	if err == nil && st.transfer == nil {
		draft, err = r.checked(ctx, st, draft)
	}
	r.routeProcedureHandoff(st)
	r.settleProcedure(st)
	r.saveLayer(ctx, st)
	if err != nil {
		return failed("provider_error", err)
	}
	return r.reviewOrTransfer(ctx, st, draft)
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

/*
checked passes a draft through the checks a reply must clear before it
is sent, each of which may send it back to be written again: its links,
the actions its procedure requires, and text never meant for the
customer.
*/
func (r *Runner) checked(ctx context.Context, st *run, draft string) (string, error) {
	draft, err := r.withoutUnknownLinks(ctx, st, draft)
	if err == nil {
		draft, err = r.verifyProcedureCompliance(ctx, st, draft)
	}
	if err == nil {
		draft, err = r.withoutNotes(ctx, st, draft)
	}
	return draft, err
}

/*
withoutUnknownLinks sends a draft that links somewhere the conversation
never mentioned back to the model once, with a note saying why, and
returns the rewritten reply. Nothing is cut from the draft: the model
writes it again. Should the rewrite still carry an unknown link, the
link guard in the review removes it as a last resort.
*/
func (r *Runner) withoutUnknownLinks(ctx context.Context, st *run, draft string) (string, error) {
	visible := earlierMarker().ReplaceAllString(earlierSources().ReplaceAllString(draft, ""), "")
	bad := r.guard.Untrusted(visible, r.allowedHosts(ctx, st))
	if len(bad) == 0 {
		return draft, nil
	}
	st.flags = append(st.flags, "link_regenerated")
	st.loop.messages = append(st.loop.messages,
		contracts.ChatMessage{Role: contracts.ChatRoleAssistant, Parts: textParts(draft)},
		userMessage("<system_note>That reply was not sent. It links to "+strings.Join(bad, ", ")+
			", which no result in this conversation contains and the customer did not give. Write the reply "+
			"again for the customer without that link. Use only links that appear in a result or that the "+
			"customer gave; when there is none, say in words where to go.</system_note>"))
	return r.agentLoop(ctx, st)
}

/*
ErrNoAgent marks a workspace without an agent to answer.
*/
var ErrNoAgent = agents.ErrNoAgent
