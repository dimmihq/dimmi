package operator_test

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
shopURL is the base URL of the stand-in customer API.
*/
const shopURL = "https://shop.example.test/api"

/*
shopSeal is the stand-in API's credential.
*/
const shopSeal = "sk_test_shop_9"

/*
shop fakes the customer API and connects it as the owner, returning the
fake and the connection's public id.
*/
func (k *desk) shop() (*httpclienttest.Fake, string) {
	k.t.Helper()
	api := httpclienttest.New(k.d.TC.App.HTTP).PreventStrayRequests()
	conn := k.d.Owner.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
		"name": "Shop", "kind": "http", "base_url": shopURL, "auth_type": "bearer", "token": shopSeal,
	}).AssertCreated().String("data.id")
	return api, conn
}

/*
action creates and publishes an action, returning its public id.
*/
func (k *desk) action(conn string, fields testkit.JSON) string {
	k.t.Helper()
	body := testkit.JSON{"connection_id": conn}
	maps.Copy(body, fields)
	id := k.d.Owner.PostJSON("/api/v1/ai/actions", body).AssertCreated().String("data.id")
	k.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/publications", testkit.JSON{}).AssertCreated()
	return id
}

/*
orderLookup is a read action bound to the customer's email.
*/
func orderLookup(agents ...string) testkit.JSON {
	return testkit.JSON{
		"name": "get_order", "title": "Check order", "method": "GET", "agents": agents,
		"description":   "Look up one of the customer's orders by number.",
		"path_template": "/orders/{input.order_id}", "query": testkit.JSON{"email": "{contact.email}"},
		"input_schema": testkit.JSON{
			"type": "object", "required": []string{"order_id"},
			"properties": testkit.JSON{"order_id": testkit.JSON{"type": "string"}},
		},
		"response_fields": []testkit.JSON{{"name": "status", "path": "status"}},
		"channels":        []string{"api", "widget"},
	}
}

/*
refund is a write action that refunds one of the customer's orders.
*/
func refund(approval string) testkit.JSON {
	return testkit.JSON{
		"name": "refund_order", "title": "Refund order", "method": "POST", "agents": []string{"operator"},
		"description": "Refund one of the customer's orders.", "confirmation": "none", "approval": approval,
		"path_template": "/orders/{input.order_id}/refunds",
		"body_template": testkit.JSON{"amount_cents": "{input.amount_cents}", "email": "{contact.email}"},
		"input_schema": testkit.JSON{
			"type": "object", "required": []string{"order_id", "amount_cents"},
			"properties": testkit.JSON{
				"order_id":     testkit.JSON{"type": "string"},
				"amount_cents": testkit.JSON{"type": "integer", "minimum": 1, "maximum": 5000},
			},
		},
		"response_fields": []testkit.JSON{{"name": "refund_id", "path": "id"}},
		"channels":        []string{"widget"},
	}
}

