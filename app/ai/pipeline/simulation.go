/*
|--------------------------------------------------------------------------
| Procedure Simulator
|--------------------------------------------------------------------------
|
| A teammate tests a procedure as a made-up customer before publishing
| it. One simulated turn goes through the agent's whole turn as a real
| one would: the procedure is offered and checked for fit, its actions
| pinned, branches decided, prerequisites gated and required actions
| enforced. Only the edges differ: the procedure under test is the
| working copy, the customer is the persona given, and no action leaves
| dimmi; each answers with its mock, or with what its last live test
| returned. Nothing is written to any conversation. The run is recorded
| as a simulation and its spend counts toward the workspace's cap.
|
| The simulator keeps the procedure's state itself and hands it back
| each turn, so a conversation of many turns needs nothing stored. A
| write waiting on the customer or a teammate is handed back too; the
| tester confirms or approves it, and the next turn runs it and tells
| the agent how it went.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
simulationKind is the run kind of a simulated turn.
*/
const simulationKind = "simulation"

/*
SimulationInput is one simulated turn: the workspace and teammate, the
procedure under test with the definition to test, the conversation so
far, the customer's new message, the persona, the mocks by action
public id, the state the last turn handed back, and the waiting writes
the tester confirmed or approved since. With AgentID the turn answers
as that agent, in the definition given when there is one, and no
procedure is under test; without it, as the workspace's default agent.
*/
type SimulationInput struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	AgentID     *ids.UUID
	Agent       *agents.Definition
	ProcedureID ids.UUID
	Name        string
	Definition  procedure.Definition
	Version     int32
	History     []SimMessage
	Message     string
	Persona     SimPersona
	Mocks       map[string]actionkit.Mock
	State       *SimState
	Confirmed   []string
	StreamID    string
	Now         *time.Time
}

/*
SimHandoff is how a simulated turn handed over.
*/
type SimHandoff struct {
	Reason   string `json:"reason"`
	Note     string `json:"note,omitempty"`
	TeamID   string `json:"team_id,omitempty"`
	Priority string `json:"priority,omitempty"`
}

/*
SimulationResult is what the simulated turn did: the run, the reply and
outcome, the handover, the state to hand back, the procedure trace, the
action calls, and whether the procedure under test was followed.
*/
type SimulationResult struct {
	RunID          ids.UUID
	Reply          string
	Outcome        string
	Handoff        *SimHandoff
	State          SimState
	ProcedureTrace json.RawMessage
	Calls          []procedure.ObservedCall
	Matched        bool
}

/*
simulation is a simulated turn's setup and what it saw.
*/
type simulation struct {
	in      SimulationInput
	mocks   map[string]actionkit.Mock
	tested  string
	lookup  func(string) (any, bool)
	calls   []procedure.ObservedCall
	pending []SimPending
}

/*
ErrSimulation marks a simulation that cannot run as asked.
*/
var ErrSimulation = errors.New("pipeline: simulation cannot run")

/*
Simulate runs one simulated turn.
*/
func (r *Runner) Simulate(ctx context.Context, in SimulationInput) (*SimulationResult, error) {
	r.prepare()
	var st *run
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		e, sim, err := r.loadSimulation(ctx, tx, in)
		if err != nil {
			return err
		}
		row, err := r.createRun(ctx, tx, e, Args{
			WorkspaceID: in.WorkspaceID, Kind: simulationKind, MemberID: &in.MemberID,
		})
		if err != nil {
			return err
		}
		st = &run{
			env: e, rec: r.newRecorder(row), args: Args{Kind: simulationKind, MemberID: &in.MemberID},
			settings: settingsFor(e), sim: sim,
		}
		st.actions.identity = actionkit.Identity{Verified: in.Persona.Verified, Contact: e.Contact}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("simulation open: %w", err)
	}

	d := failed("provider_unavailable", errors.New("no answer model"))
	if pauseGate(st.env) != "provider_unavailable" {
		d = r.turn(ctx, st)
	}

	err = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if err := r.finishRun(ctx, tx, st, d); err != nil {
			return err
		}
		return r.recordSpend(ctx, tx, st, d)
	})
	if err != nil {
		return nil, fmt.Errorf("simulation finish: %w", err)
	}
	return r.simulationResult(st, d), nil
}

