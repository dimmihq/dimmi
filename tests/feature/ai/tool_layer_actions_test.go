package ai_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
bulkyAction is a read action with a long description, so several of
them pass the size under which actions stay in the fixed list.
*/
func bulkyAction(name string, agents ...string) testkit.JSON {
	a := orderLookup()
	a["name"], a["title"] = name, "Look up "+name
	a["description"] = "Look up one of the signed-in customer's records. " + strings.Repeat("It returns the "+
		"record's status, dates, totals and history as the shop keeps them. ", 10)
	if len(agents) > 0 {
		a["agents"] = agents
	}
	return a
}

func TestAiToolLayerActions(t *testing.T) {
	t.Parallel()

	t.Run("offers few small actions directly", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.withToolLayer()
		a.action(a.connection(), orderLookup())
		a.llm.answer.Script(reply("Let me check."))
		conv, trigger := a.conversation("Where is order 1042?")
		a.signIn(conv)

		a.run(conv, trigger)

		assert.Contains(t, a.toolNames(0), "get_order")
		assert.Contains(t, a.toolNames(0), "load_tools")
	})

	t.Run("loads many actions on demand and runs one through call_tool", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.withToolLayer()
		a.api.Stub("GET "+shopURL+"/orders/1042*",
			httpclient.Response{Status: 200, Body: []byte(`{"status":"shipped"}`)})
		conn := a.connection()
		for _, name := range []string{
			"get_order", "get_invoice", "get_refund", "get_return", "get_label", "get_pickup",
			"get_claim", "get_record",
		} {
			a.action(conn, bulkyAction(name))
		}
		a.llm.answer.Script(
			tools(loadCall("get_order")),
			tools(callTool("get_order", map[string]any{"order_id": "1042"})),
			reply("It shipped."),
		)
		conv, trigger := a.conversation("Where is order 1042?")
		a.signIn(conv)

		res := a.run(conv, trigger)

		assert.NotContains(t, a.toolNames(0), "get_order")
		sent := a.llm.answer.Sent()
		assert.Contains(t, sent[2].Messages[len(sent[2].Messages)-1].Parts[0].Text, "shipped")
		r := a.layerRecord(a.load(conv, res.RunID))
		assert.Contains(t, r.Suggested, "get_order")
		assert.Equal(t, []string{"get_order"}, r.Stats.Used)
	})

	t.Run("never loads or calls an action given only to Operator", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.withToolLayer()
		conn := a.connection()
		a.action(conn, bulkyAction("get_order", "operator"))
		for _, name := range []string{
			"get_invoice", "get_refund", "get_return", "get_label", "get_pickup",
			"get_claim", "get_record",
		} {
			a.action(conn, bulkyAction(name))
		}
		a.llm.answer.Script(
			tools(loadCall("get_order")),
			tools(callTool("get_order", map[string]any{"order_id": "1042"})),
			reply("I cannot look that up."),
		)
		conv, trigger := a.conversation("Where is order 1042?")
		a.signIn(conv)

		a.run(conv, trigger)

		sent := a.llm.answer.Sent()
		last := func(i int) bool { return sent[i].Messages[len(sent[i].Messages)-1].ToolError }
		assert.True(t, last(1))
		assert.True(t, last(2))
		assert.Empty(t, a.api.Sent())
	})
}
