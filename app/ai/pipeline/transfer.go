/*
|--------------------------------------------------------------------------
| Agent Transfers
|--------------------------------------------------------------------------
|
| An agent may hand a conversation to another agent it is allowed to,
| silently: the customer sees no goodbye, the other agent answers the
| same message, and the team sees a timeline entry saying who took over
| and why. Loops are cut short: a conversation moves at most twice, and
| never back to an agent it already left. The procedure the first agent
| was following ends with the transfer.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
toolTransfer is the built-in tool that hands the conversation to
another agent.
*/
const toolTransfer = "transfer_agent"

/*
maxTransfers is how many times one conversation may move between agents.
*/
const maxTransfers = 2

/*
transferEvent names the timeline row a transfer leaves.
*/
const transferEvent = "ai.agent_transferred"

/*
transferRequest is the agent the conversation goes to and why.
*/
type transferRequest struct {
	to      models.AiAgent
	summary string
}

/*
loadAgentExtras reads what the answering agent's settings point at: the
articles its guidance cites and the agents it may hand over to.
*/
func loadAgentExtras(ctx context.Context, q *models.Queries, e *env) error {
	var err error
	if e.GuidanceArticles, err = guidanceArticles(ctx, q, e.Agent); err != nil {
		return err
	}
	e.TransferTargets, err = transferTargets(ctx, q, e)
	return err
}

/*
transferTargets are the agents the answering agent may hand this
conversation to: live, allowed by its settings, and never one the
conversation already left. A conversation that moved twice may not
move again, and an agent a workflow step chose stays put.
*/
func transferTargets(ctx context.Context, q *models.Queries, e *env) ([]models.AiAgent, error) {
	allowed := e.Agent.TransferAgentIds
	left := e.Conversation.AiTransferredFromIds
	if len(allowed) == 0 || len(left) >= maxTransfers || (e.WorkflowStep != nil && e.WorkflowStep.AgentID != nil) {
		return nil, nil
	}
	rows, err := q.ListAgentsByID(ctx, models.ListAgentsByIDParams{WorkspaceID: e.Workspace.ID, Ids: allowed})
	if err != nil {
		return nil, fmt.Errorf("list transfer targets: %w", err)
	}
	out := rows[:0]
	for _, a := range rows {
		if a.ID != e.Agent.ID && a.Enabled && !slices.Contains(left, a.ID) {
			out = append(out, a)
		}
	}
	return out, nil
}

/*
agentLines lists the agents a conversation may be passed to, one a
line: its id, its name and role, and what the team says it handles. An
agent the team has not described says so, so the answering agent goes
by its name and role alone.
*/
func agentLines(targets []models.AiAgent) string {
	lines := make([]string, len(targets))
	for i, a := range targets {
		handles := strings.TrimSpace(a.Description)
		if handles == "" {
			handles = "The team has not described what it handles."
		}
		lines[i] = "- " + a.Name + " (id " + models.PublicID(models.PrefixAiAgent, a.ID) + ", " +
			strings.ReplaceAll(a.Role, "_", " ") + " agent): " + handles
	}
	return strings.Join(lines, "\n")
}

/*
jobText is the part of the system prompt that tells the agent its own
job, as the team described it, or nothing when it has no description.
*/
func (r *Runner) jobText(e *env) string {
	job := strings.TrimSpace(e.Agent.Description)
	if job == "" {
		return ""
	}
	return r.render(e, "job.md", prompts.Vars{"job": job})
}

/*
transfersText is the part of the system prompt that lists the agents
this conversation may be passed to, with what each handles and when to
pass it on. It is empty when no transfer is possible, so an agent that
cannot transfer is never told about one.
*/
func (r *Runner) transfersText(e *env) string {
	if len(e.TransferTargets) == 0 {
		return ""
	}
	return r.render(e, "transfers.md", prompts.Vars{"agents": agentLines(e.TransferTargets)})
}

