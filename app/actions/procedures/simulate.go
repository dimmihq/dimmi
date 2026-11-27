/*
|--------------------------------------------------------------------------
| Simulating Procedures
|--------------------------------------------------------------------------
|
| A teammate talks to dimmi as a made-up customer to see how it follows
| a procedure, the working copy by default, before anyone else does. No
| action reaches the team's systems: each answers with the mock given or
| its last live test's result. A real past conversation can be replayed:
| its earlier messages become the history and the customer's last one
| the message. Each turn hands back the procedure's state for the next.
|
*/

package procedures

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
Simulation limits: the history a turn carries and the words of one
message.
*/
const (
	maxSimHistory = 40
	maxSimMessage = 4000
)

/*
The failures of the simulator.
*/
var (
	ErrSimulatorUnavailable = fault.New(fault.Unavailable, "simulator_unavailable",
		"The simulator is not available: dimmi's agent is not set up.")
	ErrSimulationInvalid = fault.New(fault.Invalid, "simulation_invalid", "The simulation cannot run as asked.")
	ErrNotLive           = fault.New(fault.Invalid, "procedure_not_live", "This procedure is not live.").
				OnField("target")
)

/*
Simulator runs one simulated turn of the agent.
*/
type Simulator interface {
	/*
		Simulate runs the turn.
	*/
	Simulate(ctx context.Context, in pipeline.SimulationInput) (*pipeline.SimulationResult, error)
}

/*
Customer plays the customer of a saved simulation when it runs, and
writes the scenario it plays from when one is saved.
*/
type Customer interface {
	/*
		PlayCustomer returns the customer's next move.
	*/
	PlayCustomer(ctx context.Context, in pipeline.CustomerTurn) (pipeline.CustomerMove, error)
	/*
		DescribeCustomer writes the scenario of a saved chat's customer.
	*/
	DescribeCustomer(ctx context.Context, in pipeline.CustomerProfile) (string, error)
}

/*
Vectors embeds a procedure's description.
*/
type Vectors interface {
	/*
		EmbedDocument returns the vector and the key of the model that
		made it.
	*/
	EmbedDocument(ctx context.Context, workspaceID ids.UUID, text string) ([]float32, string, error)
}

/*
Judge decides whether a transcript meets a run's written checks.
*/
type Judge interface {
	/*
		JudgeRubrics answers every written check of one run, in the
		order given, each with the judge's reason when it has one.
	*/
	JudgeRubrics(ctx context.Context, in pipeline.Rubrics) ([]procedure.Judgment, error)
}

/*
SimulateInput is one simulated turn: the procedure and which copy to
test, the conversation so far or a real one to replay, the customer's
message, the persona, the mocks by action public id, the state handed
back by the last turn, and the waiting writes confirmed since. HeldBy
is the agent an earlier turn transferred the chat to, which answers
from then on.
*/
type SimulateInput struct {
	Membership         authz.Membership
	ID                 ids.UUID
	Target             string
	History            []pipeline.SimMessage
	Message            string
	Persona            pipeline.SimPersona
	Mocks              map[string]actionkit.Mock
	State              *pipeline.SimState
	Confirmed          []string
	SourceConversation *ids.UUID
	StreamID           string
	ChatID             ids.UUID
	Now                *time.Time
	HeldBy             *ids.UUID
}

/*
Simulate runs one simulated turn of a procedure.
*/
func (p *Procedures) Simulate(ctx context.Context, in SimulateInput) (*pipeline.SimulationResult, error) {
	m := in.Membership
	if !m.Can("ai_agent:manage") {
		return nil, exceptions.ErrForbidden
	}

	if p.Simulator == nil {
		return nil, ErrSimulatorUnavailable
	}

	sim, err := p.simulation(ctx, in)
	if err != nil {
		return nil, err
	}

	res, err := p.Simulator.Simulate(ctx, sim)
	if errors.Is(err, pipeline.ErrNoAgent) {
		return nil, ErrSimulatorUnavailable
	}

	return res, err
}

