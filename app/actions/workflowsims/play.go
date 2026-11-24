/*
|--------------------------------------------------------------------------
| Playing A Workflow Simulation
|--------------------------------------------------------------------------
|
| One saved simulation is played through one copy of a workflow. The
| customer's first saved message is the message that triggers it, for
| every trigger but the opener, which fires before the customer writes.
| The workflow then walks as a dry run, and this file plays the rest:
|
|   - When the workflow asks, the simulated customer moves: a saved
|     message, an answer of its own, a tap on a button, or nothing,
|     which lets the wait run out.
|   - In an agent step, dimmi answers on the agent simulator, as the
|     agent the step names or the workspace's default, and the customer
|     answers it, until dimmi hands over, the customer is done (the
|     resolved path) or says nothing more (the quiet path).
|
| Whoever started the run watches it happen on its stream: each step as
| it is taken, each message, each of dimmi's turns, then the checks
| being judged and the verdict.
|
*/

package workflowsims

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
	engine "github.com/dimmihq/dimmi/app/workflows"
)

/*
Play limits: dimmi's turns in one agent step, the transfers one turn
follows, and the customer moves one run may make.
*/
const (
	maxAgentTurns   = 6
	maxTransfers    = 2
	maxCustomerMove = 24
)

/*
The reasons a run stops without a verdict.
*/
var (
	errNoCustomer = errors.New("The simulated customer needs dimmi's built-in Claude or GPT models, " +
		"which this workspace lacks.")
	errSilent    = errors.New("The simulated customer did not answer, so this run does not count. Run it again.")
	errTooLong   = errors.New("The customer made more moves than a simulation allows.")
	errNoOpening = errors.New("This simulation has no message to start from.")
)

/*
Result is how one run of a workflow simulation went: whether it passed,
when it ran and on which copy, the workflow's trace, the chat as the
customer saw it, dimmi's turns inside agent steps, the checks, and the
error of a run that could not be played to its end.
*/
type Result struct {
	Passed           bool                        `json:"passed"`
	RanAt            time.Time                   `json:"ran_at"`
	Version          string                      `json:"version"`
	Trace            *engine.DryTrace            `json:"trace,omitempty"`
	Transcript       []Line                      `json:"transcript"`
	Turns            []procedures.TestTurn       `json:"turns"`
	AssertionResults []procedure.AssertionResult `json:"assertion_results"`
	Error            string                      `json:"error,omitempty"`
}

/*
Line is one line of the chat: who wrote it (customer, workflow or
agent), the words, the step that wrote it, the buttons it offered, the
agent's name, and whether the customer tapped a button or said nothing.
*/
type Line struct {
	From    string   `json:"from"`
	Text    string   `json:"text"`
	StepID  string   `json:"step_id,omitempty"`
	Choices []string `json:"choices,omitempty"`
	Name    string   `json:"name,omitempty"`
	Tapped  bool     `json:"tapped,omitempty"`
	Silent  bool     `json:"silent,omitempty"`
}

/*
playing is one simulation to play: who plays it, the case, the copy of
the workflow, which copy that is, and the stream to watch it on.
*/
type playing struct {
	m       authz.Membership
	c       models.WorkflowSimulation
	copy    copied
	version string
	stream  string
}

