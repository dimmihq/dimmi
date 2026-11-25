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
| before it hands it to the team.
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
TryInput is one chat to play once: the copy, the customer's first
message, their situation, and who they are.
*/
type TryInput struct {
	Version  string
	Message  string
	Scenario string
	Persona  json.RawMessage
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
	messages, err := json.Marshal([]procedures.TestMessage{{Text: text}})
	if err != nil {
		return Result{}, fmt.Errorf("write the first message: %w", err)
	}
	persona := in.Persona
	if len(persona) == 0 {
		persona = json.RawMessage(`{}`)
	}
	c := models.WorkflowSimulation{
		WorkspaceID: m.Workspace.ID, WorkflowID: workflowID, Name: "Operator's try", Messages: messages,
		Persona: persona, Scenario: strings.TrimSpace(in.Scenario), ActionMocks: json.RawMessage(`{}`),
		Assertions: json.RawMessage(`[]`),
	}

	return s.play(ctx, playing{m: m, c: c, copy: copied, version: in.Version}), nil
}
