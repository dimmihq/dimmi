/*
|--------------------------------------------------------------------------
| Action Follow-Ups
|--------------------------------------------------------------------------
|
| When a request the agent made ends (run, declined, failed, or lapsed
| while the team decided) the agent is woken to tell the customer, in
| their conversation, whether or not they are still there. The run sees
| the conversation and an update saying exactly how the request ended,
| and writes the message itself, in the customer's language. It obeys
| the same gate as any turn: a conversation a teammate took over hears
| nothing from the agent, and the timeline entry already tells the team.
| A request that lapsed hands the conversation to the team with high
| priority, and one that failed for reasons the customer cannot act on
| goes to the team too.
|
*/

package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RunActionFollowUp tells the customer how an action request ended. A
request already reported, or still waiting, is skipped.
*/
func (r *Runner) RunActionFollowUp(ctx context.Context, workspaceID, callID ids.UUID) (*Result, error) {
	r.prepare()

	st, early, err := r.openFollowUp(ctx, workspaceID, callID)
	if err != nil {
		return nil, fmt.Errorf("follow-up open: %w", err)
	}
	if early != nil {
		return early, nil
	}

	d := r.followUpTurn(ctx, st)
	res, err := r.close(ctx, st, d)
	if err != nil {
		return nil, fmt.Errorf("follow-up commit: %w", err)
	}
	return res, nil
}

/*
openFollowUp loads the request and its conversation, passes the gate
and opens the run.
*/
func (r *Runner) openFollowUp(ctx context.Context, workspaceID, callID ids.UUID) (*run, *Result, error) {
	var st *run
	var early *Result
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		call, err := q.FindActionCall(ctx, models.FindActionCallParams{WorkspaceID: workspaceID, ID: callID})
		if errors.Is(err, database.ErrNotFound) {
			early = &Result{Outcome: outcomeSkipped, SkipReason: "call_missing"}
			return nil
		}
		if err != nil {
			return fmt.Errorf("find action call: %w", err)
		}
		if reason := unreportable(call); reason != "" {
			early = &Result{Outcome: outcomeSkipped, SkipReason: reason}
			return nil
		}
		st, early, err = r.startFollowUp(ctx, tx, call)
		return err
	})
	return st, early, err
}

/*
unreportable names why a request needs no follow-up.
*/
func unreportable(call models.AiActionCall) string {
	switch {
	case call.ReportedAt != nil:
		return "already_reported"
	case call.ConversationID == nil || call.Mode != "live":
		return "no_conversation"
	case call.State == "expired" && !actionkit.WasAwaitingApproval(call):
		return "nothing_to_report"
	}
	switch call.State {
	case "succeeded", "failed", "declined", "expired":
		return ""
	}
	return "still_waiting"
}

/*
startFollowUp loads the conversation and opens the run, or marks the
request reported when the gate keeps the agent quiet.
*/
func (r *Runner) startFollowUp(ctx context.Context, tx *database.Tx, call models.AiActionCall) (*run, *Result, error) {
	args := Args{WorkspaceID: call.WorkspaceID, ConversationID: *call.ConversationID, Kind: "conversation"}
	e, err := r.loadEnv(ctx, tx, args)
	if err != nil {
		return nil, nil, err
	}
	q := models.New(tx)
	if e == nil {
		return nil, &Result{Outcome: outcomeSkipped, SkipReason: "agent_missing"}, r.markReported(ctx, q, call)
	}

	settings := settingsFor(e)
	if call.State == "expired" {
		settings.RaisePriority = true
	}
	if reason := r.followUpGate(e, settings); reason != "" {
		return nil, &Result{Outcome: outcomeSkipped, SkipReason: reason}, r.markReported(ctx, q, call)
	}

	row, err := r.createRun(ctx, tx, e, args)
	if err != nil {
		return nil, nil, err
	}
	if err := q.SetRunTriggerActionCall(ctx, models.SetRunTriggerActionCallParams{
		WorkspaceID: call.WorkspaceID, ID: row.ID, ActionCallID: &call.ID, Now: r.Clock.Now(),
	}); err != nil {
		return nil, nil, fmt.Errorf("name follow-up call: %w", err)
	}

	st := &run{env: e, rec: r.newRecorder(row), args: args, settings: settings}
	st.actions.followUp, st.actions.noWrites = &call, true
	return st, nil, nil
}

/*
followUpGate is the gate of a follow-up: the agent stays quiet where a
turn would, except that no new customer message is needed.
*/
func (r *Runner) followUpGate(e *env, s *TurnSettings) string {
	if reason := eligibility(e); reason != "" {
		return reason
	}
	if s.Workflow.Controlled && !s.Workflow.StepActive {
		return "workflow_controls_ai"
	}
	if reason := humanGate(e); reason != "" {
		return reason
	}
	if e.Conversation.AiState != aiStateActive {
		return "agent_not_in_conversation"
	}
	return pauseGate(e)
}

/*
followUpTurn writes the follow-up: the conversation with the update as
its last turn, tools that only read, and a handoff set up front when
the team must take it from here.
*/
func (r *Runner) followUpTurn(ctx context.Context, st *run) *decision {
	call := *st.actions.followUp
	switch {
	case call.State == "expired":
		st.handoff = &handoffRequest{reason: "action_expired"}
	case call.State == "failed" && !actionkit.Explainable(call):
		st.handoff = &handoffRequest{reason: "action_failed"}
	}
	st.actions.answered = call.State == "succeeded"

	llm, err := r.LLM.ForKey(ctx, *st.env.AnswerKey)
	if err != nil {
		return failed("provider_unavailable", err)
	}
	stopTyping := r.typing(ctx, st.env)
	defer stopTyping()

	st.loop = &loop{llm: llm, system: r.render(st.env, "answer.system.md", r.answerVars(st))}
	st.loop.messages = r.envelope(st.env, actionkit.FollowUpNote(call))
	r.offerActions(ctx, st)

	draft, err := r.agentLoop(ctx, st)
	if err != nil {
		return failed("provider_error", err)
	}
	d := r.review(ctx, st, draft)
	if d.outcome == outcomeUnknown && d.reply == "" {
		return &decision{outcome: outcomeGreeted, replyKey: "action_update"}
	}
	return d
}

/*
markReported closes a request whose customer the agent will not tell.
*/
func (r *Runner) markReported(ctx context.Context, q *models.Queries, call models.AiActionCall) error {
	return q.MarkActionCallReported(ctx, models.MarkActionCallReportedParams{
		WorkspaceID: call.WorkspaceID, ID: call.ID, Now: r.Clock.Now(),
	})
}
