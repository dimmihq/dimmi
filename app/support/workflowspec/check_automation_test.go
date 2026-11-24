package workflowspec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

func TestCheckAutomationSteps(t *testing.T) {
	t.Parallel()

	const fm = workflowspec.TriggerFirstMessage

	extract := func(fields string) string {
		return def(fm, `{"id":"s_extr01","kind":"extract","settings":{"fields":[`+fields+`]},
			"paths":[{"id":"p_nonexx","outcome":"none","steps":[]}]}`)
	}
	action := func(settings string) string {
		return def(fm, `{"id":"s_act001","kind":"run_action","settings":`+settings+`}`)
	}

	accepted := []struct{ name, raw string }{
		{"values of every type", extract(`{"key":"order","type":"text","save_to":"contact.custom.order"},
			{"key":"seats","type":"number","fallback":1},{"key":"due","type":"date","fallback":"2026-12-01"},
			{"key":"refund","type":"boolean","fallback":false},
			{"key":"reason","type":"choice","options":["damaged","late"],"fallback":"late"}`)},
		{"an action with inputs and outputs", action(`{"action_id":"aac_x",
			"inputs":[{"name":"email","value":"{{contact.email}}"},
				{"name":"intent","value":"{{conversation.ai.intent}}"}],
			"save":[{"output":"plan","field":"run.vars.plan"}]}`)},
	}

	for _, tc := range accepted {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, errorCodes(t, tc.raw))
		})
	}

	refused := []struct{ name, raw, code string }{
		{"no values", extract(``), workflowspec.CodeMissingSetting},
		{
			"two values of one key", extract(`{"key":"a","type":"text"},{"key":"a","type":"text"}`),
			workflowspec.CodeInvalidSetting,
		},
		{"an unknown type", extract(`{"key":"a","type":"color"}`), workflowspec.CodeInvalidSetting},
		{"a choice without choices", extract(`{"key":"a","type":"choice"}`), workflowspec.CodeInvalidSetting},
		{
			"a fallback of another type", extract(`{"key":"a","type":"number","fallback":"many"}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a fallback off the list", extract(`{"key":"a","type":"choice","options":["x"],"fallback":"y"}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a value saved to the email", extract(`{"key":"a","type":"text","save_to":"contact.email"}`),
			workflowspec.CodeInvalidSetting,
		},
		{"an action not picked", action(`{}`), workflowspec.CodeMissingSetting},
		{"an unknown variable in an input", action(`{"action_id":"aac_x",
			"inputs":[{"name":"email","value":"{{contact.mail}}"}]}`), workflowspec.CodeUnknownVariable},
		{
			"an output saved nowhere", action(`{"action_id":"aac_x","save":[{"output":"plan","field":"plan"}]}`),
			workflowspec.CodeInvalidSetting,
		},
	}

	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, errorCodes(t, tc.raw), tc.code)
		})
	}
}
