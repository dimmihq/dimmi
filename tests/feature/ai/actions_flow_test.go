package ai_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
eventNames lists the timeline entries of a conversation's state.
*/
func eventNames(t *testing.T, s state) []string {
	t.Helper()
	var out []string
	for _, m := range s.messages {
		if m.Type == "event" {
			name, _ := jsonOf(t, m.Event)["name"].(string)
			out = append(out, name)
		}
	}
	return out
}

func TestAiActionReads(t *testing.T) {
	t.Parallel()

	t.Run("runs a read inline and answers from its result", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.api.Stub("GET "+shopURL+"/orders/1042*", httpclient.Response{
			Status: 200, Body: []byte(`{"status":"shipped","eta":"Thursday","internal":"do not show"}`),
		})
		a.action(a.connection(), orderLookup())
		conv, trigger := a.conversation("Where is my order 1042?")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("get_order", `{"order_id":"1042"}`)), reply("It shipped and arrives Thursday."))

		res := a.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome)
		assert.Contains(t, a.toolNames(0), "get_order")
		c := a.onlyCall(conv)
		assert.Equal(t, "succeeded", c.State)
		assert.Equal(t, "read", c.SideEffect)
		sent := a.sentTo("GET", "/api/orders/1042")
		require.Len(t, sent, 1)
		assert.Equal(t, "user_42", sent[0].URL.Query().Get("customer"))
		assert.Equal(t, "Bearer "+shopSeal, sent[0].Header.Get("Authorization"))
		assert.Equal(t, "get_order", sent[0].Header.Get("Dimmi-Action"))
		assert.Empty(t, sent[0].Header.Get("Idempotency-Key"))

		result := a.llm.answer.Sent()[1].Messages[2].Parts[0].Text
		assert.Contains(t, result, `<action_result action="get_order"`)
		assert.Contains(t, result, "shipped")
		assert.NotContains(t, result, "do not show")
		s := a.load(conv, res.RunID)
		assert.Contains(t, eventNames(t, s), "ai.action_call_completed")
		assert.Equal(t, "It shipped and arrives Thursday.", str(s.agentReplies()[0].Body))
	})

	t.Run("never offers an identity-bound action to a customer it cannot bind", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.action(a.connection(), orderLookup())
		conv, trigger := a.conversation("Where is my order 1042?")
		a.llm.answer.Script(reply("Please sign in so I can look that up."))

		a.run(conv, trigger)

		assert.NotContains(t, a.toolNames(0), "get_order")
		assert.Empty(t, a.calls(conv))
		assert.Empty(t, a.api.Sent())
	})

	t.Run("never binds identity in a messenger session that is not signed in", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, widgetAgent)
		a.action(a.connection(), orderLookup())
		v := newVisitor(t, a.d)
		_, conv := v.start(t)
		a.signIn(conv)
		a.llm.answer.Script(reply("Please sign in so I can look that up."))

		a.run(conv, firstMessage(a, conv))

		assert.NotContains(t, a.toolNames(0), "get_order")
		assert.Empty(t, a.api.Sent())
	})

	t.Run("tells the model when the arguments do not fit and sends nothing", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.action(a.connection(), orderLookup())
		conv, trigger := a.conversation("Where is my order?")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("get_order", `{"order_id":"../../admin"}`)), reply("Which order number?"))

		a.run(conv, trigger)

		assert.Contains(t, a.llm.answer.Sent()[1].Messages[2].Parts[0].Text, "invalid_arguments")
		assert.Empty(t, a.calls(conv))
		assert.Empty(t, a.api.Sent())
	})
}

