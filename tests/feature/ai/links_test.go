package ai_test

import (
	"context"
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

	t.Run("keeps a link the cited passage quotes and drops one it does not", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.d.TC.App.AI.Knowledge = quoting{Knowledge: h.d.TC.App.AI.Knowledge}

		conv, trigger := h.conversation("How do I create an order through the API?")
		h.llm.answer.Script(search("create order api"), reply(
			"Send a `POST` to `https://api.lovelace.test/v1/orders` [1]. See also https://evil.example/login."))
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		replies := h.load(conv, res.RunID).agentReplies()
		require.Len(t, replies, 1)
		body := str(replies[0].Body)
		assert.Contains(t, body, "`https://api.lovelace.test/v1/orders`")
		assert.NotContains(t, body, "evil.example")
	})
}
