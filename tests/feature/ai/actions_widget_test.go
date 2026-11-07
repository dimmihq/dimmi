package ai_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mcp"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
signedInVisitor is a messenger session signed in with a verified
identity token as the external id, before it says anything.
*/
func signedInVisitor(a *actionDesk, externalID string) visitor {
	a.t.Helper()
	v := newVisitor(a.t, a.d)
	a.operator(`UPDATE widget_sessions SET kind = 'identified', verified_at = now(), expires_at = '2030-01-01'`)
	a.operator(`UPDATE contacts SET external_id = $1, kind = 'user'
		WHERE id IN (SELECT contact_id FROM widget_sessions)`, externalID)
	return v
}

/*
proposal is a refund the agent proposed to a signed-in visitor: the
visitor, the conversation's public and internal ids and the call.
*/
type proposal struct {
	v      visitor
	public string
	conv   ids.UUID
	call   string
}

/*
path is the visitor's address of the proposed call.
*/
func (p proposal) path() string {
	return "/widget/v1/conversations/" + p.public + "/action-calls/" + p.call
}

/*
proposeRefund has a signed-in visitor ask for a refund the agent
proposes for their confirmation.
*/
func proposeRefund(t *testing.T, a *actionDesk) proposal {
	t.Helper()
	a.action(a.connection(), refund("customer", "none"))
	v := signedInVisitor(a, "user_42")
	public, conv := v.start(t)
	a.llm.answer.Script(tools(call("refund_order", `{"order_id":"1042","amount_cents":2000}`)),
		reply("Please check the refund below and confirm."))
	a.run(conv, firstMessage(a, conv))
	return proposal{v: v, public: public, conv: conv, call: publicCall(a.onlyCall(conv))}
}

func TestAiActionConfirmations(t *testing.T) {
	t.Parallel()

	t.Run("runs a write once the customer confirms the card", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, widgetAgent)
		a.api.Stub("POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 200, Body: []byte(`{"id":"re_9"}`)})
		p := proposeRefund(t, a)
		v, convID, callID := p.v, p.conv, p.call

		c := a.onlyCall(convID)
		assert.Equal(t, "awaiting_confirmation", c.State)
		require.NotNil(t, c.MessageID)
		var card map[string]any
		a.d.TC.InWorkspace(func(ctx context.Context) error {
			for _, m := range a.messages(ctx, convID) {
				if m.ID == *c.MessageID {
					card, _ = aiMeta(t, m)["confirmation"].(map[string]any)
				}
			}
			return nil
		})
		require.NotNil(t, card)
		assert.Equal(t, callID, card["action_call_id"])
		assert.Equal(t, "Refund order: amount cents: 2000, order id: 1042", card["text"])

		path := p.path()
		v.client().GetJSON(path).AssertOK().AssertJSONPath("data.state", "awaiting_confirmation")
		v.client().PatchJSON(path, testkit.JSON{"status": "confirmed"}).AssertOK().
			AssertJSONPath("data.state", "approved")
		v.client().PatchJSON(path, testkit.JSON{"status": "confirmed"}).AssertOK()
		a.llm.answer.Script(reply("Done: your $20.00 refund is on its way."))
		a.drain()

		c = a.onlyCall(convID)
		assert.Equal(t, "succeeded", c.State)
		assert.Len(t, a.sentTo("POST", "/api/orders/1042/refunds"), 1)
		v.client().GetJSON(path).AssertOK().AssertJSONPath("data.state", "succeeded")
		s := a.load(convID, *c.ReportRunID)
		card, _ = aiMeta(t, s.agentReplies()[0])["confirmation"].(map[string]any)
		assert.Equal(t, "succeeded", card["status"])
		assert.Contains(t, str(s.agentReplies()[1].Body), "on its way")
	})

	t.Run("sends nothing once the customer cancels", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, widgetAgent)
		p := proposeRefund(t, a)
		v, convID, path := p.v, p.conv, p.path()

		v.client().PatchJSON(path, testkit.JSON{"status": "canceled"}).AssertOK().
			AssertJSONPath("data.state", "canceled")
		v.client().PatchJSON(path, testkit.JSON{"status": "canceled"}).AssertOK()
		v.client().PatchJSON(path, testkit.JSON{"status": "confirmed"}).AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "action_call_not_pending")
		a.drain()

		assert.Equal(t, "canceled", a.onlyCall(convID).State)
		assert.Empty(t, a.api.Sent())
	})

	t.Run("lets no other visitor see or confirm the card", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, widgetAgent)
		p := proposeRefund(t, a)
		stranger := newVisitor(t, a.d)
		convID, path := p.conv, p.path()

		stranger.client().GetJSON(path).AssertNotFound()
		stranger.client().PatchJSON(path, testkit.JSON{"status": "confirmed"}).AssertNotFound()
		assert.Equal(t, "awaiting_confirmation", a.onlyCall(convID).State)
	})
}

