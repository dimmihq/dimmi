/*
|--------------------------------------------------------------------------
| Sorting A Conversation By Topic
|--------------------------------------------------------------------------
|
| A classify step asks the workspace's own model which of the team's
| categories the customer's words belong to, and takes that category's
| path. The customer's words are untrusted: the classifier reads them
| as data and must answer with one of the categories, nothing else. A
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
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
maxClassifyText is the most of the customer's words the model reads.
*/
const maxClassifyText = 4000

/*
Classifier sorts a conversation into one of a step's categories.
*/
type Classifier interface {
	/*
		Classify returns the category the words belong to, an empty id
		when none fits.
	*/
	Classify(ctx context.Context, q ClassifyQuestion) (ClassifyAnswer, error)
}

/*
ClassifyQuestion is what a classifier reads: the workspace and
conversation it spends for, the team's instructions, the categories
and the customer's words.
*/
type ClassifyQuestion struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	Instructions   string
	Categories     []Category
	Text           string
}

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
ClassifyAnswer is the category picked and how sure the model was: low,
medium or high.
*/
type ClassifyAnswer struct {
	CategoryID string
	Confidence string
}

/*
runClassify waits for the classifier, which a job asks off the
conversation.
*/
func runClassify(_ context.Context, st *state, s workflowspec.Step) (outcome, error) {
	if st.e.Classifier == nil {
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

	text, err := st.customerWords(ctx, c.Source)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(text) == "" {
		return &call{answer: &callAnswer{Reason: "no_text"}}, nil
	}

	q := ClassifyQuestion{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: st.conv.ID, Instructions: c.Question, Text: text,
		Categories: categories(s),
	}

	return &call{kind: s.Kind, question: q}, nil
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
ask asks the classifier; one that cannot answer leaves the step
unsorted instead of failing the run.
*/
func (e *Engine) ask(ctx context.Context, q ClassifyQuestion) *callAnswer {
	if e.Classifier == nil {
		return &callAnswer{Reason: "ai_unavailable"}
	}

	answer, err := e.Classifier.Classify(ctx, q)
	if err != nil {
		return &callAnswer{Reason: "ai_unavailable"}
	}

	return &callAnswer{Category: answer.CategoryID, Confidence: answer.Confidence}
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

	if !sureEnough(answer.Confidence, c.MinConfidence) {
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
sureEnough reports whether a confidence meets the step's least.
*/
func sureEnough(confidence, least string) bool {
	rank := map[string]int{"low": 1, "medium": 2, "high": 3}

	return least == "" || rank[confidence] >= rank[least]
}

/*
customerWords is what the customer wrote: their newest message, or
everything they wrote with the subject, cut to the newest words.
*/
func (st *state) customerWords(ctx context.Context, source string) (string, error) {
	msgs, err := st.q.ListMessages(ctx, models.ListMessagesParams{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: st.conv.ID, RowLimit: 100,
	})
	if err != nil {
		return "", fmt.Errorf("list messages: %w", err)
	}

	var parts []string
	if source != "last_message" && st.conv.Subject != nil {
		parts = append(parts, "Subject: "+*st.conv.Subject)
	}

	var said []string
	for _, m := range msgs {
		if m.AuthorType == string(enums.AuthorTypeContact) && m.Type == string(enums.MessageTypeMessage) &&
			m.Body != nil {
			said = append(said, *m.Body)
		}
	}

	if source == "last_message" && len(said) > 0 {
		said = said[len(said)-1:]
	}

	text := strings.Join(append(parts, said...), "\n\n")
	if runes := []rune(text); len(runes) > maxClassifyText {
		text = string(runes[len(runes)-maxClassifyText:])
	}

	return text, nil
}
