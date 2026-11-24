package knowledge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/fake"
)

/*
TestKnowledgeSeveralQueries searches with the question and another way
of asking it.
*/
func TestKnowledgeSeveralQueries(t *testing.T) {
	t.Parallel()

	t.Run("searches each way of asking on its own, and the results take turns", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		x.embed.Script("plans pricing", fake.Axis(0, 1024)).Script("nine dollars", fake.Axis(0, 1024)).
			Script("just opened", fake.Axis(1, 1024)).Script("three months free", fake.Axis(1, 1024))
		x.publish(t, "Plans", "Plans start at nine dollars a month.")
		x.publish(t, "Plans in detail", "Every plan is nine dollars more than the one below it.")
		x.publish(t, "New shop offer", "New shops get three months free.")
		x.tc.Drain()
		before := len(x.queries())

		question, angle := "plans pricing", "Is there an offer for a shop that has just opened?"
		res, err := x.tc.App.Knowledge.Search(t.Context(), x.tc.Workspace.Workspace.ID, question,
			contracts.KnowledgeSearchOptions{
				Queries:    []string{angle, question, " "},
				Visibility: []contracts.KnowledgeVisibility{"public", "agent_only"},
			})
		require.NoError(t, err)

		asked := x.queries()[before:]
		require.Len(t, asked, 1, "the ways of asking are embedded together, in one request")
		assert.Equal(t, []string{question, angle}, asked[0].Inputs, "each once, the question first")
		assert.Equal(t, []string{angle, question}, x.rerank.read(),
			"each query's passages are ranked against that query's own words")
		assert.Equal(t, "reranked", res.Mode)
		require.GreaterOrEqual(t, len(res.Chunks), 3)
		assert.Contains(t, res.Chunks[0].Title, "Plans", "the question's best passage leads")
		assert.Equal(t, "New shop offer", res.Chunks[1].Title,
			"the other query's best passage comes second, though the question alone would not rank it there")
		seen := map[string]bool{}
		for i, c := range res.Chunks {
			assert.False(t, seen[c.ID.String()], "a passage two queries found is kept once")
			seen[c.ID.String()] = true
			assert.Equal(t, i+1, c.Rank)
		}
		assert.NotNil(t, res.EmbeddingUsage, "the one request's cost is reported")
	})
}