/*
loadSimulation builds the simulated world: the workspace and agent, the
persona as the customer, the conversation so far, the live procedures
with the one under test in its working copy, and the state handed back.
*/
func (r *Runner) loadSimulation(ctx context.Context, tx *database.Tx, in SimulationInput) (*env, *simulation, error) {
	q := models.New(tx)
	e, err := r.simulatedEnv(ctx, q, in)
	if err != nil {
		return nil, nil, err
	}
	sim := &simulation{in: in, mocks: maps.Clone(in.Mocks), lookup: personaLookup(in.Persona, e.ChannelType)}
	if sim.mocks == nil {
		sim.mocks = map[string]actionkit.Mock{}
	}
	if err := r.loadSimProcedures(ctx, q, e, sim); err != nil {
		return nil, nil, err
	}
	return e, sim, r.loadTestResponses(ctx, q, e.Workspace.ID, sim)
}

/*
simulatedEnv is the run's world with the persona as the customer and
the scripted turns as the conversation.
*/
func (r *Runner) simulatedEnv(ctx context.Context, q *models.Queries, in SimulationInput) (*env, error) {
	agent, err := simulatedAgent(ctx, q, in)
	if err != nil {
		return nil, err
	}
	ws, err := q.FindWorkspace(ctx, in.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("find workspace: %w", err)
	}
	e := &env{Workspace: ws, Agent: agent, ChannelType: cmpOr(in.Persona.Channel, "widget"), now: simulatedNow(in, r)}
	e.Conversation = models.Conversation{WorkspaceID: ws.ID}
	if id, ok := decodePublic(models.PrefixInbox, in.Persona.InboxID); ok {
		e.Conversation.InboxID = id
	}
	e.Contact = personaContact(ws.ID, in.Persona)
	e.Conversation.ContactID = e.Contact.ID
	e.History, e.Pending = simulatedTurns(in)
	e.AnswerKey = keyFor(ctx, q, ws.ID, agent.AnswerKeyID)
	e.ClassifierKey = e.AnswerKey
	e.Version = strv(agent.PromptVersion)
	if r.Prompts != nil {
		e.Version = r.Prompts.Version(e.Version)
		e.Defaults = r.Prompts.Defaults(e.Version, r.family(e.AnswerKey, e.Agent.AnswerModel))
	}
	if e.Triage, err = loadTriage(ctx, q, e); err != nil {
		return nil, err
	}
	if e.GuidanceArticles, err = guidanceArticles(ctx, q, agent); err != nil {
		return nil, err
	}
	loadTicketContext(ctx, q, e)
	online, err := q.CountOnlineMembers(ctx, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("count online members: %w", err)
	}
	e.Facts.onlineMembers, e.Facts.withinHours = online, true
	return e, nil
}

/*
simulatedTurns are the conversation so far and the customer's new
message, as messages.
*/
func simulatedTurns(in SimulationInput) ([]models.Message, []models.Message) {
	var history, pending []models.Message
	for _, m := range in.History {
		author := enums.AuthorTypeContact
		if m.From == "agent" {
			author = enums.AuthorTypeAgent
		}
		history = append(history, models.Message{
			AuthorType: string(author), Type: string(enums.MessageTypeMessage), Body: new(m.Text),
		})
	}
	if strings.TrimSpace(in.Message) != "" {
		pending = []models.Message{{
			AuthorType: string(enums.AuthorTypeContact), Type: string(enums.MessageTypeMessage), Body: new(in.Message),
		}}
	}
	return history, pending
}

