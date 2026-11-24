package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

func TestFuse(t *testing.T) {
	t.Parallel()

	cosine, coverage := 0.62, 0.21
	both := contracts.KnowledgeChunk{
		ID: ids.New(), DocumentID: ids.New(), Text: "Connect the store from the app store.",
		Similarity: &cosine, Coverage: &coverage, MatchedBy: []string{"vector", "bm25"},
	}
	keywordOnly := contracts.KnowledgeChunk{
		ID: ids.New(), DocumentID: ids.New(), Text: "Stores sync every five minutes.",
		Similarity: &coverage, Coverage: &coverage, MatchedBy: []string{"bm25"},
	}

	t.Run("keeps the similarity by meaning of a passage one list carries with both scores", func(t *testing.T) {
		t.Parallel()

		out := Fuse(8, 3, []contracts.KnowledgeChunk{both, keywordOnly})

		require.Len(t, out, 2)
		assert.InDelta(t, cosine, *out[0].Similarity, 1e-9, "keyword coverage must not replace it")
		assert.InDelta(t, coverage, *out[0].Coverage, 1e-9)
		assert.ElementsMatch(t, []string{"vector", "bm25"}, out[0].MatchedBy)
		assert.InDelta(t, coverage, *out[1].Similarity, 1e-9, "a passage only keywords found is scored by coverage")
	})

	t.Run("keeps it whichever list names the passage first", func(t *testing.T) {
		t.Parallel()
		byMeaning, byKeyword := both, both
		byMeaning.Coverage, byMeaning.MatchedBy = nil, []string{"vector"}
		byKeyword.Similarity, byKeyword.MatchedBy = &coverage, []string{"bm25"}

		for _, lists := range [][][]contracts.KnowledgeChunk{
			{{byMeaning}, {byKeyword}},
			{{byKeyword}, {byMeaning}},
		} {
			out := Fuse(8, 3, lists...)

			require.Len(t, out, 1)
			assert.InDelta(t, cosine, *out[0].Similarity, 1e-9)
			assert.InDelta(t, coverage, *out[0].Coverage, 1e-9)
		}
	})
}
