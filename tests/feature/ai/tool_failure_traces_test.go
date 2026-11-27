package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/tests"
)

/*
The tracker counts a span as failed when its status is set and is not
"ok", and nothing else: the Tools table's Errors column
(apps/server/src/services/span.rs reduce_latency_groups,
`status.is_some_and(|s| s != "ok")`), the Errors figure and each run's
errors (agent_summary and agent_traces, `status IS NOT NULL AND status
<> 'ok'`), and the red span in the trace view
(packages/ui/src/components/waterfall/waterfall.tsx, `span.status &&
span.status !== 'ok'`). No rule reads error.type; it is the attribute a
person filters and reads in the span's details.
*/
func TestToolFailureTraces(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		stub    func(a *actionDesk)
		action  func() map[string]any
		call    fake.Step
		status  string // An empty status is a call the tracker does not count as failed.
		errType string
		outcome string
	}{
		{
			name: "an action that answers 500 is unavailable",
			stub: func(a *actionDesk) {
				a.api.Stub("GET "+shopURL+"/orders/1042*", httpclient.Response{Status: 500, Body: []byte(`{}`)})
			},
			action: orderLookup, call: tools(call("get_order", `{"order_id":"1042"}`)),
			status: "unavailable", errType: "unavailable",
		},
		{
			name: "an action that gets no answer is unavailable",
			stub: func(*actionDesk) {}, action: orderLookup, call: tools(call("get_order", `{"order_id":"1042"}`)),
			status: "unavailable", errType: "unavailable",
		},
		{
			name: "an action its API refuses to authenticate failed authentication",
			stub: func(a *actionDesk) {
				a.api.Stub("GET "+shopURL+"/orders/1042*", httpclient.Response{Status: 401, Body: []byte(`{}`)})
			},
			action: orderLookup, call: tools(call("get_order", `{"order_id":"1042"}`)),
			status: "unauthenticated", errType: "auth_failed",
		},
		{
			name: "an action that is rate limited",
			stub: func(a *actionDesk) {
				a.api.Stub("GET "+shopURL+"/orders/1042*", httpclient.Response{Status: 429, Body: []byte(`{}`)})
			},
			action: orderLookup, call: tools(call("get_order", `{"order_id":"1042"}`)),
			status: "resource_exhausted", errType: "rate_limited",
		},
		{
			name: "arguments that break the action's schema",
			stub: func(*actionDesk) {}, action: orderLookup, call: tools(call("get_order", `{"order_id":"x"}`)),
			status: "invalid_argument", errType: "invalid_arguments",
		},
		{
			name: "a tool the agent does not have",
			stub: func(*actionDesk) {}, action: orderLookup, call: tools(call("delete_everything", `{}`)),
			status: "not_found", errType: "unknown_tool",
		},
		{
			name:    "a write that waits for a teammate answers no, not a failure",
			stub:    func(*actionDesk) {},
			action:  func() map[string]any { return refund("none", "teammate") },
			call:    tools(call("refund_order", `{"order_id":"1042","amount_cents":500}`)),
			outcome: "pending_approval",
		},
		{
			name: "an action that succeeds",
			stub: func(a *actionDesk) {
				a.api.Stub("GET "+shopURL+"/orders/1042*",
					httpclient.Response{Status: 200, Body: []byte(`{"status":"shipped"}`)})
			},
			action: orderLookup, call: tools(call("get_order", `{"order_id":"1042"}`)),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tracker := tests.NewTracker(t)
			a := newActionDesk(t, nil, tracker.Option())
			tc.stub(a)
			a.action(a.connection(), tc.action())
			conv, trigger := a.conversation("Where is my order 1042?")
			a.signIn(conv)
			a.llm.answer.Script(tc.call, reply("Let me check on that for you."))

			a.run(conv, trigger)
			spans := a.d.TC.TrackedSpans(tracker)

			var tool, run tests.TrackedSpan
			for _, s := range spans {
				switch {
				case s.Kind() == "tool" && s.Attr("gen_ai.tool.name") != "search_knowledge":
					tool = s
				case s.Kind() == "agent":
					run = s
				}
			}
			require.NotEmpty(t, tool.SpanID, "the call has a tool span")
			assert.Equal(t, tc.status, tool.Status)
			assert.Equal(t, tc.errType, tool.Attr("error.type"))
			assert.Equal(t, tc.outcome, tool.Attr("tool.outcome"))
			assert.NotContains(t, tool.Attr("error.type"), shopSeal)
			assert.Empty(t, run.Status, "the agent recovered, so its run did not fail")
		})
	}
}
