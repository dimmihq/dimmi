package ai_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/mcp"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/agentactions"
)

/*
signedInVisitor is a messenger session signed in the way a host app does
it: the owner creates an identity secret, and the visitor's browser
starts its session with a token signed by it naming the external id.
*/
func signedInVisitor(a *actionDesk, externalID string) visitor {
	a.t.Helper()
	created := a.d.Owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{"algorithm": "HS256"}).AssertCreated()
	key, err := jwt.NewKey(created.String("data.kid"), jwt.HS256, []byte(created.String("data.secret")))
	require.NoError(a.t, err)
	now := a.d.TC.Clock.Now()
	token, err := jwt.Sign(jwt.Claims{Subject: externalID, IssuedAt: now, Expiry: now.Add(10 * time.Minute)}, key)
	require.NoError(a.t, err)
	res := a.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{
		"app_id": appOf(a.t, a.d), "user_jwt": token,
	}).AssertCreated().AssertJSONPath("data.kind", "identified")
	return visitor{d: a.d, token: res.String("data.token")}
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

	t.Run("forgets what a call held once its customer is erased", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, widgetAgent)
		p := proposeRefund(t, a)
		c := a.onlyCall(p.conv)
		require.NotNil(t, c.ContactID)
		assert.Contains(t, string(c.Input), "1042")

		eraser := &agentactions.CallEraser{DB: a.d.TC.App.DB, Clock: a.d.TC.Clock}
		assert.Equal(t, "ai_action_calls", eraser.Name())
		require.NoError(t, eraser.Erase(t.Context(), a.d.TC.Workspace.Workspace.ID, *c.ContactID))
		require.NoError(t, eraser.Erase(t.Context(), a.d.TC.Workspace.Workspace.ID, *c.ContactID))

		c = a.onlyCall(p.conv)
		assert.JSONEq(t, `{}`, string(c.Input))
		assert.JSONEq(t, `{}`, string(c.Bindings))
		assert.Empty(t, c.Summary)
		assert.Nil(t, c.Result)
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
		a.d.Owner.PostJSON("/api/v1/ai/actions/"+id+"/tests", testkit.JSON{
			"input": testkit.JSON{"order_id": "77"}, "contact": testkit.JSON{"external_id": "user_9"}, "mode": "live",
		}).AssertCreated().AssertJSONPath("data.status", "succeeded")

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
