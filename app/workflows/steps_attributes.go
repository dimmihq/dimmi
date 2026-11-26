/*
|--------------------------------------------------------------------------
| Conversation Attribute Steps
|--------------------------------------------------------------------------
|
| A step sets conversation attributes the way a teammate would, through
| the run's writer, so the timeline says the workflow set them and the
| AI never replaces them. Each value is rendered with the run's
| variables and read as its attribute holds it; an empty one clears it.
| A value that does not fit fails the step with the reason, and sets
| nothing; the run goes on.
|
*/

package workflows

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/actions/aiattributes"
	"github.com/dimmihq/dimmi/app/support/fieldvalues"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
attributeExecutors returns the executors of the conversation attribute
steps.
*/
func attributeExecutors() map[string]executor {
	return map[string]executor{workflowspec.KindSetConversationAttributes: runSetConversationAttributes}
}

/*
runSetConversationAttributes sets the step's attribute values.
*/
func runSetConversationAttributes(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	list := settings[workflowspec.SetConversationAttributesSettings](s).Attributes

	defs, err := st.q.ListAIConversationAttributes(ctx, st.conv.WorkspaceID)
	if err != nil {
		return outcome{}, err
	}

	types := map[string]string{}
	for _, d := range defs {
		types[d.Key] = d.DataType
	}

	values := map[string]any{}
	for _, v := range list {
		text, err := st.render(ctx, v.Value)
		if err != nil {
			return outcome{}, err
		}

		values[v.Key] = attributeValue(types[v.Key], strings.TrimSpace(text))
	}

	changes, err := aiattributes.Write(ctx, st.writer(), aiattributes.Writing{
		Conversation: st.conv, Values: values, By: aiattributes.SetByWorkflow,
	})
	st.subj = nil

	if err != nil && fault.KindOf(err) != fault.Internal {
		return failed(err), nil
	}

	if err != nil {
		return outcome{}, err
	}

	return changed(len(changes) > 0), nil
}

/*
attributeValue reads rendered text as an attribute of the type holds
it: nil for nothing, numbers, yes or no, and lists of choices or files.
Text that does not parse stays text, for the attribute to refuse.
*/
func attributeValue(dataType, text string) any {
	if text == "" {
		return nil
	}

	switch dataType {
	case aiattributes.TypeChoice:
		return text
	case aiattributes.TypeFile:
		return typed(fieldvalues.TypeMultipleChoice, text)
	}

	return typed(dataType, text)
}

/*
attributes records the values a step would set.
*/
func (d *dry) attributes(_ context.Context, s workflowspec.Step) outcome {
	values := map[string]any{}
	for _, v := range settings[workflowspec.SetConversationAttributesSettings](s).Attributes {
		values[v.Key] = v.Value
	}

	return d.act(s, "Would set conversation attributes", map[string]any{"attributes": values})
}
