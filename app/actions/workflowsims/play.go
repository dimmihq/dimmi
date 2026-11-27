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
|     which lets the wait run out. At a wait step it may write before
|     the time is up, which interrupts the wait, or let the time pass.
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
	"context"
	"encoding/json"
	"errors"
	"maps"
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
	errNoCustomer = errors.New("workflowsims: no simulated customer")
	errSilent     = errors.New("workflowsims: the simulated customer did not answer")
	errTooLong    = errors.New("workflowsims: too many customer moves")
)

/*
stopped is what a run that stopped without a verdict says, by why.
*/
func stopped() map[error]string {
	return map[error]string{
		errNoCustomer: "The simulated customer needs dimmi's built-in Claude or GPT models, " +
			"which this workspace lacks.",
		errSilent:  "The simulated customer did not answer, so this run does not count. Run it again.",
		errTooLong: "The customer made more moves than a simulation allows.",
	}
}

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
	watch   Watch
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
		first, ok := ch.opening(ctx)
		if !ok {
			res.Error = "This simulation has no message to start from."
			return res
		}
		trigger["message.body"] = first
	}
	in := engine.DryRun{
		WorkspaceID: p.m.Workspace.ID, WorkflowID: p.c.WorkflowID, Spec: p.copy.spec,
		Conversation: p.c.SourceConversationID, Person: dryPerson(ch.script.persona), Trigger: trigger,
		Mocks: ch.script.mocks, Driver: ch,
	}
	trace, err := s.Engine.DryRun(ctx, in)
	res.Transcript, res.Turns, res.Trace = ch.lines, ch.turns, trace
	if err != nil {
		res.Error = runError(err)
		return res
	}

	ch.tell(ctx, "judging", nil)
	res.AssertionResults = s.judgeRun(ctx, ch, trace)
	res.Passed = procedure.Passed(res.AssertionResults)
	res.Error = unjudged(res.AssertionResults)
	return res
}

/*
runError is what a run that could not be played to its end says.
*/
func runError(err error) string {
	for known, said := range stopped() {
		if errors.Is(err, known) {
			return said
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
	id        ids.UUID
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
	return &chat{
		id: ids.New(),
		s:  s, p: p, script: sc, sent: make([]bool, len(sc.messages)), lines: []Line{},
		turns: []procedures.TestTurn{},
	}
}

/*
opening sends the first saved message as the one that triggers the
workflow.
*/
func (c *chat) opening(ctx context.Context) (string, bool) {
	for i, m := range c.script.messages {
		if !m.Confirm && strings.TrimSpace(m.Text) != "" {
			c.sent[i] = true
			c.customerSaid(ctx, m.Text, Line{})
			return m.Text, true
		}
	}
	return "", false
}

/*
customerSaid keeps a message of the customer's.
*/
func (c *chat) customerSaid(ctx context.Context, text string, l Line) {
	l.From, l.Text = "customer", text
	c.add(ctx, l)
	c.history = append(c.history, pipeline.SimMessage{From: "customer", Text: text})
	c.waiting = text
}

/*
add keeps one line of the chat and tells whoever watches.
*/
func (c *chat) add(ctx context.Context, l Line) {
	c.lines = append(c.lines, l)
	c.tell(ctx, "line", map[string]any{"line": l})
}

/*
Said keeps what the workflow said.
*/
func (c *chat) Said(ctx context.Context, m engine.DryMessage) {
	text := m.Text
	if len(m.Choices) > 0 {
		text += "\n\n" + numbered(m.Choices)
	}
	c.add(ctx, Line{From: "workflow", Text: m.Text, StepID: m.StepID, Choices: m.Choices})
	c.history = append(c.history, pipeline.SimMessage{From: "agent", Text: text})
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
		Choices: ask.Choices, ChoicesOnly: ask.ChoicesOnly, CanWait: ask.CanWait, Waiting: ask.Waiting,
	})
	if err != nil {
		return engine.DryAnswer{}, err
	}
	if move.Done || move.Waited || (move.Message == "" && move.Choice == 0) {
		c.add(ctx, Line{From: "customer", Silent: true, StepID: ask.StepID})
		return engine.DryAnswer{Silent: true}, nil
	}
	c.customerSaid(ctx, move.Message, Line{StepID: ask.StepID, Tapped: move.Choice > 0})
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
	c.moves++
	if c.moves > maxCustomerMove {
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
tell sends one moment of the run to whoever watches.
*/
func (c *chat) tell(ctx context.Context, moment string, details map[string]any) {
	if c.p.watch != nil {
		c.p.watch(ctx, moment, details)
	}
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
