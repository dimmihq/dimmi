/*
|--------------------------------------------------------------------------
| Simulation Harness
|--------------------------------------------------------------------------
|
| Runs simulated customer conversations against the real customer-facing
| agent and measures what each one cost. Each conversation is started
| and continued through the same actions a customer's messages go
| through, and each customer message is answered by the same pipeline
| run the answer job makes, with real search over the workspace's help
| center and the workspace's real actions; only the network under the
| actions is answered in process, since their API is made up. Nothing
| is counted on the side: costs are read back from the runs the
| pipeline recorded.
|
| Every model call passes the spend guard, so the run's limit, the
| per-conversation limit and the turn limit hold before any call is
| made. A dry run swaps every provider's driver for an offline one, so
| no call can leave the process.
|
*/

package costsim

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The customer simulator modes.
*/
const (
	ModeScripted = "scripted"
	ModeModel    = "model"
)

/*
Options shape one simulation run. Limits are in micro dollars; zero
means none, except that a real run needs MaxSpend. Model, when set,
answers every call of a dry run instead of the offline model, which is
how tests script the agent with the fake driver. Actions, when set,
answers the workspace's actions' API on the client it is given, so no
action request leaves the process.
*/
type Options struct {
	Owner           authz.Membership
	Inbox           ids.UUID
	Channel         ids.UUID
	Company         string
	Scenarios       []Scenario
	Conversations   int
	Mode            string
	AgentModel      string
	ClassifierModel string
	CustomerModel   string
	MaxSpend        int64
	MaxPerConv      int64
	MaxTurns        int
	DryRun          bool
	Actions         func(*httpclient.Client)
	Model           contracts.LLM
	Label           string
	Log             func(format string, args ...any)
	Parallel        int
	AnswerKey       *ids.UUID
}

/*
Harness runs simulations on an application.
*/
type Harness struct {
	App      *app.App
	Budget   *Budget
	provider string
}

/*
ErrNoAgentModel means the workspace's agent has no connection to run on.
*/
var ErrNoAgentModel = errors.New("the agent has no AI connection; connect one in the workspace's AI settings")

/*
ConversationResult is one simulated conversation and what it cost.
*/
type ConversationResult struct {
	Index          int                 `json:"index"`
	Scenario       string              `json:"scenario"`
	Kind           string              `json:"kind"`
	ConversationID string              `json:"conversation_id"`
	Turns          int                 `json:"turns"`
	AIReplies      int                 `json:"ai_replies"`
	Runs           int                 `json:"runs"`
	ModelCalls     int                 `json:"model_calls"`
	Searches       int                 `json:"searches"`
	ToolCalls      int                 `json:"tool_calls"`
	NewTokens      int64               `json:"new_input_tokens"`
	CachedTokens   int64               `json:"cached_input_tokens"`
	OutputTokens   int64               `json:"output_tokens"`
	CostMicros     int64               `json:"cost_micros"`
	ConvCost       int64               `json:"conversation_ai_cost_micros"`
	ReplyCosts     []int64             `json:"reply_cost_micros"`
	Models         map[string]int      `json:"models"`
	Outcome        string              `json:"outcome"`
	Error          string              `json:"error,omitempty"`
	Layer          *LayerResult        `json:"tool_layer,omitempty"`
	Ledger         map[string]PartCost `json:"ledger,omitempty"`
	Reply          string              `json:"reply,omitempty"`
	RunErrors      int                 `json:"run_errors,omitempty"`
	BigPrompts     int                 `json:"calls_over_100k_tokens,omitempty"`
	RunIDs         []string            `json:"run_ids,omitempty"`
	Retrieved      []string            `json:"retrieved,omitempty"`
}

/*
Run plays the conversations and returns their results. It stops early,
cleanly, when a spend limit is reached or three conversations in a row
fail to start.
*/
func (h *Harness) Run(ctx context.Context, o Options) (*Report, error) {
	if len(o.Scenarios) == 0 {
		return nil, errors.New("no scenarios")
	}
	ctx, p, err := h.begin(ctx, &o)
	if err != nil {
		return nil, err
	}
	defer p.restore()

	customer := Customer(Scripted{})
	if o.Mode == ModeModel {
		llm, err := h.App.LLM.ForKey(ctx, h.answerKey(ctx, p.agent))
		if err != nil {
			return nil, fmt.Errorf("customer model: %w", err)
		}
		customer = Played{LLM: llm, Model: o.CustomerModel, Company: o.Company}
	}

	report := h.newReport(o, p)
	if o.Parallel > 1 {
		h.runParallel(ctx, turn{o: o, p: p, customer: customer}, report)
		return h.finish(report), nil
	}
	failures := 0
	for i := range o.Conversations {
		if !h.roomFor(p) {
			report.StoppedBy = "run spend limit"
			break
		}
		sc := o.Scenarios[i%len(o.Scenarios)]
		res := h.conversation(ctx, turn{o: o, p: p, customer: customer, index: i, sc: sc})
		report.Conversations = append(report.Conversations, res)
		o.Log("%3d %-28s %-16s turns %2d replies %2d calls %2d  $%.4f  %s %s", i+1, sc.Key, sc.Kind, res.Turns,
			res.AIReplies, res.ModelCalls, float64(res.CostMicros)/1e6, res.Outcome, res.Error)
		failures = failuresAfter(failures, res)
		if stop := stopReason(res, failures); stop != "" {
			report.StoppedBy = stop
			break
		}
	}
	return h.finish(report), nil
}