/*
play plays one simulation through its copy of the workflow and judges
it.
*/
func (s *Simulations) play(ctx context.Context, p playing) Result {
	res := Result{
		RanAt: s.Clock.Now(), Version: p.version, Transcript: []Line{}, Turns: []procedures.TestTurn{},
		AssertionResults: []procedure.AssertionResult{},
	}
	ch := s.newChat(p)
	ch.tell(ctx, "started", nil)
	defer func() { ch.tell(ctx, "finished", map[string]any{"result": res}) }()

	spec, err := workflowspec.Parse(p.copy.spec)
	if err != nil {
		res.Error = "The workflow's definition cannot be read."
		return res
	}
	trigger := map[string]any{}
	if spec.Trigger != workflowspec.TriggerOpener {
		first, ok := ch.opening()
		if !ok {
			res.Error = errNoOpening.Error()
			return res
		}
		trigger["message.body"] = first
	}
	in := engine.DryRun{
		WorkspaceID: p.m.Workspace.ID, Spec: p.copy.spec, Conversation: p.c.SourceConversationID,
		Person: dryPerson(ch.script.persona), Trigger: trigger, Mocks: ch.script.mocks, Driver: ch,
	}
	trace, err := s.Engine.DryRun(ctx, in)
	res.Transcript, res.Turns, res.Trace = ch.lines, ch.turns, trace
	if err != nil {
		res.Error = runError(err)
		return res
	}

	ch.tell(ctx, "judging", nil)
	res.AssertionResults = s.judgeRun(ctx, p.m, ch.script.assertions, ch, trace)
	res.Passed = procedure.Passed(res.AssertionResults)
	res.Error = unjudged(res.AssertionResults)
	return res
}

/*
runError is what a run that could not be played to its end says.
*/
func runError(err error) string {
	for _, known := range []error{errNoCustomer, errSilent, errTooLong} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	if errors.Is(err, engine.ErrDryRun) {
		return "The simulation cannot start: its conversation or the workflow's definition cannot be read."
	}
	return "The simulation failed. Try again."
}

/*
dryPerson is the made-up customer the dry run reads facts from.
*/
func dryPerson(p pipeline.SimPersona) engine.DryPerson {
	attrs := map[string]any{}
	maps.Copy(attrs, p.Attributes)
	return engine.DryPerson{Name: p.Name, Email: p.Email, Channel: p.Channel, InboxID: p.InboxID, Attributes: attrs}
}

/*
script is a saved simulation as it is played.
*/
type script struct {
	persona    pipeline.SimPersona
	scenario   string
	messages   []procedures.TestMessage
	mocks      map[string]actionkit.Mock
	assertions []procedure.Assertion
}

/*
chat is a run's chat so far, and the driver the dry run asks: what was
said, what has been sent of the saved script, dimmi's turns and state,
and who watches.
*/
type chat struct {
	s         *Simulations
	p         playing
	script    script
	sent      []bool
	lines     []Line
	history   []pipeline.SimMessage
	turns     []procedures.TestTurn
	waiting   string
	state     *pipeline.SimState
	heldBy    *ids.UUID
	calls     []procedure.ObservedCall
	handedOff bool
	team      string
	priority  string
	wrote     []string
	moves     int
}

/*
newChat reads the simulation's script and opens its chat.
*/
func (s *Simulations) newChat(p playing) *chat {
	sc := script{mocks: map[string]actionkit.Mock{}, scenario: p.c.Scenario}
	_ = json.Unmarshal(p.c.Persona, &sc.persona)
	_ = json.Unmarshal(p.c.Messages, &sc.messages)
	_ = json.Unmarshal(p.c.ActionMocks, &sc.mocks)
	_ = json.Unmarshal(p.c.Assertions, &sc.assertions)
	return &chat{s: s, p: p, script: sc, sent: make([]bool, len(sc.messages)), lines: []Line{},
		turns: []procedures.TestTurn{}}
}

/*
opening sends the first saved message as the one that triggers the
workflow.
*/
func (c *chat) opening() (string, bool) {
	for i, m := range c.script.messages {
		if !m.Confirm && strings.TrimSpace(m.Text) != "" {
			c.sent[i] = true
			c.customerSaid(m.Text, Line{})
			return m.Text, true
		}
	}
	return "", false
}

/*
customerSaid keeps a message of the customer's.
*/
func (c *chat) customerSaid(text string, l Line) {
	l.From, l.Text = "customer", text
	c.lines = append(c.lines, l)
	c.history = append(c.history, pipeline.SimMessage{From: "customer", Text: text})
	c.waiting = text
}

/*
Said keeps what the workflow said.
*/
func (c *chat) Said(ctx context.Context, m engine.DryMessage) {
	text := m.Text
	if len(m.Choices) > 0 {
		text += "\n\n" + numbered(m.Choices)
	}
	c.lines = append(c.lines, Line{From: "workflow", Text: m.Text, StepID: m.StepID, Choices: m.Choices})
	c.history = append(c.history, pipeline.SimMessage{From: "agent", Text: text})
	c.tell(ctx, "said", map[string]any{"message": m})
}

