package ai_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
quoting is a knowledge base whose one passage quotes an API address on
another host than the help center.
*/
type quoting struct {
	contracts.Knowledge
}

func (quoting) Search(context.Context, ids.UUID, string, contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	similarity := 0.9
	return contracts.KnowledgeSearchResult{Mode: "hybrid", Chunks: []contracts.KnowledgeChunk{{
		ID: ids.New(), DocumentID: ids.New(), Title: "Create an order", URL: "https://help.lovelace.test/orders",
		Text:       "Send a POST to https://api.lovelace.test/v1/orders with a key that has orders:write.",
		Similarity: &similarity, MatchedBy: []string{"vector"},
	}}}, nil
}

func TestReplyLinks(t *testing.T) {
	t.Parallel()

	t.Run("keeps a link the cited passage quotes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.d.TC.App.AI.Knowledge = quoting{Knowledge: h.d.TC.App.AI.Knowledge}

		conv, trigger := h.conversation("How do I create an order through the API?")
		h.llm.answer.Script(search("create order api"), reply("Send a `POST` to `https://api.lovelace.test/v1/orders` [1]."))
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		replies := h.load(conv, res.RunID).agentReplies()
		require.Len(t, replies, 1)
		assert.Equal(t, "Send a `POST` to `https://api.lovelace.test/v1/orders`.", str(replies[0].Body))
		assert.NotContains(t, h.load(conv, res.RunID).run.Flags, "link_regenerated")
	})

	t.Run("has a reply with an unknown link written again instead of cutting it", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.d.TC.App.AI.Knowledge = quoting{Knowledge: h.d.TC.App.AI.Knowledge}

		conv, trigger := h.conversation("How do I create an order through the API?")
		h.llm.answer.Script(search("create order api"),
			reply("Send a `POST` to the orders endpoint [1]. The guide is at https://docs.lovelace.test/orders."),
			reply("Send a `POST` to `https://api.lovelace.test/v1/orders` [1]; the orders guide covers the body."))
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Contains(t, s.run.Flags, "link_regenerated")
		assert.NotContains(t, s.run.Flags, "link_removed")
		assert.Equal(t, "Send a `POST` to `https://api.lovelace.test/v1/orders`; the orders guide covers the body.",
			str(s.agentReplies()[0].Body), "the customer gets the rewritten reply, nothing cut out")

		sent := h.llm.answer.Sent()
		note := sent[len(sent)-1].Messages[len(sent[len(sent)-1].Messages)-1].Parts[0].Text
		assert.Contains(t, note, "<system_note>")
		assert.Contains(t, note, "https://docs.lovelace.test/orders")
	})
}

func TestEarlierReplies(t *testing.T) {
	t.Parallel()

	t.Run("shows the model its earlier replies with the documents they cited", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, trigger := h.conversation("How much is Pro per seat?")
		h.llm.answer.Script(search("pro plan price per seat"), reply("Pro costs $12 per seat per month [1]."))
		first := h.run(conv, trigger)
		require.Equal(t, "answered", first.Outcome)
		replies := h.load(conv, first.RunID).agentReplies()
		assert.Equal(t, "Pro costs $12 per seat per month.", str(replies[0].Body), "the customer sees no markers")

		next := h.add(conv, "contact", "message", "And Business?")
		h.llm.answer.Script(search("business plan price"), reply(
			"Business costs $20 per seat [1], like Pro [S1] [S?].\n\n"+
				"[S1] Plans and pricing\n<cited>[S1] Plans</cited>"))
		res := h.run(conv, next)
		require.Equal(t, "answered", res.Outcome)

		sent := h.llm.answer.Sent()
		var earlier string
		for _, m := range sent[len(sent)-2].Messages {
			if m.Role == contracts.ChatRoleAssistant {
				earlier = m.Parts[0].Text
			}
		}
		assert.Regexp(t, `^<cited>\[S1\] [^<]+</cited>\nPro costs \$12 per seat per month \[S1\]\.$`, earlier)

		replies = h.load(conv, res.RunID).agentReplies()
		assert.Equal(t, "Business costs $20 per seat, like Pro.", str(replies[len(replies)-1].Body),
			"an earlier reply's markers, a made-up one and the source list never reach the customer")
	})
}

/*
twoParts is a knowledge base whose one article comes back as two
passages.
*/
type twoParts struct {
	contracts.Knowledge
	doc ids.UUID
}

func (k twoParts) Search(context.Context, ids.UUID, string, contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	similarity := 0.9
	part := func(n int, text string) contracts.KnowledgeChunk {
		return contracts.KnowledgeChunk{
			ID: ids.New(), DocumentID: k.doc, Title: "Automation rules", URL: "https://help.lovelace.test/rules",
			Text: text, Similarity: &similarity, MatchedBy: []string{"vector"}, Rank: n,
		}
	}
	return contracts.KnowledgeSearchResult{Mode: "hybrid", Chunks: []contracts.KnowledgeChunk{
		part(1, "Open a rule and choose Test to try it on an order without changing anything."),
		part(2, "The rule action Split by tag splits an order into parcels by SKU tag."),
	}}, nil
}

