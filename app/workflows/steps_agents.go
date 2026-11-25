/*
|--------------------------------------------------------------------------
| AI Agent Steps
|--------------------------------------------------------------------------
|
| Assigning an agent pins it to the conversation in place of the one it
| had, so it answers from the next customer message on; an agent that
| was archived or is gone is skipped. Turning AI off hands the
| conversation away from every agent, the way a handover to the team
| does, so no agent answers until a teammate or a Turn on AI step lets
| one back in. Turning AI on clears that, and any handover before it.
|
*/

package workflows

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
turnedOffReason is the handoff reason a Turn off AI step records.
*/
const turnedOffReason = "turned_off_by_workflow"

/*
agentExecutors returns the executors of the AI agent steps.
*/
func agentExecutors() map[string]executor {
	return map[string]executor{
		workflowspec.KindAssignAgent: runAssignAgent,
		workflowspec.KindTurnOffAI:   runTurnOffAI,
		workflowspec.KindTurnOnAI:    runTurnOnAI,
	}
}

/*
runAssignAgent pins the step's agent to the conversation.
*/
func runAssignAgent(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	id, ok := st.decode(models.PrefixAiAgent, settings[workflowspec.AssignAgentSettings](s).AgentID)
	if !ok {
		return skipped("agent_gone"), nil
	}

	_, err := st.q.FindAgentByID(ctx, models.FindAgentByIDParams{WorkspaceID: st.conv.WorkspaceID, ID: id})
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return skipped("agent_gone"), nil
		}

		return outcome{}, fmt.Errorf("find agent: %w", err)
	}

	n, err := st.q.AssignConversationAgent(ctx, models.AssignConversationAgentParams{
		WorkspaceID: st.conv.WorkspaceID, ID: st.conv.ID, AgentID: &id, Now: st.now(),
	})
	if err != nil {
		return outcome{}, fmt.Errorf("assign agent: %w", err)
	}

	if n > 0 {
		st.conv.AiAgentID = &id
	}

	return changed(n > 0), nil
}

/*
runTurnOffAI keeps every agent off the conversation.
*/
func runTurnOffAI(ctx context.Context, st *state, _ workflowspec.Step) (outcome, error) {
	if st.conv.AiState == "handed_off" {
		return changed(false), nil
	}

	reason, now := turnedOffReason, st.now()
	if err := st.q.HandoffConversationAi(ctx, models.HandoffConversationAiParams{
		WorkspaceID: st.conv.WorkspaceID, ID: st.conv.ID, Reason: &reason, Now: &now,
	}); err != nil {
		return outcome{}, fmt.Errorf("turn off ai: %w", err)
	}

	st.conv.AiState = "handed_off"

	return changed(true), nil
}

/*
runTurnOnAI lets the agent answer the conversation again.
*/
func runTurnOnAI(ctx context.Context, st *state, _ workflowspec.Step) (outcome, error) {
	if st.conv.AiState != "handed_off" {
		return changed(false), nil
	}

	if err := st.q.ReadyConversationAi(ctx, models.ReadyConversationAiParams{
		WorkspaceID: st.conv.WorkspaceID, ID: st.conv.ID, Now: st.now(),
	}); err != nil {
		return outcome{}, fmt.Errorf("turn on ai: %w", err)
	}

	st.conv.AiState = "off"

	return changed(true), nil
}