/*
numbered lists choices as numbered lines.
*/
func numbered(choices []string) string {
	lines := make([]string, len(choices))
	for i, o := range choices {
		lines[i] = strconv.Itoa(i+1) + ". " + o
	}
	return strings.Join(lines, "\n")
}

/*
Stepped tells whoever watches of a step once it ran.
*/
func (c *chat) Stepped(ctx context.Context, s engine.DryStep) {
	c.tell(ctx, "step", map[string]any{"step": s})
}

/*
Words is what the customer wrote so far.
*/
func (c *chat) Words() []string {
	var out []string
	for _, l := range c.lines {
		if l.From == "customer" && !l.Silent {
			out = append(out, l.Text)
		}
	}
	return out
}

/*
Customer asks the simulated customer for its answer to the workflow.
*/
func (c *chat) Customer(ctx context.Context, ask engine.DryAsk) (engine.DryAnswer, error) {
	move, err := c.move(ctx, pipeline.CustomerTurn{
		Choices: ask.Choices, ChoicesOnly: ask.ChoicesOnly, CanWait: ask.CanWait,
	})
	if err != nil {
		return engine.DryAnswer{}, err
	}
	if move.Done || move.Waited || (move.Message == "" && move.Choice == 0) {
		c.lines = append(c.lines, Line{From: "customer", Silent: true, StepID: ask.StepID})
		return engine.DryAnswer{Silent: true}, nil
	}
	c.customerSaid(move.Message, Line{StepID: ask.StepID, Tapped: move.Choice > 0})
	return engine.DryAnswer{Text: move.Message, Choice: move.Choice}, nil
}

/*
move asks the simulated customer for its next move, the saved chat and
the run so far filled in, and marks a saved message it sends as sent.
*/
func (c *chat) move(ctx context.Context, turn pipeline.CustomerTurn) (pipeline.CustomerMove, error) {
	p := c.s.Procedures
	if p == nil || p.Customer == nil {
		return pipeline.CustomerMove{}, errNoCustomer
	}
	if c.moves++; c.moves > maxCustomerMove {
		return pipeline.CustomerMove{}, errTooLong
	}
	saved := make([]pipeline.SavedMessage, len(c.script.messages))
	for i, msg := range c.script.messages {
		saved[i] = pipeline.SavedMessage{Text: msg.Text, Confirm: msg.Confirm, Sent: c.sent[i]}
	}
	turn.WorkspaceID, turn.MemberID = c.p.m.Workspace.ID, c.p.m.Member.ID
	turn.Name, turn.Scenario, turn.Saved, turn.Run = c.script.persona.Name, c.script.scenario, saved, c.history
	move, err := p.Customer.PlayCustomer(ctx, turn)
	if errors.Is(err, pipeline.ErrCustomerUnavailable) {
		return move, errNoCustomer
	}
	if err != nil {
		return move, errSilent
	}
	if move.Saved >= 1 && move.Saved <= len(c.sent) {
		c.sent[move.Saved-1] = true
	}
	return move, nil
}