func TestAiActionApprovals(t *testing.T) {
	t.Parallel()

	t.Run("waits for a teammate, runs once on approval and tells the customer", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.api.Stub("POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 200, Body: []byte(`{"id":"re_77"}`)})
		a.action(a.connection(), refund("none", "teammate"))
		conv, trigger := a.conversation("Order 1042 arrived broken, refund me 20 dollars.")
		a.signIn(conv)
		a.llm.answer.Script(
			tools(call("refund_order", `{"order_id":"1042","amount_cents":2000}`)),
			reply("I've asked the team to approve a refund of $20.00. I'll update you here."),
		)

		res := a.run(conv, trigger)

		c := a.onlyCall(conv)
		assert.Equal(t, "pending_approval", c.State)
		assert.Equal(t, "Refund order: amount cents: 2000, order id: 1042", c.Summary)
		require.NotNil(t, c.ExpiresAt)
		assert.Empty(t, a.api.Sent())
		assert.Contains(t, a.llm.answer.Sent()[1].Messages[2].Parts[0].Text, "pending_approval")
		assert.Empty(t, a.llm.answer.Sent()[1].Tools)
		s := a.load(conv, res.RunID)
		assert.Contains(t, eventNames(t, s), "ai.action_call_requested")
		require.Len(t, s.agentReplies(), 1)

		member, _ := a.d.Join(t, enums.RoleMember)
		member.PatchJSON("/api/v1/ai/action-calls/"+publicCall(c), testkit.JSON{"status": "approved"}).AssertOK().
			AssertJSONPath("data.state", "approved")
		a.llm.answer.Script(reply("Good news: your $20.00 refund (re_77) was approved and is on its way."))
		a.drain()

		c = a.onlyCall(conv)
		assert.Equal(t, "succeeded", c.State)
		require.NotNil(t, c.ReportedAt)
		require.NotNil(t, c.ReportRunID)
		sent := a.sentTo("POST", "/api/orders/1042/refunds")
		require.Len(t, sent, 1)
		assert.Equal(t, publicCall(c), sent[0].Header.Get("Idempotency-Key"))
		assert.JSONEq(t, `{"amount_cents":2000,"customer":"user_42"}`, string(sent[0].Body))
		assert.Contains(t, sentText(a.llm.answer), "<action_update")
		s = a.load(conv, *c.ReportRunID)
		replies := s.agentReplies()
		require.Len(t, replies, 2)
		assert.Contains(t, str(replies[1].Body), "Good news")
		assert.Contains(t, eventNames(t, s), "ai.action_call_approved")
		reported, _ := aiMeta(t, replies[0])["action_call"].(map[string]any)
		assert.Equal(t, publicCall(c), reported["id"])
		assert.Equal(t, "succeeded", reported["state"])
		assert.Contains(t, reported["result_summary"], "re_77")
		decider, _ := reported["decided_by"].(map[string]any)
		assert.NotEmpty(t, decider["name"])

		member.PatchJSON("/api/v1/ai/action-calls/"+publicCall(c), testkit.JSON{"status": "approved"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "action_call_not_pending")
	})

	t.Run("asks approvers in their bell and takes the request away once decided", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.api.Stub("POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 200, Body: []byte(`{"id":"re_78"}`)})
		a.action(a.connection(), refund("none", "teammate"))
		conv, trigger := a.conversation("Refund order 1042 please.")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("refund_order", `{"order_id":"1042","amount_cents":2000}`)),
			reply("I've asked the team to approve it."))
		a.run(conv, trigger)
		a.drain()

		c := a.onlyCall(conv)
		bell := a.d.Owner.GetJSON("/api/v1/notifications?filter[trigger]=ai_action_approval_requested").AssertOK()
		bell.AssertJSONPath("data.0.action_call_id", publicCall(c)).AssertJSONPath("data.0.read", false)
		assert.Contains(t, bell.String("data.0.title"), "Refund order")

		member, _ := a.d.Join(t, enums.RoleMember)
		member.PatchJSON("/api/v1/ai/action-calls/"+publicCall(c), testkit.JSON{"status": "approved"}).AssertOK()
		a.llm.answer.Script(reply("Approved."))
		a.drain()
		a.d.Owner.GetJSON("/api/v1/notifications?filter[trigger]=ai_action_approval_requested").AssertOK().
			AssertJSONPath("data.0.read", true).AssertJSONPath("data.0.resolved", "approved")
	})

	t.Run("counts calls waiting for approval on the conversation and its list row", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.action(a.connection(), refund("none", "teammate"))
		conv, trigger := a.conversation("Refund order 1042 please.")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("refund_order", `{"order_id":"1042","amount_cents":2000}`)),
			reply("I've asked the team to approve it."))
		a.run(conv, trigger)
		public := models.PublicID(models.PrefixConversation, conv)

		a.d.Owner.GetJSON("/api/v1/conversations/"+public).AssertOK().AssertJSONPath("data.ai_pending_approvals", 1)
		a.d.Owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().
			AssertJSONPath("data.0.id", public).AssertJSONPath("data.0.ai_pending_approvals", 1)

		a.d.Owner.PatchJSON("/api/v1/ai/action-calls/"+publicCall(a.onlyCall(conv)), testkit.JSON{
			"status": "declined", "reason": "not eligible",
		}).AssertOK()

		a.d.Owner.GetJSON("/api/v1/conversations/"+public).AssertOK().AssertJSONPath("data.ai_pending_approvals", 0)
	})

	t.Run("passes a decline on to the customer without the internal reason", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.action(a.connection(), refund("none", "teammate"))
		conv, trigger := a.conversation("Refund order 1042 please.")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("refund_order", `{"order_id":"1042","amount_cents":2000}`)),
			reply("I've asked the team to approve it."))
		a.run(conv, trigger)
		c := a.onlyCall(conv)

		a.d.Owner.PatchJSON("/api/v1/ai/action-calls/"+publicCall(c), testkit.JSON{
			"status": "declined", "reason": "serial refunder", "customer_message": "Refunds need the original box.",
		}).AssertOK().AssertJSONPath("data.state", "declined")
		a.llm.answer.Script(reply("The team couldn't approve this refund: they need the original box."))
		a.drain()

		c = a.onlyCall(conv)
		assert.Equal(t, "declined", c.State)
		require.NotNil(t, c.ReportedAt)
		assert.Empty(t, a.api.Sent())
		text := sentText(a.llm.answer)
		assert.Contains(t, text, "Refunds need the original box.")
		assert.NotContains(t, text, "serial refunder")
		assert.NotContains(t, a.toolNames(len(a.llm.answer.Sent())-1), "refund_order")
	})

	t.Run("hands the conversation to the team with priority when nobody decides in time", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.action(a.connection(), refund("none", "teammate"))
		conv, trigger := a.conversation("Refund order 1042 please.")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("refund_order", `{"order_id":"1042","amount_cents":2000}`)),
			reply("I've asked the team to approve it."))
		a.run(conv, trigger)
		c := a.onlyCall(conv)

		a.d.TC.Clock.Travel(61 * time.Minute)
		a.expire(c)
		a.llm.answer.Script(reply("This is taking longer than expected; a teammate will follow up here."))
		a.drain()

		c = a.onlyCall(conv)
		assert.Equal(t, "expired", c.State)
		s := a.load(conv, *c.ReportRunID)
		assert.Equal(t, "handed_off", s.conv.AiState)
		assert.Positive(t, s.conv.Priority)
		assert.Contains(t, str(s.agentReplies()[1].Body), "taking longer")
		a.d.Owner.PatchJSON("/api/v1/ai/action-calls/"+publicCall(c), testkit.JSON{"status": "approved"}).
			AssertStatus(fhttp.StatusConflict)
		assert.Empty(t, a.api.Sent())
	})

	t.Run("runs once however many teammates approve at the same moment", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.api.Stub("POST "+shopURL+"/orders/1042/refunds",
			httpclient.Response{Status: 200, Body: []byte(`{"id":"re_1"}`)})
		a.action(a.connection(), refund("none", "teammate"))
		conv, trigger := a.conversation("Refund order 1042 please.")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("refund_order", `{"order_id":"1042","amount_cents":100}`)), reply("Requested."))
		a.run(conv, trigger)
		c := a.onlyCall(conv)

		statuses := make([]int, 4)
		var wg sync.WaitGroup
		for i := range statuses {
			client, _ := a.d.Join(t, enums.RoleMember)
			wg.Go(func() {
				statuses[i] = client.PatchJSON("/api/v1/ai/action-calls/"+publicCall(c),
					testkit.JSON{"status": "approved"}).Status
			})
		}
		wg.Wait()
		a.llm.answer.Script(reply("Done: refunded."))
		a.drain()

		assert.Equal(t, 1, countOf(statuses, fhttp.StatusOK))
		assert.Equal(t, 3, countOf(statuses, fhttp.StatusConflict))
		assert.Len(t, a.sentTo("POST", "/api/orders/1042/refunds"), 1)
		assert.Equal(t, "succeeded", a.onlyCall(conv).State)
	})

	t.Run("lets only those who manage actions decide an admin approval", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.action(a.connection(), refund("none", "admin"))
		conv, trigger := a.conversation("Refund order 1042 please.")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("refund_order", `{"order_id":"1042","amount_cents":100}`)), reply("Requested."))
		a.run(conv, trigger)
		c := a.onlyCall(conv)
		member, _ := a.d.Join(t, enums.RoleMember)

		member.PatchJSON("/api/v1/ai/action-calls/"+publicCall(c), testkit.JSON{"status": "approved"}).AssertForbidden()
		a.d.Owner.PatchJSON("/api/v1/ai/action-calls/"+publicCall(c), testkit.JSON{"status": "declined"}).AssertOK()
	})

	t.Run("lists a conversation's calls for members, payloads only for admins", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, nil)
		a.action(a.connection(), refund("none", "teammate"))
		conv, trigger := a.conversation("Refund order 1042 please.")
		a.signIn(conv)
		a.llm.answer.Script(tools(call("refund_order", `{"order_id":"1042","amount_cents":100}`)), reply("Requested."))
		a.run(conv, trigger)
		member, _ := a.d.Join(t, enums.RoleMember)
		public := models.PublicID(models.PrefixConversation, conv)

		member.GetJSON("/api/v1/ai/action-calls?conversation_id="+public).AssertOK().
			AssertJSONPath("data.#", float64(1)).AssertJSONMissing("data.0.input")
		a.d.Owner.GetJSON("/api/v1/ai/action-calls?conversation_id="+public).AssertOK().
			AssertJSONPath("data.0.input.order_id", "1042")
		other := newActionDesk(t, nil)
		other.d.Owner.GetJSON("/api/v1/ai/action-calls/" + publicCall(a.onlyCall(conv))).AssertNotFound()
	})
}

/*
countOf counts a value in a list.
*/
func countOf(list []int, v int) int {
	n := 0
	for _, x := range list {
		if x == v {
			n++
		}
	}
	return n
}

/*
widgetAgent lets the agent answer in the messenger too.
*/
func widgetAgent(p *models.UpdateAgentParams) {
	p.EnabledChannels = []string{"api", "widget"}
}

/*
firstMessage is the conversation's first customer message.
*/
func firstMessage(a *actionDesk, conv ids.UUID) ids.UUID {
	a.t.Helper()
	var id ids.UUID
	a.d.TC.InWorkspace(func(ctx context.Context) error {
		for _, m := range a.messages(ctx, conv) {
			if m.AuthorType == "contact" && m.Type == "message" {
				id = m.ID
				return nil
			}
		}
		return nil
	})
	require.NotEqual(a.t, ids.UUID{}, id)
	return id
}