/*
simulation builds the pipeline's input: the copy under test, the
replayed conversation when one is named, and the checked message.
*/
func (p *Procedures) simulation(ctx context.Context, in SimulateInput) (pipeline.SimulationInput, error) {
	m := in.Membership
	out := pipeline.SimulationInput{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ProcedureID: in.ID, History: in.History,
		Message: in.Message, Persona: in.Persona, Mocks: in.Mocks, State: in.State, Confirmed: in.Confirmed,
		StreamID: in.StreamID, ChatID: in.ChatID, Now: in.Now,
	}
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := find(ctx, q, m.Workspace.ID, in.ID)
		if err != nil {
			return err
		}

		out.Name = current.Name
		if out.Definition, err = underTest(current, in.Target); err != nil {
			return err
		}

		if in.SourceConversation == nil || len(in.History) > 0 {
			return nil
		}

		script, err := p.conversationScript(ctx, q, procedureRef{ws: m.Workspace.ID, id: *in.SourceConversation})
		out.History, out.Message = replay(script, in.Message)

		return err
	})
	if err != nil {
		return out, err
	}

	return out, checkSimulation(out)
}

/*
underTest is the copy of a procedure a simulation tests: the working
copy, or the live one when asked.
*/
func underTest(current models.Procedure, target string) (procedure.Definition, error) {
	raw := current.Draft
	if target == "live" {
		if current.Live == nil {
			return procedure.Definition{}, ErrNotLive
		}
		raw = current.Live
	}

	d, err := procedure.Parse(raw)
	if err != nil {
		return d, ErrInvalid.With("reason", err.Error())
	}

	return d, nil
}

/*
checkSimulation refuses a turn with nothing to answer or too much to
read.
*/
func checkSimulation(in pipeline.SimulationInput) error {
	if strings.TrimSpace(in.Message) == "" && len(in.Confirmed) == 0 {
		return ErrSimulationInvalid.With("reason", "Write the customer's message.").OnField("message")
	}

	if len(in.History) > maxSimHistory {
		return ErrSimulationInvalid.With("reason", "Keep the history to 40 messages.").OnField("history")
	}

	for _, msg := range append(slices.Clone(in.History), pipeline.SimMessage{Text: in.Message}) {
		if utf8.RuneCountInString(msg.Text) > maxSimMessage {
			return ErrSimulationInvalid.With("reason", "Keep each message to 4,000 characters.").OnField("message")
		}
	}

	if in.Persona.Channel != "" && !slices.Contains(procedure.Channels(), in.Persona.Channel) {
		return ErrSimulationInvalid.With("reason", "Pick a channel dimmi answers on.").OnField("persona")
	}

	return nil
}

/*
conversationScript reads a real conversation's public messages, oldest
first, as the simulator replays them.
*/
func (p *Procedures) conversationScript(ctx context.Context, q *models.Queries, ref procedureRef,
) ([]pipeline.SimMessage, error) {
	ws, conv, now := ref.ws, ref.id, p.Clock.Now()
	if _, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: conv}); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, exceptions.ErrNotFound
		}
		return nil, fmt.Errorf("find conversation: %w", err)
	}

	rows, err := q.ListLatestPublicMessages(ctx, models.ListLatestPublicMessagesParams{
		WorkspaceID: ws, ConversationID: conv, Now: &now, RowLimit: maxSimHistory,
	})
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	slices.Reverse(rows)
	var out []pipeline.SimMessage
	for _, r := range rows {
		if r.Body == nil || strings.TrimSpace(*r.Body) == "" || r.Type != string(enums.MessageTypeMessage) {
			continue
		}
		role := "agent"
		if r.AuthorType == string(enums.AuthorTypeContact) {
			role = "customer"
		}
		out = append(out, pipeline.SimMessage{From: role, Text: *r.Body})
	}

	return out, nil
}

/*
replay splits a conversation into the history before the customer's
last message and that message; a message given takes its place.
*/
func replay(script []pipeline.SimMessage, message string) ([]pipeline.SimMessage, string) {
	last := -1
	for i, msg := range script {
		if msg.From == "customer" {
			last = i
		}
	}

	if last < 0 {
		return script, message
	}

	if strings.TrimSpace(message) == "" {
		message = script[last].Text
	}

	return script[:last], message
}
