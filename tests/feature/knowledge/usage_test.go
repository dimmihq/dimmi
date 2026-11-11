package knowledge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKnowledgeUsage(t *testing.T) {
	t.Parallel()

	t.Run("embedding a source is charged to it in the month's usage", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, nil)
		settle(t, x, source)
		x.tc.DrainLater()

		usage := x.owner.GetJSON("/api/v1/knowledge/usage").AssertOK()
		usage.AssertJSONPath("data.object", "knowledge_usage")
		usage.AssertJSONPath("data.lines.0.source_id", source)
		usage.AssertJSONPath("data.lines.0.kind", "embedding")
		usage.AssertJSONPath("data.lines.0.source_name", "Hivy docs")
		usage.AssertJSONPath("data.unpriced", false)

		tokens := x.count(t,
			"SELECT coalesce(sum(input_tokens), 0) FROM knowledge_usage_periods WHERE kind = 'embedding'")
		assert.Positive(t, tokens, "the tokens embedded are counted")
		require.Positive(t, x.count(t, "SELECT count(*) FROM knowledge_usage_periods WHERE source_id IS NOT NULL"))
	})

	t.Run("a removed source keeps its line", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, nil)
		settle(t, x, source)
		x.tc.DrainLater()

		x.owner.Delete("/api/v1/knowledge/sources/" + source).AssertStatus(204)

		usage := x.owner.GetJSON("/api/v1/knowledge/usage").AssertOK()
		usage.AssertJSONPath("data.lines.0.source_id", source)
		usage.AssertJSONPath("data.lines.0.source_name", "")
	})
}
