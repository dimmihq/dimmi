/*
|--------------------------------------------------------------------------
| Seeding The AI Agent
|--------------------------------------------------------------------------
|
| The agent is set up the way Stowline's support lead would set it up:
| its name, tone and guidance, the channels it answers, how many times
| it may answer before a person takes over, the team it hands to, and
| a monthly spend cap. It can call Stowline's own API through five
| actions, three reads and two writes that need the customer's or a
| teammate's agreement, and it follows three procedures for the
| requests the team handles the same way every time, each published
| three times as the team improved it. Its knowledge is
| the help center. The agent is switched on only when the install has
| a model to give it; otherwise it is saved switched off, ready.
|
| Past runs are written for every planned AI reply, with the steps and
| token counts a real run of that size leaves and the cost the price
| table gives them, so the usage pages read like a year of real use. A
| run of a conversation on a procedure's topic followed that procedure:
| it carries the trace of the steps it reached and the actions it called.
|
*/

package stowline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

func (r *run) agent(ctx context.Context) error {
	if err := r.configureAgent(ctx); err != nil {
		return err
	}
	published, err := r.connectActions(ctx)
	if err != nil {
		return err
	}
	r.actions = published
	return r.publishProcedures(ctx, published)
}

/*
configureAgent saves the agent's settings and switches it on when the
install gives it a model.
*/
func (r *run) configureAgent(ctx context.Context) error {
	a := r.s.App
	view, err := a.Actions.AI.Agents.Default(ctx, r.owner)
	if err != nil {
		return fmt.Errorf("provision agent: %w", err)
	}
	spendCap := int64(500_000_000)
	if _, err := a.Actions.AI.Agents.UpdateSettings(ctx, r.owner, ai.SettingsPatch{
		MonthlyCapMicros: ai.Optional[int64]{Set: true, Value: &spendCap},
	}); err != nil {
		return fmt.Errorf("set spend cap: %w", err)
	}
	mode := enums.AIHandoffModeFull
	patch := ai.AgentPatch{
		Name: new("Sky"), Tone: new("friendly"), Guidance: new(AgentGuidance), Schedule: new("always"),
		AssignedBehavior: new("skip_when_human_active"), UnknownBehavior: new("offer_handoff"),
		DefaultLanguage: new("en"), IncludeContact: new("basic"), RequireCitations: new(true),
		PiiRedaction: new(true), Streaming: new(true), EnabledChannels: &[]string{"widget", "email"},
		Languages: &[]string{"en", "de", "fr", "es", "pt", "nl"}, LinkAllowlist: &[]string{Domain},
		MaxAiReplies: new(int32(12)), ProductName: ai.Optional[string]{Set: true, Value: new(Name)},
		HandoffTeamID: ai.Optional[ids.UUID]{Set: true, Value: new(r.teams["Support"])}, HandoffMode: &mode,
	}
	if _, err := a.Actions.AI.Agents.Update(ctx, ai.UpdateAgentInput{
		Membership: r.owner, AgentID: view.Agent.ID, Patch: patch,
	}); err != nil {
		return fmt.Errorf("configure agent: %w", err)
	}
	if !view.Ready {
		r.s.Log("  the install offers no model, so the agent is saved switched off")
		return nil
	}
	if _, err := a.Actions.AI.Agents.Publish(ctx, ai.PublishInput{
		Membership: r.owner, AgentID: view.Agent.ID, ChangeNote: "Set up by the demo",
	}); err != nil {
		r.s.Log("  the agent is saved switched off: %v", err)
	}
	return nil
}