func TestCarriedPassages(t *testing.T) {
	t.Parallel()

	t.Run("carries every passage a reply cites, not one per article", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator("UPDATE ai_agents SET carry_passages = true WHERE workspace_id = $1", h.d.TC.Workspace.Workspace.ID)
		h.d.TC.App.AI.Knowledge = twoParts{Knowledge: h.d.TC.App.AI.Knowledge, doc: ids.New()}

		conv, trigger := h.conversation("Can rules split orders, and can I try one first?")
		h.llm.answer.Script(search("rules split test"),
			reply("Choose **Test** on a rule to try it [1], and the **Split by tag** action splits orders [2]."))
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		m := h.memoryOf(conv)
		require.Len(t, m.Passages, 2)
		assert.Equal(t, m.Passages[0].Title, m.Passages[1].Title, "both passages of the one article are kept")
	})
}

func TestImpliedHandoff(t *testing.T) {
	t.Parallel()

	t.Run("makes a hand-over the reply claims real", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, trigger := h.conversation("Can I get a refund on last month's invoice?")
		h.llm.judge(map[string]bool{"claims_handoff": true})
		h.llm.answer.Script(search("refund invoice"),
			reply("Monthly plans renew automatically [1]. "+
				"I've asked the billing team to look at last month's invoice."))
		res := h.run(conv, trigger)

		assert.Equal(t, "handoff", res.Outcome)
		assert.Equal(t, "other", res.HandoffReason)
		s := h.load(conv, res.RunID)
		assert.Equal(t, "handed_off", s.conv.AiState, "the team really has the conversation")
		assert.Contains(t, s.run.Flags, "implied_handoff")
	})

	t.Run("leaves a reply that only offers the team alone", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		conv, trigger := h.conversation("How much is Pro per seat?")
		h.llm.answer.Script(search("pro plan price per seat"),
			reply("Pro costs $12 per seat per month [1]. "+
				"I haven't passed anything on; want me to get someone from the team?"))
		assert.Equal(t, "answered", h.run(conv, trigger).Outcome)
	})
}

/*
longPart is a knowledge base whose one long passage states the cited
fact near its end.
*/
type longPart struct {
	contracts.Knowledge
}

func (longPart) Search(context.Context, ids.UUID, string, contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	similarity := 0.9
	filler := strings.Repeat("Presets save a package and service for reuse. ", 120)
	return contracts.KnowledgeSearchResult{Mode: "hybrid", Chunks: []contracts.KnowledgeChunk{{
		ID: ids.New(), DocumentID: ids.New(), Title: "Release notes", URL: "https://help.lovelace.test/notes",
		Text:       filler + "The rule action Split by tag splits an order into parcels by SKU tag. " + filler,
		Similarity: &similarity, MatchedBy: []string{"vector"},
	}}}, nil
}

func TestCarriedCitedPart(t *testing.T) {
	t.Parallel()

	t.Run("keeps the part of a long passage the reply cited", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator("UPDATE ai_agents SET carry_passages = true WHERE workspace_id = $1", h.d.TC.Workspace.Workspace.ID)
		h.d.TC.App.AI.Knowledge = longPart{Knowledge: h.d.TC.App.AI.Knowledge}

		conv, trigger := h.conversation("Can rules split an order by SKU tag?")
		h.llm.answer.Script(search("split order sku tag"), reply("Presets save a package and service for reuse, "+
			"and presets save a package for every order. "+
			"Separately, the **Split by tag** action splits an order by SKU tag [1]."))
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)

		var text string
		require.NoError(t, h.d.TC.Operator().QueryRow(t.Context(),
			"SELECT ai_memory->'passages'->0->>'text' FROM conversations WHERE id = $1", conv).Scan(&text))
		assert.Contains(t, text, "Split by tag splits an order")
		assert.LessOrEqual(t, len([]rune(text)), 2500)
	})

	t.Run("keeps every fact the reply took from a long passage, marked or not", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator("UPDATE ai_agents SET carry_passages = true WHERE workspace_id = $1", h.d.TC.Workspace.Workspace.ID)
		h.d.TC.App.AI.Knowledge = spreadFacts{Knowledge: h.d.TC.App.AI.Knowledge}

		conv, trigger := h.conversation("How do I create an order through the API?")
		h.llm.answer.Script(search("create order api"), reply("Send a POST to the orders endpoint [1]. "+
			"Authenticate with a key carrying the orders:write scope."))
		require.Equal(t, "answered", h.run(conv, trigger).Outcome)

		var text string
		require.NoError(t, h.d.TC.Operator().QueryRow(t.Context(),
			"SELECT ai_memory->'passages'->0->>'text' FROM conversations WHERE id = $1", conv).Scan(&text))
		assert.Contains(t, text, "POST to the orders endpoint")
		assert.Contains(t, text, "orders:write scope", "a fact the reply states without a marker is carried too")
		assert.LessOrEqual(t, len([]rune(text)), 2500)
	})
}

/*
spreadFacts is a knowledge base whose one long passage states two facts
far apart, among varied filler.
*/
type spreadFacts struct {
	contracts.Knowledge
}

func (spreadFacts) Search(context.Context, ids.UUID, string, contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	similarity := 0.9
	var filler strings.Builder
	for i := range 60 {
		fmt.Fprintf(&filler, "Field number %d of the sample object holds a value you can ignore here. ", i)
	}
	return contracts.KnowledgeSearchResult{Mode: "hybrid", Chunks: []contracts.KnowledgeChunk{{
		ID: ids.New(), DocumentID: ids.New(), Title: "API reference", URL: "https://help.lovelace.test/api",
		Text: "Create one with a POST to the orders endpoint. " + filler.String() +
			"Authenticate with a key carrying the orders:write scope. " + filler.String(),
		Similarity: &similarity, MatchedBy: []string{"vector"},
	}}}, nil
}
