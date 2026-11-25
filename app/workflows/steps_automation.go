/*
|--------------------------------------------------------------------------
| Extracting Values And Running Actions
|--------------------------------------------------------------------------
|
| An extract step asks the AI decision step for typed values in the
| conversation and saves them where the team said, a fallback standing
| in for a value the models were unsure of. A value with no fallback
| that stays unknown, or no model to ask, takes the none path; without
| one the run goes on with what it found.
|
| A run action step calls one of the workspace's actions, the ones
| agents call, with inputs filled from the run's variables, and saves
| the outputs the team picked. A call that fails takes the failure path,
| or goes on when the step has none.
|
| Both leave the database, so they only wait: a job makes the call off
| the conversation and outside any transaction, exactly once per wait.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
ActionRunner runs the workspace's actions for automations.
*/
type ActionRunner interface {
	/*
		RunForAutomation runs an action's published version.
	*/
	RunForAutomation(ctx context.Context, in actionkit.AutomationCall) (actionkit.AutomationResult, error)
}

/*
automationExecutors returns the executors of these steps.
*/
func automationExecutors() map[string]executor {
	return map[string]executor{workflowspec.KindExtract: runExtract, workflowspec.KindRunAction: runRunAction}
}

/*
automationResumers returns the resumers of these steps.
*/
func automationResumers() map[string]resumer {
	return map[string]resumer{workflowspec.KindExtract: resumeExtract, workflowspec.KindRunAction: resumeHTTPRequest}
}

/*
runExtract waits for the decision step, which a job asks off the
conversation.
*/
func runExtract(_ context.Context, st *state, s workflowspec.Step) (outcome, error) {
	if st.e.Decisions == nil {
		return extracted(s, nil, nil, "ai_unavailable"), nil
	}

	return callingOut(false), nil
}

/*
prepareExtraction reads the conversation and the fields, an answer
instead when there is nothing to read.
*/
func (st *state) prepareExtraction(ctx context.Context, s workflowspec.Step) (*call, error) {
	e := settings[workflowspec.ExtractSettings](s)

	text, err := st.conversationText(ctx, e.Source)
	if err != nil || text == "" {
		return &call{answer: &callAnswer{Reason: "no_text"}}, err
	}

	fields := make([]aidecision.Field, 0, len(e.Fields))
	for _, f := range e.Fields {
		fields = append(fields, aidecision.Field{
			Key: f.Key, Label: f.Label, Description: f.Description, Type: f.Type, Options: f.Options,
		})
	}

	return &call{kind: s.Kind, extraction: aidecision.Extraction{
		Subject: st.decisionSubject(text), Instructions: e.Instructions, Fields: fields,
	}}, nil
}

/*
extract asks the decision step for the values; one that cannot answer
leaves every value unknown.
*/
func (e *Engine) extract(ctx context.Context, x aidecision.Extraction) *callAnswer {
	if e.Decisions == nil {
		return &callAnswer{Reason: "ai_unavailable"}
	}

	got, err := e.Decisions.Extract(ctx, x)
	if err != nil {
		return &callAnswer{Reason: "ai_unavailable"}
	}

	return &callAnswer{Values: got.Values}
}

/*
resumeExtract saves what the decision step found, and the fallbacks of
what it did not.
*/
func resumeExtract(ctx context.Context, st *state, s workflowspec.Step, _ Input) (outcome, error) {
	e, answer := settings[workflowspec.ExtractSettings](s), st.answered()

	var saved, unsure []string

	for _, f := range e.Fields {
		value, ok := answer.Values[f.Key]
		if !ok || !workflowspec.FitsValue(f, mustJSON(value)) {
			value, ok = fallbackOf(f)
		}

		if !ok {
			unsure = append(unsure, f.Key)
			continue
		}

		if err := st.store(ctx, f.Target(), value); err != nil {
			return outcome{}, err
		}

		saved = append(saved, f.Target())
	}

	return extracted(s, saved, unsure, answer.Reason), nil
}

