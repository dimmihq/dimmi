package ai_test

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
state is everything a test asserts on after a run.
*/
type state struct {
	conv     models.Conversation
	messages []models.Message
	run      models.AiRun
	steps    []models.AiRunStep
	runs     []models.AiRun
}

/*
agentReplies are the agent's public messages.
*/
func (s state) agentReplies() []models.Message {
	var out []models.Message
	for _, m := range s.messages {
		if m.AuthorType == "agent" && m.Type == "message" {
			out = append(out, m)
		}
	}
	return out
}

/*
timeline are the conversation's timeline events, as stored.
*/
func (s state) timeline() []string {
	var out []string
	for _, m := range s.messages {
		if m.Type == "event" {
			out = append(out, string(m.Event))
		}
	}
	return out
}

/*
notes are the conversation's internal notes.
*/
func (s state) notes() []models.Message {
	var out []models.Message
	for _, m := range s.messages {
		if m.Type == "note" {
			out = append(out, m)
		}
	}
	return out
}

/*
stepKinds lists the run's step kinds in order.
*/
func (s state) stepKinds() []string {
	out := make([]string, len(s.steps))
	for i, st := range s.steps {
		out[i] = st.Kind
	}
	return out
}

/*
load reads the conversation, its messages, its runs and the given
run's steps.
*/
func (h *harness) load(conv, runID ids.UUID) state {
	h.t.Helper()
	var s state
	ws := h.d.TC.Workspace.Workspace.ID
	h.d.TC.InWorkspace(func(ctx context.Context) error {
		q := models.New(h.d.TC.App.DB)
		var err error
		if s.conv, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: conv}); err != nil {
			return err
		}
		s.messages = h.messages(ctx, conv)
		if s.runs, err = q.ListConversationRuns(ctx, models.ListConversationRunsParams{
			WorkspaceID: ws, ConversationID: &conv, PageLimit: 50,
		}); err != nil {
			return err
		}
		if runID == (ids.UUID{}) {
			return nil
		}
		if s.run, err = q.FindRun(ctx, models.FindRunParams{WorkspaceID: ws, ID: runID}); err != nil {
			return err
		}
		s.steps, err = q.ListRunSteps(ctx, models.ListRunStepsParams{WorkspaceID: ws, RunID: runID})
		return err
	})
	return s
}