/*
shopMCP is an in-process MCP server with an order lookup tool, reached
at the fake URL with its bearer token.
*/
func shopMCP(t *testing.T, a *actionDesk) {
	t.Helper()
	s := mcp.New(mcp.ServerOptions{Name: "shop", Version: "1.0.0", Errors: fhttp.Errors{}})
	type lookup struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
	}
	type order struct {
		Status string `json:"status"`
		Owner  string `json:"owner"`
	}
	require.NoError(t, mcp.Tool(s, mcp.ToolDef{Name: "lookup_order", Description: "Look up an order by number."},
		func(_ context.Context, in lookup) (order, error) {
			return order{Status: "packed " + in.OrderID, Owner: in.UserID}, nil
		}))
	guard := auth.GuardFunc(func(r *fhttp.Request) (auth.Principal, error) {
		if r.Header.Get("Authorization") != "Bearer "+shopSeal {
			return nil, auth.ErrInvalidToken
		}
		return &auth.Identity{Via: "token"}, nil
	})
	a.api.Serve("https://mcp.shop.example.test/mcp*", s.HTTPHandler(mcp.Options{Auth: guard}))
}

func TestAiActionMCP(t *testing.T) {
	t.Parallel()

	t.Run("syncs a server's tools as drafts and calls one with bound arguments", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		shopMCP(t, a)
		conn := a.d.Owner.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
			"name": "Shop MCP", "kind": "mcp", "base_url": "https://mcp.shop.example.test/mcp",
			"auth_type": "bearer", "token": shopSeal,
		}).AssertCreated().String("data.id")
		a.d.Owner.PostJSON("/api/v1/ai/action-connections/"+conn+"/tests", testkit.JSON{}).AssertCreated().
			AssertJSONPath("data.status", "ok").AssertJSONPath("data.tools_count", float64(1))

		sync := a.d.Owner.PostJSON("/api/v1/ai/action-connections/"+conn+"/tool-syncs", nil).AssertCreated()
		sync.AssertJSONPath("data.created.0.name", "lookup_order").
			AssertJSONPath("data.created.0.side_effect", "write").AssertJSONPath("data.created.0.status", "draft")
		id := sync.String("data.created.0.id")
		a.d.Owner.PostJSON("/api/v1/ai/action-connections/"+conn+"/tool-syncs", nil).AssertCreated().
			AssertJSONPath("data.existing", float64(1))

		a.d.Owner.PatchJSON("/api/v1/ai/actions/"+id, testkit.JSON{
			"side_effect": "read", "channels": []string{"api"},
			"bound_arguments": testkit.JSON{"user_id": "{contact.external_id}"},
		}).AssertOK().AssertJSONPath("data.confirmation", "none")
		a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/publications", testkit.JSON{}).AssertCreated()

		conv, trigger := a.conversation("Where is order 77?")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("lookup_order", `{"order_id":"77"}`)), reply("It is packed."))
		res := a.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome)
		c := a.onlyCall(conv)
		assert.Equal(t, "succeeded", c.State)
		assert.JSONEq(t, `{"status":"packed 77","owner":"user_42"}`, string(c.Result))
		for _, tool := range a.llm.answer.Sent()[0].Tools {
			if tool.Name == "lookup_order" {
				assert.NotContains(t, string(tool.Schema), "user_id")
			}
		}
	})
}
