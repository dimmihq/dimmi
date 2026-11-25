/*
|--------------------------------------------------------------------------
| One-Off Simulations
|--------------------------------------------------------------------------
|
| Operator tries a workflow the way a teammate's saved simulation would
| run: an AI customer writes the first message and plays the rest from
| a situation, against the draft or the live version. Nothing is kept:
| no simulation is saved, no result recorded, nothing sent and no
| action run. It is how Operator checks the workflow it just built
| before it hands it to the team: it may play the version Operator
| suggested rather than the saved one, and a watcher sees each moment
| of the chat as it happens, so the teammate can follow it live.
|
*/

package workflowsims

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Watch sees each moment of a chat as it happens: started, line, step,
turn_started, turn_finished, judging and finished, with its details.
*/
type Watch func(ctx context.Context, moment string, details map[string]any)

/*
TryInput is one chat to play once: the copy, or the definition to play
instead of it, the customer's first message, their situation, who they
are, and who watches it.
*/
type TryInput struct {
	Version  string
	Spec     json.RawMessage
	Message  string
	Scenario string
	Persona  json.RawMessage
	Watch    Watch
}

/*
Try plays one chat through a workflow once and keeps nothing.
*/
func (s *Simulations) Try(ctx context.Context, m authz.Membership, workflowID ids.UUID, in TryInput) (Result, error) {
	if err := requireManage(m); err != nil {
		return Result{}, err
	}
	if s.Engine == nil {
		return Result{}, ErrSimulatorOff
	}
	text := strings.TrimSpace(in.Message)
	if text == "" {
		return Result{}, ErrSimulationInvalid.With("reason", "Write the customer's first message.").OnField("message")
	}
	if in.Version != VersionLive {
		in.Version = VersionDraft
	}
	copied, err := s.copyOf(ctx, m, workflowID, in.Version)
	if err != nil {
		return Result{}, err
	}
	if len(in.Spec) > 0 {
		copied.spec = in.Spec
	}
	messages, err := json.Marshal([]procedures.TestMessage{{Text: text}})
	if err != nil {
		return Result{}, fmt.Errorf("write the first message: %w", err)
	}
	persona := in.Persona
	if len(persona) == 0 {
		persona = json.RawMessage(`{}`)
	}
	c := models.WorkflowSimulation{
		ID: ids.New(), WorkspaceID: m.Workspace.ID, WorkflowID: workflowID, Name: "Operator's try", Messages: messages,
		Persona: persona, Scenario: strings.TrimSpace(in.Scenario), ActionMocks: json.RawMessage(`{}`),
		Assertions: json.RawMessage(`[]`),
	}

	return s.play(ctx, playing{m: m, c: c, copy: copied, version: in.Version, watch: in.Watch}), nil
}
