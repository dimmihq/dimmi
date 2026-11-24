/*
|--------------------------------------------------------------------------
| Sorting A Conversation By Topic
|--------------------------------------------------------------------------
|
| A classify step asks the AI decision step which of the team's
| categories the conversation belongs to, and takes that category's
| path. The conversation is untrusted: the models read it as data and
| must answer with one of the categories, nothing else. A
| workspace with no model, over its spend cap, or a model that fails or
| is unsure, takes the none path, so a conversation is never stuck on
| a step that cannot decide. The model is asked by a job, off the
| conversation and outside any transaction.
|
*/

package workflows

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
Category is one category of a classify step: its path id, label and
description.
*/
type Category struct {
	ID          string
	Label       string
	Description string
}

/*
runClassify waits for the classifier, which a job asks off the
conversation.
*/
func runClassify(_ context.Context, st *state, s workflowspec.Step) (outcome, error) {
	if st.e.Decisions == nil {
		return unsorted(s, "ai_unavailable"), nil
	}

	return callingOut(false), nil
}

/*
prepareQuestion reads the customer's words and the categories, an
answer instead when there is nothing to sort.
*/
func (st *state) prepareQuestion(ctx context.Context, s workflowspec.Step) (*call, error) {
	c := settings[workflowspec.ClassifySettings](s)

	text, err := st.conversationText(ctx, c.Source)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(text) == "" {
		return &call{answer: &callAnswer{Reason: "no_text"}}, nil
	}

	var options []aidecision.Option
	for _, cat := range categories(s) {
		options = append(options, aidecision.Option{ID: cat.ID, Label: cat.Label, Description: cat.Description})
	}

	q := aidecision.Choice{Subject: st.decisionSubject(text), Instructions: c.Question, Options: options}

	return &call{kind: s.Kind, choice: q}, nil
}

/*
categories are the step's categories, one per path.
*/
func categories(s workflowspec.Step) []Category {
	var out []Category
	for _, p := range buttonPaths(s) {
		out = append(out, Category{ID: p.ID, Label: p.Label, Description: p.Description})
	}

	return out
}

/*
ask asks the decision step; one that cannot answer leaves the step
unsorted instead of failing the run.
*/
func (e *Engine) ask(ctx context.Context, q aidecision.Choice) *callAnswer {
	if e.Decisions == nil {
		return &callAnswer{Reason: "ai_unavailable"}
	}

	answer, err := e.Decisions.Choose(ctx, q)
	if err != nil {
		return &callAnswer{Reason: "ai_unavailable"}
	}

	return &callAnswer{Category: answer.OptionID, Confidence: answer.Confidence}
}

/*
resumeClassify takes the path of the category the classifier picked.
*/
func resumeClassify(ctx context.Context, st *state, s workflowspec.Step, _ Input) (outcome, error) {
	c, answer := settings[workflowspec.ClassifySettings](s), st.answered()
	if answer.Reason != "" {
		return unsorted(s, answer.Reason), nil
	}

	label, ok := categoryLabel(categories(s), answer.Category)
	if !ok {
		return unsorted(s, "no_match"), nil
	}

	if !aidecision.SureEnough(answer.Confidence, c.MinConfidence) {
		out := unsorted(s, "low_confidence")
		out.detail["category"], out.detail["confidence"] = label, answer.Confidence

		return out, nil
	}

	if c.SaveTo != "" {
		if err := st.store(ctx, c.SaveTo, label); err != nil {
			return outcome{}, err
		}
	}

	return outcome{path: answer.Category, outcome: outcomeMatched, detail: map[string]any{
		"category": label, "confidence": answer.Confidence,
	}}, nil
}

/*
unsorted takes the none path, recording why.
*/
func unsorted(s workflowspec.Step, reason string) outcome {
	out := fixedOrNext(s, workflowspec.OutcomeNone)
	out.outcome, out.detail = "none", map[string]any{"reason": reason}

	return out
}

/*
categoryLabel returns the label of the category with the id.
*/
func categoryLabel(categories []Category, id string) (string, bool) {
	for _, c := range categories {
		if c.ID == id {
			return c.Label, true
		}
	}

	return "", false
}

/*
conversationText is what a decision reads: the whole conversation, or
the customer's newest message.
*/
func (st *state) conversationText(ctx context.Context, source string) (string, error) {
	now := st.now()
	msgs, err := st.q.ListLatestPublicMessages(ctx, models.ListLatestPublicMessagesParams{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: st.conv.ID, Now: &now, RowLimit: 100,
	})
	if err != nil {
		return "", fmt.Errorf("list messages: %w", err)
	}

	slices.Reverse(msgs)

	if source == "last_message" {
		return aidecision.LastCustomerMessage(msgs), nil
	}

	return aidecision.Transcript(st.conv.Subject, msgs), nil
}

/*
decisionSubject is the conversation a decision is about, with what it
reads.
*/
func (st *state) decisionSubject(text string) aidecision.Subject {
	conv := st.conv.ID

	return aidecision.Subject{WorkspaceID: st.conv.WorkspaceID, ConversationID: &conv, Context: text}
}