/*
connectActions connects Stowline's API and publishes the agent's
actions, each with the answer of its last test, returning their public
ids by name.
*/
func (r *run) connectActions(ctx context.Context) (map[string]string, error) {
	a := r.s.App
	conn, err := a.Actions.AI.ActionConnections.Create(ctx, agentactions.ConnectionInput{
		Membership: r.owner, Name: new("Stowline API"), Kind: "http", BaseURL: new(APIBaseURL),
		AuthType: new("bearer"), Token: new("sk_test_seeded_not_a_real_key"), TimeoutMS: new(int32(8000)),
	})
	if err != nil {
		return nil, fmt.Errorf("create action connection: %w", err)
	}
	published := map[string]string{}
	for _, def := range AgentActions(conn.ID) {
		def.Membership = r.owner
		made, err := a.Actions.AI.ActionDefinitions.Create(ctx, def)
		if err != nil {
			return nil, fmt.Errorf("create action %s: %w", *def.Name, err)
		}
		if _, err := a.Actions.AI.ActionDefinitions.Publish(ctx, r.owner, made.Action.ID, "First version"); err != nil {
			return nil, fmt.Errorf("publish action %s: %w", *def.Name, err)
		}
		published[*def.Name] = models.PublicID(models.PrefixAiAction, made.Action.ID)
		if sample, ok := TestResponses()[*def.Name]; ok {
			body, _ := json.Marshal(sample)
			if err := models.New(a.DB).SetActionLastTestResponse(ctx, models.SetActionLastTestResponseParams{
				Response: body, WorkspaceID: r.ws.ID, ID: made.Action.ID,
			}); err != nil {
				return nil, fmt.Errorf("keep test response of %s: %w", *def.Name, err)
			}
		}
	}
	return published, nil
}

/*
workflows adds the team's automations: one live that raises the
priority of conversations the agent hands over, so it never stands
between a customer and the agent, and one draft the team is still
working on.
*/
func (r *run) workflows(ctx context.Context) error {
	a := r.s.App
	if a.Actions.Workflows == nil {
		return nil
	}
	step := func(id, kind string, settings map[string]any) map[string]any {
		return map[string]any{"id": id, "kind": kind, "settings": settings}
	}
	specs := []struct {
		name, trigger string
		spec          map[string]any
		live          bool
	}{
		{"Raise priority when the agent hands over", "dimmi.handed_over", map[string]any{
			"schema": 1, "trigger": "dimmi.handed_over",
			"steps": []any{step("s_prio0001", "priority", map[string]any{"value": "high"})},
		}, true},
		{"Follow up on low ratings", "conversation.rated", map[string]any{
			"schema": 1, "trigger": "conversation.rated",
			"steps": []any{step("s_prio0002", "priority", map[string]any{"value": "high"})},
		}, false},
	}
	for _, s := range specs {
		spec, _ := json.Marshal(s.spec)
		made, err := a.Actions.Workflows.Create(ctx, workflows.CreateInput{
			Membership: r.owner, Name: s.name, Trigger: s.trigger, Spec: spec,
		})
		if err != nil {
			r.s.Log("  skipped workflow %q: %v", s.name, err)
			continue
		}
		if !s.live {
			continue
		}
		if _, err := a.Actions.Workflows.Publish(ctx, workflows.PublishInput{
			Membership: r.owner, ID: made.ID, AcknowledgeWarnings: true,
		}); err != nil {
			r.s.Log("  left workflow %q as a draft: %v", s.name, err)
		}
	}
	return nil
}

/*
SeededModel is the model the past runs are recorded on.
*/
const SeededModel = "gpt-6-luna"

/*
aiRuns writes a finished run, with its steps, for every planned AI
reply, and adds what each conversation's answers cost to it, as the
pipeline does.
*/
func (r *run) aiRuns(ctx context.Context) error {
	a := r.s.App
	h := &history{run: r, rnd: NewRandom(r.s.Seed ^ 0xa1), answered: map[ids.UUID]int64{}}
	if a.AI != nil {
		h.prices = a.AI.Prices
	}
	var err error
	if h.actions, err = r.publishedActions(ctx); err != nil {
		return err
	}
	for _, c := range r.plan.Conversations {
		if c.AI != "" {
			h.conversation(c)
		}
	}
	return h.write(ctx)
}