/*
simulatedNow is the moment a simulated turn happens at: the one the
simulation keeps, or else now.
*/
func simulatedNow(in SimulationInput, r *Runner) time.Time {
	if in.Now != nil {
		return *in.Now
	}
	return r.Clock.Now()
}

/*
personaContact is the made-up customer as a contact, every attribute
trusted.
*/
func personaContact(ws ids.UUID, p SimPersona) *models.Contact {
	attrs, _ := json.Marshal(cmpMap(p.Attributes))
	c := &models.Contact{
		ID: ids.New(), WorkspaceID: ws, Kind: "lead", CustomAttributes: attrs,
		TrustedAttributeKeys: slices.Sorted(maps.Keys(p.Attributes)),
	}
	if p.Name != "" {
		c.Name = new(p.Name)
	}
	if p.Email != "" {
		c.Email, c.EmailVerified = new(p.Email), p.Verified
	}
	external := p.ExternalID
	if external == "" && p.Verified && p.Email == "" {
		external = testIdentity(p)
	}
	if external != "" {
		c.ExternalID, c.Kind = new(external), "user"
	}
	return c
}

/*
testIdentity is the id a made-up customer who is signed in but was given
no id or email is known by. A signed-in customer is always known to the
workspace by something, so actions that look them up can run, answered
by their test results like any other; the id is drawn from the
customer's name, so it stays the same on every run.
*/
func testIdentity(p SimPersona) string {
	return "test_" + hashing.SHA256Hex([]byte(p.Name))[:12]
}

/*
personaLookup is how a simulated procedure's rules read the persona.
*/
func personaLookup(p SimPersona, channel string) func(string) (any, bool) {
	return func(field string) (any, bool) {
		var v any
		switch {
		case field == "conversation.channel":
			v = channel
		case field == "conversation.inbox_id":
			v = p.InboxID
		case field == "conversation.verified":
			v = p.Verified
		case field == "contact.email":
			v = p.Email
		case field == "contact.name":
			v = p.Name
		case field == "contact.external_id":
			v = p.ExternalID
		case strings.HasPrefix(field, "contact.attr.") || strings.HasPrefix(field, "contact.custom."):
			v = p.Attributes[strings.TrimPrefix(strings.TrimPrefix(field, "contact.attr."), "contact.custom.")]
		case strings.HasPrefix(field, "company.attr.") || strings.HasPrefix(field, "company.custom."):
			v = p.Company[strings.TrimPrefix(strings.TrimPrefix(field, "company.attr."), "company.custom.")]
		}
		if s, ok := v.(string); v == nil || (ok && s == "") {
			return nil, false
		}
		return v, true
	}
}

/*
confirmPending runs the waiting writes the tester confirmed or
approved, with their mocks, and returns the note that tells the agent
how each went, as the follow-up after a real confirmation would.
*/
func (r *Runner) confirmPending(ctx context.Context, st *run) string {
	sim := st.sim
	var notes []string
	var still []SimPending
	for _, p := range sim.pending {
		if !slices.Contains(sim.in.Confirmed, p.ActionID) {
			still = append(still, p)
			continue
		}
		live, ok := r.liveAction(ctx, st, p.ActionID)
		if !ok {
			continue
		}
		args, _ := json.Marshal(p.Args)
		reply := r.runMock(st, live, args, true)
		notes = append(notes, "<system_note>The request "+p.Tool+" ("+p.Summary+") was confirmed and ran. Its "+
			"result: "+reply.Text+" Tell the customer how it went.</system_note>")
	}
	sim.pending = still
	return strings.Join(notes, "\n")
}

