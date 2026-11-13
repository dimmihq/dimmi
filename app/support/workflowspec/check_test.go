package workflowspec_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
def builds a definition of a trigger and raw step JSON.
*/
func def(trigger string, steps ...string) string {
	return fmt.Sprintf(`{"schema":1,"trigger":%q,"steps":[%s]}`, trigger, strings.Join(steps, ","))
}

/*
errorCodes returns the error codes a definition has.
*/
func errorCodes(t *testing.T, raw string) []string {
	t.Helper()

	r := workflowspec.Check(parse(t, raw))
	out := []string{}

	for _, i := range r.Errors {
		out = append(out, i.Code)
	}

	return out
}

/*
warningCodes returns the warning codes a definition has.
*/
func warningCodes(t *testing.T, raw string) []string {
	t.Helper()

	out := []string{}
	for _, i := range workflowspec.Check(parse(t, raw)).Warnings {
		out = append(out, i.Code)
	}

	return out
}

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("accepts a complete definition", func(t *testing.T) {
		t.Parallel()

		r := workflowspec.Check(parse(t, routeByTopic))
		assert.Empty(t, r.Errors)
		require.NoError(t, r.Err())
		assert.Empty(t, warningCodes(t, routeByTopic))

		claimed := `{"schema":1,"trigger":"message.customer","audience":{"rules":{"field":"contact.custom.plan",
			"op":"eq","value":"vip"}}}`
		assert.Equal(t, []string{workflowspec.WarnClaimedField}, warningCodes(t, claimed))
	})

	const fm = workflowspec.TriggerFirstMessage

	t.Run("accepts a close the loop workflow on ticket conditions", func(t *testing.T) {
		t.Parallel()

		raw := def(workflowspec.TriggerTicketCloseLoop, `{"id":"s_aaaaaa","kind":"branch","paths":[
			{"id":"p_aaaaaa","label":"Shipped","condition":{"all":[
				{"field":"ticket.issue_status_groups","op":"has","value":"done"},
				{"field":"ticket.fields.plan","op":"eq","value":"pro"}]},
			"steps":[{"id":"s_bbbbbb","kind":"message","settings":{"text":"It shipped."}},
				{"id":"s_cccccc","kind":"set_ticket_state","settings":{"state_id":"tkts_x"}}]},
			{"id":"p_bbbbbb","outcome":"else","steps":[]}]}`,
			`{"id":"s_dddddd","kind":"apply_sla","settings":{"sla_policy_id":"sla_x"}}`)
		require.NoError(t, workflowspec.Check(parse(t, raw)).Err())

		state := def(workflowspec.TriggerTicketStateChanged, `{"id":"s_aaaaaa","kind":"branch","paths":[
			{"id":"p_aaaaaa","label":"Done","condition":{"field":"trigger.to_group","op":"eq","value":"resolved"},
			"steps":[]},{"id":"p_bbbbbb","outcome":"else","steps":[]}]}`)
		require.NoError(t, workflowspec.Check(parse(t, state)).Err())
	})

	t.Run("accepts a close asking for a rating", func(t *testing.T) {
		t.Parallel()

		r := workflowspec.Check(parse(t, def(fm, `{"id":"s_aaaaaa","kind":"close","settings":{"ask_rating":true}}`)))
		require.NoError(t, r.Err())
	})

	refused := []struct {
		name, raw, code string
	}{
		{"a wrong schema version", `{"schema":2,"trigger":"reusable"}`, workflowspec.CodeSchema},
		{"an unknown trigger", `{"schema":1,"trigger":"page.visited"}`, workflowspec.CodeSchema},
		{"an unknown step kind", def(fm, `{"id":"s_aaaaaa","kind":"teleport"}`), workflowspec.CodeSchema},
		{"a malformed id", def(fm, `{"id":"step1","kind":"close"}`), workflowspec.CodeSchema},
		{
			"a repeated id", def(fm, `{"id":"s_aaaaaa","kind":"close"}`, `{"id":"s_aaaaaa","kind":"close"}`),
			workflowspec.CodeDuplicateID,
		},
		{
			"a customer message on a team trigger",
			def(workflowspec.TriggerNoteCreated, `{"id":"s_aaaaaa","kind":"message","settings":{"text":"hi"}}`),
			workflowspec.CodeStepUnavailableForTrigger,
		},
		{
			"an SLA step without a policy", def(fm, `{"id":"s_aaaaaa","kind":"apply_sla","settings":{}}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"a ticket without a type", def(fm, `{"id":"s_aaaaaa","kind":"create_ticket","settings":{}}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"a ticket field set twice", def(fm, `{"id":"s_aaaaaa","kind":"set_ticket_fields","settings":{"fields":[
			{"key":"order","value":"1"},{"key":"order","value":"2"}]}}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a ticket field with an unknown variable", def(fm, `{"id":"s_aaaaaa","kind":"create_ticket",
			"settings":{"type_id":"tktt_x","fields":[{"key":"order","value":"{{contact.shoe}}"}]}}`),
			workflowspec.CodeUnknownVariable,
		},
		{
			"an escalation without a title", def(fm, `{"id":"s_aaaaaa","kind":"escalate",
			"settings":{"type_id":"tktt_x"}}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"a tracker step without a tracker", def(fm, `{"id":"s_aaaaaa","kind":"add_to_tracker","settings":{}}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"a message on a ticket field change",
			def(workflowspec.TriggerTicketFieldChanged, `{"id":"s_aaaaaa","kind":"message","settings":{"text":"hi"}}`),
			workflowspec.CodeStepUnavailableForTrigger,
		},
		{
			"an empty message", def(fm, `{"id":"s_aaaaaa","kind":"message","settings":{"text":" "}}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"a message that is too long",
			def(fm, `{"id":"s_aaaaaa","kind":"message","settings":{"text":"`+strings.Repeat("a", 4001)+`"}}`),
			workflowspec.CodeLimitExceeded,
		},
		{
			"an unknown variable", def(fm, `{"id":"s_aaaaaa","kind":"message","settings":{"text":"{{contact.shoe}}"}}`),
			workflowspec.CodeUnknownVariable,
		},
		{
			"a delay over thirty seconds", def(fm, `{"id":"s_aaaaaa","kind":"message","settings":{"text":"a",
			"delay":"1m"}}`),
			workflowspec.CodeInvalidDuration,
		},
		{
			"an unknown setting", def(fm, `{"id":"s_aaaaaa","kind":"message","settings":{"txt":"a"}}`),
			workflowspec.CodeInvalidSetting,
		},
		{"one button", def(fm, `{"id":"s_aaaaaa","kind":"buttons","settings":{"text":"?"},
			"paths":[{"id":"p_aaaaaa","label":"A","steps":[]}]}`), workflowspec.CodeLimitExceeded},
		{
			"a button without a label", def(fm, `{"id":"s_aaaaaa","kind":"buttons","settings":{"text":"?"},
			"paths":[{"id":"p_aaaaaa","label":"A","steps":[]},{"id":"p_bbbbbb","label":"","steps":[]}]}`),
			workflowspec.CodeMissingSetting,
		},
		{"a typed path without typing", def(fm, `{"id":"s_aaaaaa","kind":"buttons","settings":{"text":"?",
			"allow_typing":false},
			"paths":[{"id":"p_aaaaaa","label":"A","steps":[]},{"id":"p_bbbbbb","label":"B","steps":[]},
			{"id":"p_cccccc","outcome":"typed","steps":[]}]}`), workflowspec.CodeInvalidSetting},
		{"a path outcome the step does not have", def(fm, `{"id":"s_aaaaaa","kind":"close",
			"paths":[{"id":"p_aaaaaa","outcome":"resolved","steps":[]}]}`), workflowspec.CodeSchema},
		{
			"a branch without else", def(fm, `{"id":"s_aaaaaa","kind":"branch","paths":[
			{"id":"p_aaaaaa","label":"A","condition":{"field":"conversation.status","op":"eq","value":"open"},
			"steps":[]}]}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"a branch path without a condition", def(fm, `{"id":"s_aaaaaa","kind":"branch","paths":[
			{"id":"p_aaaaaa","label":"A","steps":[]},{"id":"p_else00","outcome":"else","steps":[]}]}`),
			workflowspec.CodeMissingSetting,
		},
		{"a branch with settings", def(fm, `{"id":"s_aaaaaa","kind":"branch","settings":{"x":1},"paths":[
			{"id":"p_aaaaaa","label":"A","condition":{"field":"conversation.status","op":"eq","value":"open"},
			"steps":[]},
			{"id":"p_else00","outcome":"else","steps":[]}]}`), workflowspec.CodeInvalidSetting},
		{"a broken branch condition", def(fm, `{"id":"s_aaaaaa","kind":"branch","paths":[
			{"id":"p_aaaaaa","label":"A","condition":{"field":"conversation.status","op":"gt","value":1},"steps":[]},
			{"id":"p_else00","outcome":"else","steps":[]}]}`), workflowspec.CodeInvalidCondition},
		{"a malformed branch condition", def(fm, `{"id":"s_aaaaaa","kind":"branch","paths":[
			{"id":"p_aaaaaa","label":"","condition":{"all":[]},"steps":[]},
			{"id":"p_else00","outcome":"else","steps":[]}]}`), workflowspec.CodeMissingSetting},
		{"a note field on a message trigger", def(fm, `{"id":"s_aaaaaa","kind":"branch","paths":[
			{"id":"p_aaaaaa","label":"A","condition":{"field":"note.body","op":"contains","value":"x"},"steps":[]},
			{"id":"p_else00","outcome":"else","steps":[]}]}`), workflowspec.CodeFieldUnavailableTrigger},
		{
			"a page field outside an opener",
			`{"schema":1,"trigger":"conversation.first_message","audience":{"rules":{"field":"page.path","op":"eq",
			"value":"/a"}}}`,
			workflowspec.CodeFieldUnavailableTrigger,
		},
		{
			"conversation fields on an opener",
			`{"schema":1,"trigger":"conversation.opener","audience":{"rules":{"field":"conversation.status","op":"eq",
			"value":"open"}},
			"steps":[{"id":"s_aaaaaa","kind":"buttons","settings":{"text":"?"},"paths":[
			{"id":"p_aaaaaa","label":"A","steps":[]},{"id":"p_bbbbbb","label":"B","steps":[]}]}]}`,
			workflowspec.CodeFieldUnavailableTrigger,
		},
		{
			"run variables in an audience",
			`{"schema":1,"trigger":"conversation.first_message","audience":{"rules":{"field":"run.vars.x",
			"op":"exists"}}}`,
			workflowspec.CodeFieldUnavailableTrigger,
		},
		{
			"a malformed audience", `{"schema":1,"trigger":"reusable","audience":{"rules":{"all":[]}}}`,
			workflowspec.CodeInvalidCondition,
		},
		{
			"a channel the trigger cannot use", `{"schema":1,"trigger":"conversation.opener","channels":["email"],
			"steps":[
			{"id":"s_aaaaaa","kind":"buttons","settings":{"text":"?"},"paths":[
			{"id":"p_aaaaaa","label":"A","steps":[]},{"id":"p_bbbbbb","label":"B","steps":[]}]}]}`,
			workflowspec.CodeChannelUnsupported,
		},
		{"an opener that does not end in buttons", def(workflowspec.TriggerOpener,
			`{"id":"s_aaaaaa","kind":"message","settings":{"text":"hi"}}`), workflowspec.CodeOpenerShape},
		{
			"an unknown schedule mode", `{"schema":1,"trigger":"reusable","schedule":{"mode":"sometimes"}}`,
			workflowspec.CodeInvalidSetting,
		},
		{
			"a re-entry the trigger forbids", `{"schema":1,"trigger":"conversation.first_message",
			"reentry":{"mode":"every_time"}}`,
			workflowspec.CodeInvalidSetting,
		},
		{
			"a cooldown out of range", `{"schema":1,"trigger":"message.customer","reentry":{"mode":"cooldown",
			"cooldown":"30s"}}`,
			workflowspec.CodeInvalidDuration,
		},
		{"a wait without an end", def(fm, `{"id":"s_aaaaaa","kind":"wait",
			"settings":{}}`), workflowspec.CodeMissingSetting},
		{
			"a wait under a minute", def(fm, `{"id":"s_aaaaaa","kind":"wait","settings":{"for":"10s"}}`),
			workflowspec.CodeInvalidDuration,
		},
		{
			"a wait until a malformed time", def(fm, `{"id":"s_aaaaaa","kind":"wait",
			"settings":{"until":{"kind":"time_of_day","time":"9am"}}}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a wait counted in an unknown way", def(fm, `{"id":"s_aaaaaa","kind":"wait","settings":{"for":"1h",
			"count":"lunar"}}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a wait cut short by an unknown event", def(fm, `{"id":"s_aaaaaa","kind":"wait","settings":{"for":"1h",
			"interrupt_on":["rain"]}}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"an assignment without a member", def(fm, `{"id":"s_aaaaaa","kind":"assign","settings":{"mode":"member"}}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"an assignment with an unknown fallback", def(fm, `{"id":"s_aaaaaa","kind":"assign",
			"settings":{"mode":"team","team_id":"t","fallback":"x"}}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"an unknown unassignment", def(fm, `{"id":"s_aaaaaa","kind":"unassign","settings":{"what":"all"}}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a tag step without tags", def(fm, `{"id":"s_aaaaaa","kind":"tag","settings":{"tag_ids":[]}}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"an unknown priority", def(fm, `{"id":"s_aaaaaa","kind":"priority","settings":{"value":"asap"}}`),
			workflowspec.CodeInvalidSetting,
		},
		{"an empty note", def(fm, `{"id":"s_aaaaaa","kind":"note",
			"settings":{"text":""}}`), workflowspec.CodeMissingSetting},
		{
			"a contact update of the email", def(fm, `{"id":"s_aaaaaa","kind":"update_contact",
			"settings":{"set":[{"field":"contact.email","value":"x"}]}}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a contact update with nothing to set", def(fm, `{"id":"s_aaaaaa","kind":"update_contact",
			"settings":{"set":[]}}`),
			workflowspec.CodeMissingSetting,
		},
		{"a snooze without an end", def(fm, `{"id":"s_aaaaaa","kind":"snooze",
			"settings":{}}`), workflowspec.CodeMissingSetting},
		{
			"a collect saving to the external id", def(fm, `{"id":"s_aaaaaa","kind":"collect","settings":{"text":"?",
			"field":"contact.external_id","type":"text"}}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a collect of an unknown type", def(fm, `{"id":"s_aaaaaa","kind":"collect","settings":{"text":"?",
			"field":"contact.email","type":"date"}}`),
			workflowspec.CodeInvalidSetting,
		},
		{
			"a choice with one option", def(fm, `{"id":"s_aaaaaa","kind":"collect","settings":{"text":"?",
			"field":"run.vars.x","type":"choice","options":["a"]}}`),
			workflowspec.CodeMissingSetting,
		},
		{
			"too many attempts", def(fm, `{"id":"s_aaaaaa","kind":"collect","settings":{"text":"?",
			"field":"run.vars.x","type":"text","max_attempts":5}}`),
			workflowspec.CodeInvalidSetting,
		},
	}

	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, errorCodes(t, tc.raw), tc.code)
		})
	}

	t.Run("accepts office hours restrictions, waits and business counting", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, errorCodes(t, `{"schema":1,"trigger":"conversation.first_message","schedule":{"mode":"open"}}`))
		business := `{"id":"s_aaaaaa","kind":"wait","settings":{"for":"1h","count":"business"}}`
		assert.Empty(t, errorCodes(t, def(fm, business)))

		open := `{"id":"s_aaaaaa","kind":"wait","settings":{"until":{"kind":"office_hours_open"}}}`
		assert.Empty(t, errorCodes(t, def(fm, open)))
		soon := `{"schema":1,"trigger":"conversation.first_message","schedule":{"mode":"soon"}}`
		assert.Contains(t, errorCodes(t, soon), workflowspec.CodeInvalidSetting)
	})

	t.Run("checks how long a quiet trigger waits and whose reply starts it", func(t *testing.T) {
		t.Parallel()

		quiet := `{"schema":1,"trigger":"conversation.customer_quiet","trigger_settings":%s}`
		assert.Empty(t, errorCodes(t, fmt.Sprintf(quiet, `{"after":"2d","count":"business"}`)))
		assert.Contains(t, errorCodes(t, `{"schema":1,"trigger":"conversation.customer_quiet"}`),
			workflowspec.CodeInvalidDuration)
		assert.Contains(t,
			errorCodes(t, `{"schema":1,"trigger":"conversation.teammate_quiet","trigger_settings":{"after":"8d"}}`),
			workflowspec.CodeInvalidDuration)
		assert.Contains(t, errorCodes(t, fmt.Sprintf(quiet, `{"after":"1h","after_reply_from":"bot"}`)),
			workflowspec.CodeInvalidSetting)
		assert.Contains(t, errorCodes(t,
			`{"schema":1,"trigger":"conversation.status_changed","trigger_settings":{"to":["gone"]}}`),
			workflowspec.CodeInvalidSetting)
	})

	t.Run("refuses paths nested too deep", func(t *testing.T) {
		t.Parallel()

		step := `{"id":"s_leaf00","kind":"close"}`
		for i := range 13 {
			step = fmt.Sprintf(`{"id":"s_dd%04d","kind":"wait","settings":{"for":"1h"},"paths":[
				{"id":"p_dd%04d","outcome":"interrupted","steps":[%s]}]}`, i, i, step)
		}

		assert.Contains(t, errorCodes(t, def(fm, step)), workflowspec.CodeLimitExceeded)
	})

	t.Run("refuses more than 150 steps", func(t *testing.T) {
		t.Parallel()

		steps := make([]string, 151)
		for i := range steps {
			steps[i] = fmt.Sprintf(`{"id":"s_n%05d","kind":"close"}`, i)
		}

		assert.Contains(t, errorCodes(t, def(fm, steps...)), workflowspec.CodeLimitExceeded)
	})

	t.Run("warns that reusable workflows ignore their audience", func(t *testing.T) {
		t.Parallel()

		raw := `{"schema":1,"trigger":"reusable","audience":{"rules":{"field":"contact.name","op":"exists"}}}`
		assert.Contains(t, warningCodes(t, raw), workflowspec.WarnIgnoredOnReusable)
	})

	t.Run("lets an opener branch on what its own steps put in a run variable", func(t *testing.T) {
		t.Parallel()

		opener := `{"schema":1,"trigger":"conversation.opener","steps":[
			{"id":"s_menu00","kind":"buttons","settings":{"text":"?"},"paths":[
			{"id":"p_aaaaaa","label":"A","steps":[
				{"id":"s_coll00","kind":"collect","settings":{"text":"Order?","field":"run.vars.order",
			"type":"text"}},
				{"id":"s_br0000","kind":"branch","paths":[
				{"id":"p_known0","label":"Known","condition":{"field":"run.vars.order","op":"exists"},"steps":[]},
				{"id":"p_else00","outcome":"else","steps":[]}]}]},
			{"id":"p_bbbbbb","label":"B","steps":[]}]}]}`
		assert.Empty(t, errorCodes(t, opener))
	})

	t.Run("accepts an opener and every ready step", func(t *testing.T) {
		t.Parallel()

		opener := `{"schema":1,"trigger":"conversation.opener","channels":["widget"],
			"audience":{"rules":{"field":"page.path","op":"starts_with","value":"/pricing"}},"steps":[
			{"id":"s_hello0","kind":"message","settings":{"text":"Hi"}},
			{"id":"s_menu00","kind":"buttons","settings":{"text":"?","timeout":"1h"},"paths":[
			{"id":"p_aaaaaa","label":"A","steps":[
				{"id":"s_coll00","kind":"collect","settings":{"text":"Email?","field":"contact.email","type":"email"}},
				{"id":"s_upd000","kind":"update_contact","settings":{"set":[{"field":"contact.custom.plan",
			"value":"{{run.vars.p}}"}]}},
				{"id":"s_snz000","kind":"snooze","settings":{"for":"2h"}},
				{"id":"s_wait00","kind":"wait","settings":{"for":"1d","count":"calendar",
			"interrupt_on":["teammate_reply"]}},
				{"id":"s_wait01","kind":"wait","settings":{"until":{"kind":"time_of_day","time":"09:00"}}},
				{"id":"s_un0000","kind":"unassign","settings":{"what":"both"}},
				{"id":"s_pri000","kind":"priority","settings":{"value":"high"}},
				{"id":"s_untag0","kind":"untag","settings":{"tag_ids":["tag_a"]}},
				{"id":"s_rr0000","kind":"assign","settings":{"mode":"team_round_robin","team_id":"team_a",
			"fallback":"leave"}},
				{"id":"s_close0","kind":"close","settings":{"only_if_no_teammate_reply":true}}]},
			{"id":"p_bbbbbb","label":"B","steps":[]},{"id":"p_timeout","outcome":"timeout","steps":[]}]}]}`
		assert.Empty(t, errorCodes(t, opener))
	})
}