/*
history is the past runs being built: their rows, their steps' rows and
what each conversation's answers cost.
*/
type history struct {
	run      *run
	agent    ids.UUID
	held     [][]any
	prices   *aiprices.Table
	rnd      *Random
	runs     [][]any
	steps    [][]any
	calls    [][]any
	actions  map[string]publishedAction
	answered map[ids.UUID]int64
}

/*
call is one step of a past run: a model call or a tool call, and its
tokens.
*/
type call struct {
	kind, model           string
	input, cached, output int64
}

/*
conversation adds a run for every agent message of the conversation,
answering the customer message before it.
*/
func (h *history) conversation(c Conversation) {
	var trigger ids.UUID
	follow := h.follow(c)
	pending := h.route(c)
	for k, m := range c.Messages {
		id := stableID(m.At, fmt.Sprintf("%s/%d", c.ID, k))
		switch m.Author {
		case "contact":
			trigger = id
		case "agent":
			if pending {
				h.transfer(c, k, trigger)
				pending = false
			}
			last := k == len(c.Messages)-1 || c.Messages[k+1].Author != "contact" || k > 1
			h.reply(c, k, reply{trigger: trigger, answer: id, handoff: c.AI == "handed_off" && last, follow: follow})
		}
	}
}

/*
reply is one agent message: the customer message it answered, its own
id, whether it handed the conversation over, and the procedure the
conversation follows, if any.
*/
type reply struct {
	trigger, answer ids.UUID
	handoff         bool
	follow          *following
}

/*
reply adds one run, searching then answering, and handing over when it
did.
*/
func (h *history) reply(c Conversation, k int, rp reply) {
	m := c.Messages[k]
	run := stableID(m.At.Add(-8*time.Second), fmt.Sprintf("%s/run/%d", c.ID, k))
	started := m.At.Add(-time.Duration(3+h.rnd.IntN(6)) * time.Second)
	grown := int64(k * 260)
	calls := []call{
		{"generate", SeededModel, 2400 + grown + h.rnd.Int64N(600), 0, 40 + h.rnd.Int64N(40)},
		{"tool_call", "search_knowledge", 0, 0, 0},
		{"generate", SeededModel, 4300 + grown + h.rnd.Int64N(1800), 1900 + grown, 110 + h.rnd.Int64N(170)},
	}
	outcome, reason := "answered", any(nil)
	if rp.handoff {
		calls = append(calls, call{"tool_call", "handoff", 0, 0, 0})
		outcome, reason = "handoff", c.HandoffReason
	}
	var total, fresh, cached, output int64
	for seq, cl := range calls {
		cost := h.price(cl)
		total, fresh, cached, output = total+cost, fresh+cl.input-cl.cached, cached+cl.cached, output+cl.output
		at := started.Add(time.Duration(seq) * 1500 * time.Millisecond)
		var provider any
		if cl.kind == "generate" {
			provider = "dimmi"
		}
		h.steps = append(h.steps, []any{
			stableID(at, fmt.Sprintf("%s/%d", run, seq)), h.run.ws.ID, run, seq + 1, cl.kind, provider, cl.model,
			"ok", cl.input - cl.cached, cl.cached, cl.output, cost, 900 + h.rnd.IntN(2400), at, at.Add(time.Second), at,
		})
	}
	if !rp.handoff {
		h.answered[c.ID] += total
	}
	var feedback any
	if !rp.handoff && h.rnd.IntN(5) == 0 {
		feedback = "helped"
	}
	h.runs = append(h.runs, []any{
		run, h.run.ws.ID, h.agent, "conversation", c.ID, rp.trigger, "contact", h.run.plan.Contacts[c.Contact].ID,
		c.Channel, "completed", outcome, reason, c.Language, c.Subject,
		jsonOf([]map[string]any{{"title": c.Article, "score": 0.62 + h.rnd.Float64()*0.3}}), rp.answer,
		fresh, cached, output, total, 2400 + h.rnd.IntN(5000), 600 + h.rnd.IntN(900), started, m.At, started, m.At,
		feedback, rp.follow.flags(), h.trace(c, run, started, rp),
	})
}