/*
extracted is the outcome of an extract step: on with what it saved,
or down the none path when a value stayed unknown.
*/
func extracted(s workflowspec.Step, saved, unsure []string, reason string) outcome {
	if reason != "" && len(saved) == 0 && len(unsure) == 0 {
		for _, f := range settings[workflowspec.ExtractSettings](s).Fields {
			if _, ok := fallbackOf(f); !ok {
				unsure = append(unsure, f.Key)
			}
		}
	}

	detail := map[string]any{"saved": orNone(saved), "unsure": orNone(unsure)}
	if reason != "" {
		detail["reason"] = reason
	}

	if len(unsure) > 0 {
		out := fixedOrNext(s, workflowspec.OutcomeNone)
		out.outcome, out.detail = "none", detail

		return out
	}

	return outcome{outcome: "extracted", acted: len(saved) > 0, detail: detail}
}

/*
fallbackOf reads a field's fallback, false without one.
*/
func fallbackOf(f workflowspec.ExtractField) (any, bool) {
	if len(f.Fallback) == 0 || string(f.Fallback) == "null" {
		return nil, false
	}

	var v any
	if json.Unmarshal(f.Fallback, &v) != nil {
		return nil, false
	}

	return v, true
}

/*
mustJSON writes a value as JSON.
*/
func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)

	return raw
}

/*
orNone is a list, empty rather than nil.
*/
func orNone(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}

/*
runRunAction waits for the action, which a job runs off the
conversation.
*/
func runRunAction(_ context.Context, st *state, s workflowspec.Step) (outcome, error) {
	if st.e.Actions == nil {
		return requestFailed(s, "unavailable", 0), nil
	}

	return callingOut(true), nil
}

/*
prepareAction renders the action's inputs and reads who the customer
is, an answer instead when the action cannot be named.
*/
func (st *state) prepareAction(ctx context.Context, s workflowspec.Step) (*call, error) {
	a := settings[workflowspec.RunActionSettings](s)

	id, ok := st.decode(models.PrefixAiAction, a.ActionID)
	if !ok {
		return &call{answer: &callAnswer{Error: outcomeMissingRef}}, nil
	}

	inputs := map[string]string{}
	for _, in := range a.Inputs {
		value, err := st.renderEscaped(ctx, in.Value, func(v string) string { return v })
		if err != nil {
			return nil, err
		}

		inputs[in.Name] = value
	}

	contact, err := st.q.FindContact(ctx, models.FindContactParams{
		WorkspaceID: st.conv.WorkspaceID, ID: st.conv.ContactID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, err
	}

	conv := st.conv.ID
	in := actionkit.AutomationCall{
		WorkspaceID: st.conv.WorkspaceID, ActionID: id, ConversationID: &conv, Inputs: inputs,
		By: "Workflow",
	}

	if err == nil {
		in.ContactID, in.Identity = &contact.ID, actionkit.AutomationIdentity(contact)
	}

	return &call{kind: s.Kind, action: &in, outputs: a.Save}, nil
}

/*
runAction runs the action with no transaction open and picks the saved
outputs out of its result.
*/
func (e *Engine) runAction(ctx context.Context, c *call) *callAnswer {
	if e.Actions == nil || c.action == nil {
		return &callAnswer{Error: "unavailable"}
	}

	ctx = tenancy.With(ctx, c.action.WorkspaceID.String())

	res, err := e.Actions.RunForAutomation(ctx, *c.action)
	if err != nil {
		return &callAnswer{Error: "unavailable"}
	}

	if !res.Succeeded {
		return &callAnswer{Error: orCode(res.ErrorCode), Status: res.HTTPStatus}
	}

	return &callAnswer{Status: res.HTTPStatus, Saves: outputValues(c.outputs, res.Result)}
}

/*
outputValues picks the saved outputs out of an action's result: a
response field by its name, or a dotted path into the result.
*/
func outputValues(saves []workflowspec.ActionSave, result []byte) []savedValue {
	var out []savedValue

	if !json.Valid(result) {
		return out
	}

	for _, sv := range saves {
		if value, found := responseValue(result, sv.Output); found {
			out = append(out, savedValue{Field: sv.Field, Value: value})
		}
	}

	return out
}

/*
orCode is an error code, failed when the action gave none.
*/
func orCode(code string) string {
	if code == "" {
		return "failed"
	}

	return code
}
