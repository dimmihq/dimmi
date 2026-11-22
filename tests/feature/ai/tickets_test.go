package ai_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ticketType returns the public id of the ticket type named.
*/
func (h *harness) ticketType(name string) string {
	h.t.Helper()
	res := h.d.Owner.GetJSON("/api/v1/ticket-types").AssertOK()
	for i := 0; res.String("data."+strconv.Itoa(i)+".id") != ""; i++ {
		if res.String("data."+strconv.Itoa(i)+".name") == name {
			return res.String("data." + strconv.Itoa(i) + ".id")
		}
	}
	h.t.Fatalf("no ticket type %s", name)
	return ""
}

/*
openTicket is a scripted open_ticket call.
*/
func openTicket(args map[string]any) contracts.ToolCall {
	data, _ := json.Marshal(args)
	return call("open_ticket", string(data))
}

/*
TestAiOpensTickets lets the agent open a ticket of any type the
workspace offers it: it collects required fields, confirms, and tells the
customer the reference.
*/
func TestAiOpensTickets(t *testing.T) {
	t.Parallel()

	guided := func(p *models.UpdateAgentParams) {
		p.Guidance = "When a customer reports a bug, open a Bug report ticket for them."
	}

	t.Run("opens a confirmed ticket and writes its reference into the reply", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, guided)
		h.llm.answer.Script(tools(openTicket(map[string]any{
			"type": "Bug report", "title": "CSV export crashes", "fields": map[string]any{"steps": "Click export"},
			"confirmed": true,
		})), reply("Done: your ticket is {ticket_reference}."))

		conv, trigger := h.conversation("Export crashes, yes please open it")
		res := h.run(conv, trigger)

		s := h.load(conv, res.RunID)
		replies := s.agentReplies()
		require.NotEmpty(t, replies)
		body := str(replies[len(replies)-1].Body)
		assert.Contains(t, body, "Done: your ticket is T-")
		assert.NotContains(t, body, "{ticket_reference}")

		public := models.PublicID(models.PrefixConversation, conv)
		tickets := h.d.Owner.GetJSON("/api/v1/conversations/" + public + "/tickets").AssertOK()
		tickets.AssertJSONPath("data.ticket.title", "CSV export crashes")
		assert.Contains(t, body, "Done: your ticket is "+tickets.String("data.ticket.reference")+".")
	})

	t.Run("keeps the conversation when the reply says the ticket's team will follow up", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, guided)
		h.llm.judge(map[string]bool{"claims_handoff": true})
		h.llm.answer.Script(tools(openTicket(map[string]any{
			"type": "Bug report", "title": "CSV export crashes", "fields": map[string]any{"steps": "Click export"},
			"confirmed": true,
		})), reply("Your ticket is {ticket_reference}. The engineering team will email you."))

		conv, trigger := h.conversation("Export crashes, yes please open it")
		res := h.run(conv, trigger)

		assert.NotEqual(t, "handoff", res.Outcome, "a ticket is how the team follows up, not a claimed handover")
		require.NotEmpty(t, h.llm.judgments())
		judged := h.llm.judgments()[0].Messages[0].Parts[0].Text
		assert.NotContains(t, judged, "{ticket_reference}", "the reply is judged as the customer will read it")
		assert.Contains(t, judged, "Your ticket is T-")

		replies := h.load(conv, res.RunID).agentReplies()
		require.NotEmpty(t, replies)
		assert.Contains(t, str(replies[len(replies)-1].Body), "Your ticket is T-")
	})

	t.Run("asks for missing required fields and for confirmation first", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, guided)
		h.d.Owner.PostJSON("/api/v1/ticket-types/"+h.ticketType("Bug report")+"/fields", testkit.JSON{
			"key": "browser", "label": "Browser", "data_type": "short_text", "required_create_customer": true,
			"visible_customer": true, "editable_customer": true,
		}).AssertCreated()
		h.llm.answer.Script(
			tools(openTicket(map[string]any{
				"type": "Bug report", "title": "Export crashes", "fields": map[string]any{}, "confirmed": true,
			})),
			tools(openTicket(map[string]any{
				"type": "Bug report", "title": "Export crashes", "fields": map[string]any{"browser": "Safari"},
				"confirmed": false,
			})),
			reply("Which should I file: Export crashes on Safari?"),
		)

		conv, trigger := h.conversation("Export crashes")
		h.run(conv, trigger)

		sent := h.llm.answer.Sent()
		require.Len(t, sent, 3)
		assert.Contains(t, sentText(h.llm.answer), "missing_fields")
		assert.Contains(t, sentText(h.llm.answer), `"status":"confirm"`)

		public := models.PublicID(models.PrefixConversation, conv)
		h.d.Owner.GetJSON("/api/v1/conversations/"+public+"/tickets").AssertOK().AssertJSONPath("data.ticket", nil)
	})

	t.Run("offers the ticket tool whatever language the guidance is in", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) {
			p.Guidance = "Quand un client signale un bug, ouvre une demande pour lui."
		})
		h.llm.answer.Script(reply("Bonjour !"))

		conv, trigger := h.conversation("Salut")
		h.run(conv, trigger)

		names := []string{}
		for _, spec := range h.llm.answer.Sent()[0].Tools {
			names = append(names, spec.Name)
		}

		assert.Contains(t, names, "open_ticket")
	})
}