/*
liveAction finds a published action by public id, as the turn would
offer it to the persona.
*/
func (r *Runner) liveAction(ctx context.Context, st *run, public string) (actionkit.Live, bool) {
	if r.Actions == nil {
		return actionkit.Live{}, false
	}
	id, ok := decodePublic(models.PrefixAiAction, public)
	if !ok {
		return actionkit.Live{}, false
	}
	in := r.offerInput(st)
	in.MustInclude = []ids.UUID{id}
	offered, err := r.Actions.Offer(ctx, in)
	if err != nil {
		return actionkit.Live{}, false
	}
	for _, live := range offered {
		if live.ActionID == id {
			return live, true
		}
	}
	return actionkit.Live{}, false
}

/*
mockAction answers an action the simulated agent called with its mock.
*/
func (r *Runner) mockAction(_ context.Context, st *run, live actionkit.Live, args json.RawMessage) string {
	return r.runMock(st, live, args, false).Text
}

/*
runMock answers one call with its mock and records it as the turn
would a real one.
*/
func (r *Runner) runMock(st *run, live actionkit.Live, args json.RawMessage, agreed bool) actionkit.MockReply {
	public := models.PublicID(models.PrefixAiAction, live.ActionID)
	var mock *actionkit.Mock
	if m, ok := st.sim.mocks[public]; ok {
		mock = &m
	}
	reply := actionkit.MockCall(actionkit.MockInput{
		Live: live, Args: args, Identity: st.actions.identity, Mock: mock, Agreed: agreed,
	})
	st.sim.calls = append(st.sim.calls, procedure.ObservedCall{
		ActionID: public, Tool: live.Spec.Name, Args: reply.Input, Succeeded: reply.Succeeded,
	})
	if reply.Awaiting || reply.Pending {
		state := "pending_approval"
		if reply.Awaiting {
			state = "awaiting_confirmation"
		}
		st.sim.pending = append(st.sim.pending, SimPending{
			ActionID: public, Tool: live.Spec.Name, Args: reply.Input, State: state, Summary: reply.Call.Summary,
		})
	}
	if reply.Call != nil {
		st.record(live, reply.CallReply)
	}
	r.recordProcedureCall(st, live, reply.CallReply, reply.Result)
	return reply
}

/*
simulationResult reports what the simulated turn did.
*/
func (r *Runner) simulationResult(st *run, d *decision) *SimulationResult {
	out := &SimulationResult{RunID: st.rec.Run.ID, Outcome: d.outcome, Calls: st.sim.calls}
	if d.writesMessage() {
		out.Reply = strings.ReplaceAll(r.reply(st.env, d), ticketReference, sampleReference)
	}
	if out.Calls == nil {
		out.Calls = []procedure.ObservedCall{}
	}
	if st.handoff != nil || d.outcome == outcomeHandoff {
		h := &SimHandoff{Reason: d.handoffReason}
		if st.handoff != nil {
			h.Reason, h.Note = cmpOr(st.handoff.reason, d.handoffReason), st.handoff.note
		}
		if st.changes.team != nil {
			h.TeamID = models.PublicID(models.PrefixTeam, *st.changes.team)
		}
		if st.changes.priority != nil && *st.changes.priority {
			h.Priority = "high"
		}
		out.Handoff = h
	}
	out.State = SimState{
		Called: []string{}, Completed: []string{}, Branches: map[string]string{}, Outputs: map[string]any{},
		Pending: st.sim.pending,
	}
	if out.State.Pending == nil {
		out.State.Pending = []SimPending{}
	}
	if p := st.proc; p != nil {
		out.Matched = p.live.publicID() == st.sim.tested
		out.State.ProcedureID, out.State.Status = p.live.publicID(), p.status
		if out.Handoff != nil {
			out.State.Status = procedureHandedOff
		}
		out.State.Called = append(out.State.Called, p.called...)
		out.State.Completed = append(out.State.Completed, p.done...)
		out.State.Outputs, out.State.Branches = p.outputs, cmpSides(p.sides)
		out.ProcedureTrace = r.procedureTrace(st)
	}
	return out
}

/*
cmpMap is a map, empty when nil.
*/
func cmpMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