func TestOperatorActions(t *testing.T) {
	t.Parallel()

	t.Run("offers only the actions given to Operator and looks things up at once", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		api, conn := k.shop()
		api.Stub("GET "+shopURL+"/orders/1042*", httpclient.Response{Status: 200, Body: []byte(`{"status":"shipped"}`)})
		k.action(conn, orderLookup("dimmi", "operator"))
		dimmiOnly := orderLookup()
		dimmiOnly["name"], dimmiOnly["title"] = "get_invoice", "Check invoice"
		k.action(conn, dimmiOnly)
		thread := k.thread(k.conversation("Where is order 1042?"))
		k.llm.Script(tools(call("action_get_order", map[string]any{"order_id": "1042"})), reply("It shipped."))

		res := k.ask(thread, "Where is the order?").AssertOK()

		assert.Equal(t, "true", res.String("data.message.steps.0.ok"))
		var names []string
		for _, spec := range k.llm.Sent()[0].Tools {
			names = append(names, spec.Name)
		}
		assert.Contains(t, names, "action_get_order")
		assert.NotContains(t, names, "action_get_invoice")
		require.Len(t, api.Sent(), 1)
		assert.Contains(t, api.Sent()[0].URL.String(), "email=ana%40example.com")
		assert.Contains(t, k.llm.Sent()[1].Messages[2].Parts[0].Text, "shipped")
	})

	t.Run("proposes a change as a changeset that runs once the teammate applies it", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		api, conn := k.shop()
		api.Stub("POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 200, Body: []byte(`{"id":"re_7"}`)})
		k.action(conn, refund("teammate"))
		thread := k.thread(k.conversation("Refund order 1042 please."))
		k.llm.Script(tools(call("action_refund_order", map[string]any{"order_id": "1042", "amount_cents": 2000})),
			reply("I proposed the refund."))

		res := k.ask(thread, "Refund $20").AssertOK()
		id := res.String("data.changesets.0.id")
		assert.Equal(t, "action.run", res.String("data.changesets.0.operations.0.kind"))
		assert.Empty(t, api.Sent())

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.status", "applied")
		require.Len(t, api.Sent(), 1)
		assert.Equal(t, 1, k.count("SELECT count(*) FROM ai_action_calls WHERE state = 'succeeded' "+
			"AND decided_by_member_id IS NOT NULL"))

		k.d.Owner.PostJSON("/api/v1/operator/changesets/"+id+"/reversions", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.operations.0.outcome", "skipped")
		assert.Len(t, api.Sent(), 1)
	})

	t.Run("needs an admin to apply an action only admins approve", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		api, conn := k.shop()
		k.action(conn, refund("admin"))
		member, _ := k.d.Join(t, enums.RoleMember)
		conv := k.conversation("Refund order 1042 please.")
		thread := member.PostJSON("/api/v1/operator/threads", testkit.JSON{"conversation_id": conv}).AssertOK().
			String("data.id")
		k.llm.Script(tools(call("action_refund_order", map[string]any{"order_id": "1042", "amount_cents": 2000})),
			reply("Proposed."))
		id := member.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{"body": "Refund"}).
			AssertOK().String("data.changesets.0.id")

		member.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertForbidden()
		assert.Empty(t, api.Sent())
	})

	t.Run("lets a member apply an action a teammate approves", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		api, conn := k.shop()
		api.Stub("POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 200, Body: []byte(`{"id":"re_9"}`)})
		k.action(conn, refund("teammate"))
		member, _ := k.d.Join(t, enums.RoleMember)
		conv := k.conversation("Refund order 1042 please.")
		thread := member.PostJSON("/api/v1/operator/threads", testkit.JSON{"conversation_id": conv}).AssertOK().
			String("data.id")
		k.llm.Script(tools(call("action_refund_order", map[string]any{"order_id": "1042", "amount_cents": 2000})),
			reply("Proposed."))
		id := member.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{"body": "Refund"}).
			AssertOK().String("data.changesets.0.id")

		member.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.status", "applied")
		assert.Len(t, api.Sent(), 1)
	})

	t.Run("lets an admin apply an action only admins approve", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		api, conn := k.shop()
		api.Stub("POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 200, Body: []byte(`{"id":"re_8"}`)})
		k.action(conn, refund("admin"))
		admin, _ := k.d.Join(t, enums.RoleAdmin)
		conv := k.conversation("Refund order 1042 please.")
		thread := admin.PostJSON("/api/v1/operator/threads", testkit.JSON{"conversation_id": conv}).AssertOK().
			String("data.id")
		k.llm.Script(tools(call("action_refund_order", map[string]any{"order_id": "1042", "amount_cents": 2000})),
			reply("Proposed."))
		id := admin.PostJSON("/api/v1/operator/threads/"+thread+"/messages", testkit.JSON{"body": "Refund"}).
			AssertOK().String("data.changesets.0.id")

		admin.PostJSON("/api/v1/operator/changesets/"+id+"/applications", testkit.JSON{}).AssertOK().
			AssertJSONPath("data.status", "applied")
		assert.Len(t, api.Sent(), 1)
	})

	t.Run("withholds workspace actions when the group is off", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		_, conn := k.shop()
		k.action(conn, orderLookup("operator"))
		k.d.Owner.PatchJSON("/api/v1/operator/settings", testkit.JSON{"policies": testkit.JSON{"actions": "off"}}).
			AssertOK()
		thread := k.thread(k.conversation("Where is order 1042?"))
		k.llm.Script(reply("Hi."))

		k.ask(thread, "Hi").AssertOK()

		for _, spec := range k.llm.Sent()[0].Tools {
			assert.NotEqual(t, "action_get_order", spec.Name)
		}
	})

	t.Run("keeps the agents an action is given to", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		_, conn := k.shop()
		id := k.action(conn, orderLookup("operator"))

		k.d.Owner.GetJSON("/api/v1/ai/actions/"+id).AssertOK().AssertJSONPath("data.agents.0", "operator")
		k.d.Owner.PatchJSON("/api/v1/ai/actions/"+id, testkit.JSON{"agents": []string{"robot"}}).AssertStatus(422)
	})
}