/*
Handle plays dimmi's turns in an agent step, the customer answering
each, and says how they ended.
*/
func (c *chat) Handle(ctx context.Context, h engine.DryHandle) (engine.DryHandled, error) {
	if c.s.Simulator == nil {
		return engine.DryHandled{Outcome: workflowspec.OutcomeHandedOver, Reason: "agent_disabled"}, nil
	}
	agent := h.AgentID
	confirm := false
	for range maxAgentTurns {
		if c.waiting == "" && !confirm {
			move, err := c.move(ctx, pipeline.CustomerTurn{CanWait: true, Confirming: c.pendingSummary()})
			if err != nil {
				return engine.DryHandled{}, err
			}
			switch {
			case move.Confirm:
				confirm = true
				c.lines = append(c.lines, Line{From: "customer", Text: "(pressed Confirm)"})
			case move.Done:
				return engine.DryHandled{Outcome: workflowspec.OutcomeResolved, Reason: "customer_done"}, nil
			case move.Waited || move.Message == "":
				c.lines = append(c.lines, Line{From: "customer", Silent: true, StepID: h.StepID})
				return engine.DryHandled{Outcome: workflowspec.OutcomeQuiet, Reason: "customer_quiet"}, nil
			default:
				c.customerSaid(move.Message, Line{})
			}
		}
		res, err := c.agentTurn(ctx, h.StepID, agent, confirm)
		if errors.Is(err, pipeline.ErrNoAgent) {
			return engine.DryHandled{Outcome: workflowspec.OutcomeHandedOver, Reason: "agent_disabled"}, nil
		}
		if err != nil {
			return engine.DryHandled{}, err
		}
		confirm = false
		if res.Handoff != nil {
			return engine.DryHandled{Outcome: workflowspec.OutcomeHandedOver, Reason: res.Handoff.Reason}, nil
		}
		if res.Outcome == "error" || res.Outcome == "handoff" {
			c.handedOff = true
			return engine.DryHandled{Outcome: workflowspec.OutcomeHandedOver, Reason: "agent_" + res.Outcome}, nil
		}
	}

	return engine.DryHandled{Outcome: workflowspec.OutcomeQuiet, Reason: "turn_limit"}, nil
}

/*
pendingSummary is the card dimmi waits on the customer to confirm, if
any.
*/
func (c *chat) pendingSummary() string {
	if c.state == nil || len(c.state.Pending) == 0 {
		return ""
	}
	return c.state.Pending[0].Summary
}

/*
agentTurn answers the customer's waiting message as dimmi, following a
transfer to the agent it hands the chat to, and keeps the turn.
*/
func (c *chat) agentTurn(ctx context.Context, stepID string, agent *ids.UUID, confirm bool,
) (*pipeline.SimulationResult, error) {
	m := c.p.m
	at := len(c.turns)
	msg := c.waiting
	var confirmed []string
	if confirm && c.state != nil {
		for _, p := range c.state.Pending {
			confirmed = append(confirmed, p.ActionID)
		}
	}
	c.tell(ctx, "turn_started", map[string]any{"turn": at, "message": msg, "confirm": confirm, "step_id": stepID})
	answering := agent
	if c.heldBy != nil {
		answering = c.heldBy
	}
	history := withoutLast(c.history, msg)
	in := pipeline.SimulationInput{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, AgentID: answering, History: history, Message: msg,
		Persona: c.script.persona, Mocks: c.script.mocks, State: c.state, Confirmed: confirmed,
		StreamID: c.turnStream(at), Now: c.p.c.SimulatedAt,
	}
	res, err := c.s.Simulator.Simulate(ctx, in)
	var moved *pipeline.SimTransfer
	for hops := 0; err == nil && res.Transfer != nil && hops < maxTransfers; hops++ {
		moved = res.Transfer
		to := moved.To
		in.AgentID = &to
		res, err = c.s.Simulator.Simulate(ctx, in)
	}
	if err != nil {
		return nil, err
	}
	if res.Transfer == nil {
		res.Transfer = moved
	}
	if res.Transfer != nil {
		to := res.Transfer.To
		c.heldBy = &to
	}
	c.keepTurn(ctx, stepID, msg, confirm, res)
	return res, nil
}

/*
keepTurn keeps one of dimmi's turns: the reply in the chat, the calls
and the handover in what was observed, and the turn itself.
*/
func (c *chat) keepTurn(ctx context.Context, stepID, msg string, confirm bool, res *pipeline.SimulationResult) {
	if res.Model != "" && !slices.Contains(c.wrote, res.Model) {
		c.wrote = append(c.wrote, res.Model)
	}
	if res.Reply != "" {
		c.lines = append(c.lines, Line{From: "agent", Text: res.Reply, StepID: stepID, Name: res.AgentName})
		c.history = append(c.history, pipeline.SimMessage{From: "agent", Text: res.Reply})
	}
	c.calls = append(c.calls, res.Calls...)
	if h := res.Handoff; h != nil {
		c.handedOff, c.team, c.priority = true, h.TeamID, h.Priority
	}
	c.state, c.waiting = &res.State, ""
	turn := procedures.TestTurn{
		Message: msg, Confirm: confirm, Reply: res.Reply, Outcome: res.Outcome, Calls: res.Calls,
		ProcedureTrace: res.ProcedureTrace, RunID: models.PublicID(models.PrefixAiRun, res.RunID), Transfer: res.Transfer,
	}
	c.turns = append(c.turns, turn)
	c.tell(ctx, "turn_finished", map[string]any{"turn": len(c.turns) - 1, "result": turn, "step_id": stepID})
}