/*
transferSpec describes transfer_agent, naming every agent it may hand
the conversation to.
*/
func transferSpec(targets []models.AiAgent) contracts.ToolSpec {
	choices := make([]string, len(targets))
	for i, a := range targets {
		choices[i] = models.PublicID(models.PrefixAiAgent, a.ID)
	}
	return contracts.ToolSpec{
		Name: toolTransfer,
		Description: "Pass this conversation to another AI agent of the team, when the customer's request is " +
			"that agent's job as your instructions describe it. The other agent answers the customer's " +
			"message itself, so after calling this write nothing more. The customer is not told; the team " +
			"sees the transfer. Agents:\n" + agentLines(targets),
		Schema: objectSchema(map[string]any{
			"agent_id": enumProperty(choices),
			"summary":  textProperty("One sentence for the team: what the customer needs and why it moves."),
		}, "agent_id", "summary"),
	}
}

/*
transferAgent records the transfer the model asked for; the turn ends
at once and the other agent answers.
*/
func (r *Runner) transferAgent(_ context.Context, st *run, args json.RawMessage) (string, error) {
	var in struct {
		AgentID string `json:"agent_id"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", errToolArguments
	}
	for _, a := range st.env.TransferTargets {
		if models.PublicID(models.PrefixAiAgent, a.ID) == in.AgentID {
			st.transfer = &transferRequest{to: a, summary: truncate(strings.TrimSpace(in.Summary), 300)}
			return `{"ok":true,"note":"transferred; write nothing more"}`, nil
		}
	}
	return "", errToolArguments
}

/*
writeTransfer moves the conversation to the other agent: it records
the agent it left, counts what the turn cost on the conversation, ends
the procedure it followed and leaves the timeline entry the team reads.
*/
func (r *Runner) writeTransfer(ctx context.Context, tx *database.Tx, st *run) error {
	e, t := st.env, st.transfer
	q := models.New(tx)
	if err := q.TransferConversationAgent(ctx, models.TransferConversationAgentParams{
		WorkspaceID: e.Workspace.ID, ID: e.Conversation.ID, ToAgentID: &t.to.ID, FromAgentID: e.Agent.ID,
	}); err != nil {
		return fmt.Errorf("transfer conversation: %w", err)
	}
	if err := q.AddConversationAiCost(ctx, models.AddConversationAiCostParams{
		WorkspaceID: e.Workspace.ID, ID: e.Conversation.ID, CostMicros: st.rec.costMicros, Now: r.Clock.Now(),
	}); err != nil {
		return fmt.Errorf("count transfer cost: %w", err)
	}
	if err := q.EndProcedureStates(ctx, models.EndProcedureStatesParams{
		WorkspaceID: e.Workspace.ID, ConversationID: e.Conversation.ID, Status: procedureExited, Now: r.Clock.Now(),
	}); err != nil {
		return fmt.Errorf("end procedure states: %w", err)
	}
	return r.agentWriter(tx, e).Record(ctx, e.Conversation, transferEvent, map[string]any{
		"to_agent": map[string]any{"id": models.PublicID(models.PrefixAiAgent, t.to.ID), "name": t.to.Name},
		"summary":  t.summary,
	})
}

/*
reviewOrTransfer reviews the reply, or, when the turn handed the
conversation to another agent, records that instead.
*/
func (r *Runner) reviewOrTransfer(ctx context.Context, st *run, draft string) *decision {
	if st.transfer != nil {
		return transferred(st.transfer)
	}
	return r.review(ctx, st, draft)
}

/*
transferred is the decision of a turn that handed the conversation on.
*/
func transferred(t *transferRequest) *decision {
	return &decision{outcome: outcomeTransferred, skipReason: "to " + t.to.Name}
}

/*
SimTransfer is a transfer a simulated turn made: the agent it went to,
by public id and name, the agent it left, and the team's summary. To
is the agent's id, for the simulator to answer as it next.
*/
type SimTransfer struct {
	AgentID  string   `json:"agent_id"`
	Name     string   `json:"name"`
	FromName string   `json:"from_name"`
	Summary  string   `json:"summary"`
	To       ids.UUID `json:"-"`
}

/*
simTransfer is the simulated turn's transfer, or nil.
*/
func simTransfer(st *run) *SimTransfer {
	t := st.transfer
	if t == nil {
		return nil
	}
	return &SimTransfer{
		AgentID: models.PublicID(models.PrefixAiAgent, t.to.ID), Name: t.to.Name, FromName: st.env.Agent.Name,
		Summary: t.summary, To: t.to.ID,
	}
}