/*
runParallel plays the conversations several at a time under the same
run limit. The per-conversation limit cannot tell parallel conversations
apart, so this mode is for one-message questions, where the run limit
is the guard that matters.
*/
func (h *Harness) runParallel(ctx context.Context, base turn, report *Report) {
	o, p, customer := base.o, base.p, base.customer
	results := make([]ConversationResult, o.Conversations)
	started := make([]bool, o.Conversations)
	slots := make(chan struct{}, o.Parallel)
	var wg sync.WaitGroup
	for i := range o.Conversations {
		if !h.roomFor(p) {
			report.StoppedBy = "run spend limit"
			break
		}
		slots <- struct{}{}
		started[i] = true
		sc := o.Scenarios[i%len(o.Scenarios)]
		wg.Go(func() {
			defer func() { <-slots }()
			results[i] = h.conversation(ctx, turn{o: o, p: p, customer: customer, index: i, sc: sc})
			o.Log("%3d %-28s calls %2d  $%.4f  %s %s", i+1, sc.Key, results[i].ModelCalls,
				float64(results[i].CostMicros)/1e6, results[i].Outcome, results[i].Error)
		})
	}
	wg.Wait()
	for i, res := range results {
		if started[i] {
			report.Conversations = append(report.Conversations, res)
		}
	}
}

/*
begin checks the options, scopes the context to the workspace and
prepares the application.
*/
func (h *Harness) begin(ctx context.Context, o *Options) (context.Context, *prepared, error) {
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	if !o.DryRun && o.MaxSpend <= 0 {
		return ctx, nil, errors.New("a real run needs a spend limit")
	}
	ctx = tenancy.With(ctx, o.Owner.Workspace.ID.String())
	p, err := h.prepare(ctx, o)
	return ctx, p, err
}

/*
newReport starts the report of a run.
*/
func (h *Harness) newReport(o Options, p *prepared) *Report {
	return &Report{Started: h.App.Clock.Now().UTC(), Options: o.summary(), AgentModel: strv(p.agent.AnswerModel)}
}

/*
finish closes the report with what the guard counted and the figures.
*/
func (h *Harness) finish(r *Report) *Report {
	r.Finished = h.App.Clock.Now().UTC()
	r.SpendMicros, r.CustomerSpendMicros, r.Calls = h.Budget.Spent()
	r.Unpriced = h.Budget.Unpriced()
	r.summarize()
	return r
}

/*
roomFor reports whether the run's budget has room for another turn.
*/
func (h *Harness) roomFor(p *prepared) bool {
	run, _ := h.Budget.Remaining()
	return run < 0 || run >= h.turnFloor(p.agent)
}

/*
failuresAfter counts conversations in a row that failed before the
agent ran at all.
*/
func failuresAfter(failures int, res ConversationResult) int {
	if res.Error != "" && res.Runs == 0 {
		return failures + 1
	}
	return 0
}

/*
stopReason says why the run stops after this conversation, "" to go on.
*/
func stopReason(res ConversationResult, failures int) string {
	switch {
	case res.Outcome == "stopped_at_run_limit":
		return "run spend limit"
	case failures >= 3:
		return "three conversations in a row failed: " + res.Error
	}
	return ""
}

/*
turnFloor is the least a turn is worth starting with: the largest call
seen so far, or a small request on the agent's model before any.
*/
func (h *Harness) turnFloor(agent models.AiAgent) int64 {
	return max(h.Budget.Largest(),
		h.Budget.WorstCase(h.provider, contracts.ChatRequest{Model: strv(agent.AnswerModel), MaxOutputTokens: 200}))
}

func contains(list []string, want string) bool {
	return slices.Contains(list, want)
}

func strv(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
