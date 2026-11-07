package ai_test

import (
	"context"
	"encoding/json"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
shopURL is the base URL of the stand-in customer API.
*/
const shopURL = "https://shop.example.test/api"

/*
shopSeal is the stand-in API's credential; no response may repeat it.
*/
const shopSeal = "sk_live_shop_secret_4f2a"

/*
actionDesk is a harness with the customer API faked.
*/
type actionDesk struct {
	*harness
	api *httpclienttest.Fake
}

/*
newActionDesk boots a harness whose agent may call actions, with every
outbound request answered by stubs and any other refused.
*/
func newActionDesk(t *testing.T, change func(*models.UpdateAgentParams)) *actionDesk {
	t.Helper()
	h := newHarness(t, func(p *models.UpdateAgentParams) {
		p.ActionsEnabled = true
		if change != nil {
			change(p)
		}
	})
	return &actionDesk{harness: h, api: httpclienttest.New(h.d.TC.App.HTTP).PreventStrayRequests()}
}

/*
connection creates the shop's HTTP connection as the owner and returns
its public id.
*/
func (a *actionDesk) connection() string {
	a.t.Helper()
	return a.d.Owner.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
		"name": "Shop", "kind": "http", "base_url": shopURL, "auth_type": "bearer", "token": shopSeal,
		"variables": testkit.JSON{"region": "eu"},
	}).AssertCreated().String("data.id")
}

/*
action creates an action on the connection with the fields given and
publishes it, returning its public id.
*/
func (a *actionDesk) action(conn string, fields testkit.JSON) string {
	a.t.Helper()
	body := testkit.JSON{"connection_id": conn}
	maps.Copy(body, fields)
	id := a.d.Owner.PostJSON("/api/v1/ai/actions", body).AssertCreated().String("data.id")
	a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/publications", testkit.JSON{}).AssertCreated()
	return id
}

/*
orderLookup is a read action bound to the customer's external id.
*/
func orderLookup() testkit.JSON {
	return testkit.JSON{
		"name": "get_order", "title": "Check order", "method": "GET",
		"description":   "Look up one of the signed-in customer's orders by number.",
		"path_template": "/orders/{input.order_id}", "query": testkit.JSON{"customer": "{contact.external_id}"},
		"input_schema": testkit.JSON{
			"type": "object", "required": []string{"order_id"},
			"properties": testkit.JSON{"order_id": testkit.JSON{"type": "string", "pattern": "^[0-9]{3,10}$"}},
		},
		"response_fields": []testkit.JSON{{"name": "status", "path": "status"}, {"name": "eta", "path": "eta"}},
		"channels":        []string{"api", "widget"},
	}
}

/*
refund is a write action that refunds an order of the signed-in
customer, with the agreement given.
*/
func refund(confirmation, approval string) testkit.JSON {
	return testkit.JSON{
		"name": "refund_order", "title": "Refund order", "method": "POST",
		"description":   "Refund one of the signed-in customer's orders.",
		"path_template": "/orders/{input.order_id}/refunds", "confirmation": confirmation, "approval": approval,
		"body_template": testkit.JSON{"amount_cents": "{input.amount_cents}", "customer": "{contact.external_id}"},
		"input_schema": testkit.JSON{
			"type": "object", "required": []string{"order_id", "amount_cents"},
			"properties": testkit.JSON{
				"order_id":     testkit.JSON{"type": "string"},
				"amount_cents": testkit.JSON{"type": "integer", "minimum": 1, "maximum": 5000},
			},
		},
		"response_fields": []testkit.JSON{{"name": "refund_id", "path": "id"}},
		"channels":        []string{"api", "widget"},
	}
}

/*
signIn gives the conversation's contact the external id user_42, as a
verified identity token would.
*/
func (a *actionDesk) signIn(conv ids.UUID) {
	a.t.Helper()
	a.operator(`UPDATE contacts SET external_id = 'user_42', kind = 'user'
		WHERE id = (SELECT contact_id FROM conversations WHERE id = $1)`, conv)
}

/*
calls lists the conversation's action calls, newest first.
*/
func (a *actionDesk) calls(conv ids.UUID) []models.AiActionCall {
	a.t.Helper()
	var out []models.AiActionCall
	a.d.TC.InWorkspace(func(ctx context.Context) error {
		var err error
		out, err = models.New(a.d.TC.App.DB).ListConversationActionCalls(ctx, models.ListConversationActionCallsParams{
			WorkspaceID: a.d.TC.Workspace.Workspace.ID, ConversationID: &conv, RowLimit: 20,
		})
		return err
	})
	return out
}

/*
onlyCall returns the conversation's one action call.
*/
func (a *actionDesk) onlyCall(conv ids.UUID) models.AiActionCall {
	a.t.Helper()
	calls := a.calls(conv)
	require.Len(a.t, calls, 1)
	return calls[0]
}

/*
publicCall is a call's public id.
*/
func publicCall(c models.AiActionCall) string {
	return models.PublicID(models.PrefixAiActionCall, c.ID)
}

/*
drain runs every queued job: executions and follow-ups.
*/
func (a *actionDesk) drain() {
	a.t.Helper()
	a.d.TC.Clock.Travel(time.Second)
	a.d.TC.Drain()
}

/*
expire runs the expiry of a call as its scheduled job would.
*/
func (a *actionDesk) expire(c models.AiActionCall) {
	a.t.Helper()
	ctx := tenancy.With(a.t.Context(), a.d.TC.Workspace.Workspace.ID.String())
	_, err := a.d.TC.App.Actions.AI.ActionKit.Expire(ctx, c.WorkspaceID, c.ID, time.Time{})
	require.NoError(a.t, err)
}

/*
sentTo lists the requests the fake API received on a path.
*/
func (a *actionDesk) sentTo(method, path string) []httpclienttest.SentRequest {
	var out []httpclienttest.SentRequest
	for _, r := range a.api.Sent() {
		if r.Method == method && r.URL.Path == path {
			out = append(out, r)
		}
	}
	return out
}

/*
toolNames lists the tools the model was offered on its nth call.
*/
func (a *actionDesk) toolNames(n int) []string {
	sent := a.llm.answer.Sent()
	if n >= len(sent) {
		return nil
	}
	var out []string
	for _, t := range sent[n].Tools {
		out = append(out, t.Name)
	}
	return out
}

/*
jsonOf decodes a JSON value for assertions.
*/
func jsonOf(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	out := map[string]any{}
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}
