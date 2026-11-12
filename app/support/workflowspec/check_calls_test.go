package workflowspec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

func TestCheckCallSteps(t *testing.T) {
	t.Parallel()

	const fm = workflowspec.TriggerFirstMessage

	classify := func(settings, paths string) string {
		return def(fm, `{"id":"s_sort01","kind":"classify","settings":`+settings+`,"paths":[`+paths+`]}`)
	}
	categories := `{"id":"p_billing","label":"Billing","description":"Invoices","steps":[]},
		{"id":"p_bugsss","label":"Bugs","steps":[]},{"id":"p_nonexx","outcome":"none","steps":[]}`

	accepted := []struct{ name, raw string }{
		{"a reply time with its variable", def(fm, `{"id":"s_reply1","kind":"reply_time",
			"settings":{"text":"Hi {{contact.first_name}}, we reply {{reply_time}}."}}`)},
		{"a reply time without text", def(fm, `{"id":"s_reply1","kind":"reply_time"}`)},
		{"categories", classify(`{"save_to":"run.vars.topic","min_confidence":"medium"}`, categories)},
		{"a call", def(fm, `{"id":"s_call01","kind":"run_workflow","settings":{"workflow_id":"wf_x"}}`)},
		{"a request through a connection", def(fm, `{"id":"s_http01","kind":"http_request","settings":{
			"connection_id":"acn_x","method":"POST","url":"/v1/people?email={{contact.email}}",
			"body":"{\"name\":\"{{contact.name}}\"}","timeout":"5s",
			"save":[{"path":"data.plan","field":"contact.custom.plan"}]},
			"paths":[{"id":"p_failed","outcome":"failure","steps":[]}]}`)},
	}

	for _, tc := range accepted {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, errorCodes(t, tc.raw))
		})
	}

	refused := []struct{ name, raw, code string }{
		{"an unknown variable in a reply time", def(fm, `{"id":"s_reply1","kind":"reply_time",
			"settings":{"text":"{{reply_tim}}"}}`), workflowspec.CodeUnknownVariable},
		{"categories without the none path", classify(`{}`, `{"id":"p_billing","label":"Billing","steps":[]},
			{"id":"p_bugsss","label":"Bugs","steps":[]}`), workflowspec.CodeMissingSetting},
		{"one category", classify(`{}`, `{"id":"p_billing","label":"Billing","steps":[]},
			{"id":"p_nonexx","outcome":"none","steps":[]}`), workflowspec.CodeLimitExceeded},
		{
			"two categories of one name", classify(`{}`, `{"id":"p_billing","label":"Billing","steps":[]},
			{"id":"p_bugsss","label":"billing","steps":[]},{"id":"p_nonexx","outcome":"none","steps":[]}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"categories saved to the email", classify(`{"save_to":"contact.email"}`, categories),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a call to nothing", def(fm, `{"id":"s_call01","kind":"run_workflow","settings":{}}`),
			workflowspec.CodeMissingSetting,
		},
		{"a plain http URL", def(fm, `{"id":"s_http01","kind":"http_request","settings":{"method":"GET",
			"url":"http://example.com/x"}}`), workflowspec.CodeInvalidSetting},
		{"a variable in the host", def(fm, `{"id":"s_http01","kind":"http_request","settings":{"method":"GET",
			"url":"https://{{contact.custom.host}}/x"}}`), workflowspec.CodeInvalidSetting},
		{"an absolute URL on a connection", def(fm, `{"id":"s_http01","kind":"http_request","settings":{
			"connection_id":"acn_x","method":"GET","url":"https://example.com/x"}}`), workflowspec.CodeInvalidSetting},
		{
			"a credential header", def(fm, `{"id":"s_http01","kind":"http_request","settings":{"method":"GET",
			"url":"https://example.com/x","headers":[{"name":"Authorization","value":"Bearer x"}]}}`),
			workflowspec.CodeInvalidSetting,
		},
		{"a long timeout", def(fm, `{"id":"s_http01","kind":"http_request","settings":{"method":"GET",
			"url":"https://example.com/x","timeout":"30s"}}`), workflowspec.CodeInvalidDuration},
		{"a body on a GET", def(fm, `{"id":"s_http01","kind":"http_request","settings":{"method":"GET",
			"url":"https://example.com/x","body":"{}"}}`), workflowspec.CodeInvalidSetting},
		{
			"a saved path with a modifier", def(fm, `{"id":"s_http01","kind":"http_request","settings":{
			"method":"GET","url":"https://example.com/x","save":[{"path":"data|@this","field":"run.vars.x"}]}}`),
			workflowspec.CodeInvalidSetting,
		},
	}

	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, errorCodes(t, tc.raw), tc.code)
		})
	}

	t.Run("warns about a request without a connection", func(t *testing.T) {
		t.Parallel()

		raw := def(fm, `{"id":"s_http01","kind":"http_request","settings":{"method":"GET",
			"url":"https://example.com/people/{{contact.custom.id}}"}}`)
		assert.Empty(t, errorCodes(t, raw))
		assert.Equal(t, []string{workflowspec.WarnHTTPRawURL}, warningCodes(t, raw))
	})
}
