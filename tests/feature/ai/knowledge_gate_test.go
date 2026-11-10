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
scripted answers every search with the same chunks.
*/
type scripted struct {
	contracts.Knowledge
	chunks []contracts.KnowledgeChunk
}

func (s *scripted) Search(context.Context, ids.UUID, string, contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	return contracts.KnowledgeSearchResult{Chunks: s.chunks, Mode: "hybrid"}, nil
}

/*
scored is a chunk found by the retrievers with the scores.
*/
func scored(title string, similarity float64, coverage *float64, by ...string) contracts.KnowledgeChunk {
	return contracts.KnowledgeChunk{
		ID: ids.New(), DocumentID: ids.New(), Title: title, Text: title + " text", Similarity: &similarity,
		Coverage: coverage, MatchedBy: by, Visibility: contracts.KnowledgeVisibilityPublic,
	}
}

func TestSimilarityGate(t *testing.T) {
	t.Parallel()

	t.Run("keeps passages found by meaning and by keywords, each on its own scale", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		low, high := 0.2, 0.8
		h.d.TC.App.AI.Knowledge = &scripted{Knowledge: h.d.TC.App.AI.Knowledge, chunks: []contracts.KnowledgeChunk{
			scored("Rephrased match", 0.72, nil, "vector"),
			scored("Weak keywords", low, &low, "bm25"),
			scored("Strong keywords", 0.3, &high, "bm25", "vector"),
			scored("Distant meaning", 0.3, nil, "vector"),
			scored("Legacy keyword", 0.5, nil, "bm25"),
		}}
		h.llm.answer.Script(search("change plan mid month"), reply("Upgrades are prorated [1]."))

		conv, trigger := h.conversation("Can I change my plan in the middle of the month? Is it prorated?")
		h.run(conv, trigger)

		sent := h.llm.answer.Sent()
		require.Len(t, sent, 2)
		results := sent[1].Messages[len(sent[1].Messages)-1].Parts[0].Text
		assert.Contains(t, results, "Rephrased match")
		assert.Contains(t, results, "Strong keywords")
		assert.Contains(t, results, "Legacy keyword")
		assert.NotContains(t, results, "Weak keywords")
		assert.NotContains(t, results, "Distant meaning")
	})
}