/*
price is what a model call costs on the price table, nothing for a
tool call.
*/
func (h *history) price(cl call) int64 {
	if cl.kind != "generate" || h.prices == nil {
		return 0
	}
	p := h.prices.Cost("dimmi", cl.model, aiprices.TokenCounts{
		Input: cl.input - cl.cached, CachedInput: cl.cached, Output: cl.output,
	})
	if p == nil {
		return 0
	}
	return *p
}

/*
write inserts the runs and their steps, adds each conversation's
answers' cost to it and sums the months' spend.
*/
func (h *history) write(ctx context.Context) error {
	return h.run.s.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		for _, set := range []rowSet{
			{"ai_runs", runColumns(), h.runs},
			{"ai_run_steps", stepColumns(), h.steps},
			{"ai_action_calls", callColumns(), h.calls},
		} {
			if err := insertMany(ctx, tx, set); err != nil {
				return err
			}
		}
		if err := h.costs(ctx, tx); err != nil {
			return err
		}
		if err := h.holders(ctx, tx); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO ai_spend_periods (workspace_id, period_start, cost_micros, input_tokens,
				cached_input_tokens, output_tokens, runs_count, answered_count, handoff_count, updated_at)
			SELECT workspace_id, date_trunc('month', created_at)::date, sum(cost_micros), sum(input_tokens),
				sum(cached_input_tokens), sum(output_tokens), count(*), count(*) FILTER (WHERE outcome = 'answered'),
				count(*) FILTER (WHERE outcome = 'handoff'), now()
			FROM ai_runs WHERE workspace_id = $1 GROUP BY 1, 2
			ON CONFLICT DO NOTHING`, h.run.ws.ID)
		if err != nil {
			return fmt.Errorf("sum spend periods: %w", err)
		}
		return nil
	})
}

/*
costs adds each conversation's answers' cost to it, through a staged
table so one statement updates them all.
*/
func (h *history) costs(ctx context.Context, tx *database.Tx) error {
	rows := make([][]any, 0, len(h.answered))
	for id, micros := range h.answered {
		rows = append(rows, []any{id, micros})
	}
	_, err := tx.Exec(ctx, `CREATE TEMP TABLE seed_costs (id uuid PRIMARY KEY, micros bigint) ON COMMIT DROP`)
	if err != nil {
		return fmt.Errorf("stage costs: %w", err)
	}
	if err := insertMany(ctx, tx, rowSet{"seed_costs", []string{"id", "micros"}, rows}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `ANALYZE seed_costs`); err != nil {
		return fmt.Errorf("analyze costs: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE conversations c SET ai_cost_micros = s.micros FROM seed_costs s
		WHERE c.workspace_id = $1 AND c.id = s.id`, h.run.ws.ID); err != nil {
		return fmt.Errorf("add conversation costs: %w", err)
	}
	return nil
}

/*
runColumns are the columns a past run is written with.
*/
func runColumns() []string {
	return []string{
		"id", "workspace_id", "agent_id", "kind", "conversation_id", "trigger_message_id", "actor_type", "actor_id",
		"channel_type", "status", "outcome", "handoff_reason", "language", "query_original", "retrieval",
		"answer_message_id", "input_tokens", "cached_input_tokens", "output_tokens", "cost_micros", "latency_ms",
		"first_token_ms", "started_at", "completed_at", "created_at", "updated_at", "feedback", "flags",
		"procedure_trace",
	}
}

/*
stepColumns are the columns a past run's step is written with.
*/
func stepColumns() []string {
	return []string{
		"id", "workspace_id", "run_id", "seq", "kind", "provider", "model", "status", "input_tokens",
		"cached_input_tokens", "output_tokens", "cost_micros", "latency_ms", "started_at", "completed_at", "created_at",
	}
}
