package operator_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
bulkyLookup is a read action with a long description, so a few of them
pass the size under which actions stay in the fixed list.
*/
func bulkyLookup(name string) testkit.JSON {
	a := orderLookup("operator")
	a["name"], a["title"] = name, "Look up "+name
	a["description"] = "Look up one record in the shop's systems. " + strings.Repeat("It returns the record's "+
		"status, owner, dates, totals and history as the shop keeps them. ", 10)
	return a
}

/*
others are enough further lookups to pass the inline size.
*/
func others() []string {
	return []string{
		"get_invoice", "get_refund", "get_return", "get_customer_record", "get_label", "get_pickup",
		"get_claim",
	}
}

func TestOperatorToolLayerActions(t *testing.T) {
	t.Parallel()

	t.Run("offers few small actions directly in the fixed list", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		_, conn := k.shop()
		k.action(conn, orderLookup("operator"))
		thread := k.thread(k.conversation("Where is order 1042?"))
		k.llm.Script(reply("Let me see."))

		k.ask(thread, "Where is the order?").AssertOK()

		assert.Contains(t, offered(k.llm.Sent()[0]), "action_get_order")
		assert.Contains(t, k.layerRecord().Fixed, "action_get_order")
	})

	t.Run("moves many actions behind the layer and suggests them", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		api, conn := k.shop()
		api.Stub("GET "+shopURL+"/orders/1042*", httpclient.Response{Status: 200, Body: []byte(`{"status":"shipped"}`)})
		for _, name := range append([]string{"get_order"}, others()...) {
			k.action(conn, bulkyLookup(name))
		}
		thread := k.thread(k.conversation("Where is order 1042?"))
		k.llm.Script(
			tools(loadTools("action_get_order")),
			tools(callTool("action_get_order", map[string]any{"order_id": "1042"})),
			reply("It shipped."),
		)

		k.ask(thread, "Look up the invoice and order 1042").AssertOK()

		sent := k.llm.Sent()
		assert.NotContains(t, offered(sent[0]), "action_get_order")
		assert.Contains(t, sent[0].System[0].Text, "- actions: the workspace's own actions on its systems")
		result := lastResult(sent[2]).Parts[0].Text
		assert.Contains(t, result, `<action_result action="get_order"`)
		assert.Contains(t, result, "shipped")
		r := k.layerRecord()
		assert.Contains(t, r.Suggested, "action_get_order")
		assert.Equal(t, []string{"action_get_order"}, r.Stats.Used)
	})

	t.Run("never loads or calls an action not given to Operator", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		api, conn := k.shop()
		for _, name := range others() {
			k.action(conn, bulkyLookup(name))
		}
		dimmiOnly := bulkyLookup("get_order")
		dimmiOnly["agents"] = []string{"dimmi"}
		k.action(conn, dimmiOnly)
		thread := k.thread(k.conversation("Where is order 1042?"))
		k.llm.Script(
			tools(loadTools("action_get_order")),
			tools(callTool("action_get_order", map[string]any{"order_id": "1042"})),
			reply("I cannot look that up."),
		)

		k.ask(thread, "Look up order 1042").AssertOK()

		sent := k.llm.Sent()
		assert.True(t, lastResult(sent[1]).ToolError)
		assert.True(t, lastResult(sent[2]).ToolError)
		assert.Empty(t, api.Sent())
		assert.NotContains(t, k.layerRecord().Suggested, "action_get_order")
	})

	t.Run("turns a change through call_tool into a changeset that does not run", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		api, conn := k.shop()
		k.action(conn, refund("admin"))
		thread := k.thread(k.conversation("Refund order 1042 please."))
		k.llm.Script(
			tools(callTool("action_refund_order", map[string]any{"order_id": "1042", "amount_cents": 2000})),
			reply("I proposed the refund."),
		)

		res := k.ask(thread, "Refund $20").AssertOK()

		assert.Equal(t, "action.run", res.String("data.changesets.0.operations.0.kind"))
		assert.Equal(t, "pending", res.String("data.changesets.0.status"))
		assert.Empty(t, api.Sent())
	})

	t.Run("drops a deleted action from the suggestions", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.withToolLayer()
		_, conn := k.shop()
		id := k.action(conn, bulkyLookup("get_order"))
		for _, name := range others() {
			k.action(conn, bulkyLookup(name))
		}
		conv := k.conversation("Where is order 1042?")
		k.llm.Script(reply("Checked."), reply("Checked again."))

		k.ask(k.thread(conv), "Look up order 1042").AssertOK()
		require.Contains(t, k.layerRecord().Suggested, "action_get_order")
		k.d.Owner.Delete("/api/v1/ai/actions/" + id).AssertStatus(204)
		k.d.TC.Clock.Travel(time.Minute)
		k.ask(k.thread(conv), "Look up order 1042").AssertOK()

		assert.NotContains(t, k.layerRecord().Suggested, "action_get_order")
	})
}