/*
transcript is the chat as the judge reads it.
*/
func (c *chat) transcript() string {
	var b strings.Builder
	for _, l := range c.lines {
		switch {
		case l.Silent:
			b.WriteString("[The customer did not answer]\n")
		case l.From == "customer" && l.Tapped:
			b.WriteString("[The customer tapped the button: " + l.Text + "]\n")
		case l.From == "customer":
			b.WriteString("CUSTOMER: " + l.Text + "\n")
		case l.From == "workflow":
			b.WriteString("WORKFLOW: " + l.Text + "\n")
			if len(l.Choices) > 0 {
				b.WriteString("[Buttons: " + strings.Join(l.Choices, " | ") + "]\n")
			}
		default:
			b.WriteString(strings.ToUpper(cmp.Or(l.Name, "AGENT")) + ": " + l.Text + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

/*
observed is what the run did, as the agent's checks read it: the
workflow's actions as calls named by step id and kind, dimmi's own
calls, every reply and the handover.
*/
func (c *chat) observed(trace *engine.DryTrace) procedure.Observed {
	o := procedure.Observed{Branches: map[string]string{}, Calls: []procedure.ObservedCall{}, Replies: []string{}}
	if trace != nil {
		for _, a := range trace.Actions {
			o.Calls = append(o.Calls, procedure.ObservedCall{ActionID: a.StepID, Tool: a.Kind, Args: a.Detail,
				Succeeded: true})
		}
		if trace.End == engine.DryHandedOver {
			o.HandedOff = true
		}
		for _, s := range trace.Steps {
			if s.Kind == workflowspec.KindDimmi && s.Outcome == workflowspec.OutcomeHandedOver {
				o.HandedOff = true
			}
		}
	}
	o.Calls = append(o.Calls, c.calls...)
	for _, l := range c.lines {
		if l.From != "customer" && l.Text != "" {
			o.Replies = append(o.Replies, l.Text)
		}
	}
	if c.handedOff {
		o.HandedOff, o.Team, o.Priority = true, c.team, c.priority
	}
	return o
}

/*
tell sends one moment of the run to whoever watches.
*/
func (c *chat) tell(ctx context.Context, moment string, details map[string]any) {
	p := c.s.Procedures
	if c.p.stream == "" || p == nil || p.Live == nil {
		return
	}
	id := models.PublicID(models.PrefixWorkflowSim, c.p.c.ID)
	data := map[string]any{"stream_id": c.p.stream, "test_case_id": id, "simulation_id": id, "moment": moment}
	maps.Copy(data, details)
	p.Live.PublishRun(ctx, c.p.m.Member.ID, data)
}

/*
turnStream names the stream of one of dimmi's turns, which its own
steps are published on, or nothing when nobody watches.
*/
func (c *chat) turnStream(turn int) string {
	if c.p.stream == "" {
		return ""
	}
	return c.p.stream + ":" + models.PublicID(models.PrefixWorkflowSim, c.p.c.ID) + ":" + strconv.Itoa(turn)
}

/*
withoutLast is the history without the customer's last message with
the text, which a turn sends as its message.
*/
func withoutLast(history []pipeline.SimMessage, text string) []pipeline.SimMessage {
	if text == "" {
		return history
	}
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].From == "customer" && history[i].Text == text {
			return append(append([]pipeline.SimMessage{}, history[:i]...), history[i+1:]...)
		}
	}
	return history
}
