/*
|--------------------------------------------------------------------------
| A Simulation's Scenario
|--------------------------------------------------------------------------
|
| A saved simulation carries a scenario: who its customer is, what they
| came for, and where that stops. The simulated customer answers from
| it when the agent asks something the saved messages never covered,
| and agrees only to what it says the customer wants.
|
| When a chat is saved as a simulation and the teammate wrote no
| scenario, the model that plays the customer writes one from the
| customer's saved messages, by resources/prompts/tasks/simulation/
| scenario.md. The teammate can edit it afterwards. A scenario that
| could not be written is left empty and the simulation runs from its
| saved messages alone; saving never fails over it. The call is
| recorded on a simulation run and spends like any other.
|
*/

package pipeline

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
scenarioTokens bounds a written scenario; scenarioChars is the most a
scenario holds.
*/
const (
	scenarioTokens = 220
	scenarioChars  = 1000
)

/*
CustomerProfile is what a scenario is written from: the workspace and
teammate saving the simulation, the customer's name when known, and
what the customer did in the saved chat.
*/
type CustomerProfile struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	Name        string
	Saved       []SavedMessage
}

/*
DescribeCustomer writes the scenario of a saved chat's customer.
*/
func (r *Runner) DescribeCustomer(ctx context.Context, in CustomerProfile) (string, error) {
	r.prepare()
	st, key, err := r.openCustomer(ctx, in.WorkspaceID, in.MemberID)
	if err != nil {
		return "", err
	}
	llm, err := r.LLM.ForKey(ctx, key)
	if err != nil {
		return "", r.closeCustomer(ctx, st, err)
	}

	req := contracts.ChatRequest{
		Model:           customerModel,
		System:          []contracts.SystemBlock{{Text: prompts.Task("simulation/scenario", nil), Cacheable: true}},
		Messages:        []contracts.ChatMessage{userMessage(savedChat(in.Name, in.Saved))},
		MaxOutputTokens: scenarioTokens, Effort: contracts.EffortLow,
	}
	ctx, cancel := r.timeout(ctx, r.Timeouts.GenerateTotal)
	defer cancel()
	started := r.Clock.Now()
	res, err := llm.Chat(ctx, req)
	if res == nil {
		res = &contracts.ChatResponse{}
	}
	r.step(ctx, st, stepRecord{
		kind: enums.AIRunStepKindGenerate, llm: llm, key: &keyRef{id: key.ID, provider: key.Provider},
		model: customerModel, started: started, request: transcriptOf(req), response: res.Text,
		usage: res.Usage, err: err, requestID: res.RequestID, responseModel: res.Model,
	})
	if err != nil {
		return "", r.closeCustomer(ctx, st, err)
	}

	return truncate(strings.TrimSpace(res.Text), scenarioChars), r.closeCustomer(ctx, st, nil)
}
