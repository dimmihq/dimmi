package operator

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/actions/conversations"
)

func TestListConversationsFilters(t *testing.T) {
	t.Parallel()

	t.Run("offers a tag filter beside the others", func(t *testing.T) {
		t.Parallel()

		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}

		require.NoError(t, json.Unmarshal([]byte(listSchema), &schema))

		for _, name := range []string{
			"status", "assignee", "ai_agent", "tag", "customer_waiting", "started_since", "started_until", "scan",
			"order", "limit",
		} {
			assert.Contains(t, schema.Properties, name)
		}
	})

	t.Run("reads the tag and names it in the run's step", func(t *testing.T) {
		t.Parallel()

		var in listArgs

		require.NoError(t, decode(json.RawMessage(`{"status":"any","tag":"billing","customer_waiting":true}`), &in))

		assert.Equal(t, "billing", in.Tag)
		assert.Equal(t, "any, tagged billing, waiting", describe(in))
		assert.Equal(t, "open, waiting, longest wait first", describe(listArgs{Order: orderLongestWaiting}))
	})

	t.Run("lists a few in full and many as a scan", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 15, limitOf(listArgs{}))
		assert.Equal(t, 25, limitOf(listArgs{Limit: 90}))
		assert.Equal(t, 200, limitOf(listArgs{Scan: true}))
		assert.Equal(t, 60, limitOf(listArgs{Scan: true, Limit: 60}))
		assert.Equal(t, 200, limitOf(listArgs{Scan: true, Limit: 900}))
	})

	t.Run("narrows to the days named, the last one included", func(t *testing.T) {
		t.Parallel()

		var f conversations.Filters

		require.Empty(t, between(listArgs{Since: "2026-03-02", Until: "2026-03-08"}, &f))
		require.NotNil(t, f.CreatedAfter)
		require.NotNil(t, f.CreatedBefore)
		assert.Equal(t, "2026-03-02", f.CreatedAfter.Format(time.DateOnly))
		assert.Equal(t, "2026-03-09", f.CreatedBefore.Format(time.DateOnly))
	})

	t.Run("tells the model when a day is not a date", func(t *testing.T) {
		t.Parallel()

		var f conversations.Filters

		assert.Contains(t, between(listArgs{Since: "last week"}, &f), "started_since must be a day")
		assert.Contains(t, between(listArgs{Until: "03/08"}, &f), "started_until must be a day")
		assert.Nil(t, f.CreatedAfter)
	})

	t.Run("says why the agent handed over only when it did", func(t *testing.T) {
		t.Parallel()

		reason := "no_answer"

		assert.Equal(t, " · handed over: no_answer", handedOver(&reason))
		assert.Empty(t, handedOver(nil))
	})

	t.Run("leaves the tag to the lookup, never to the inbox filters", func(t *testing.T) {
		t.Parallel()

		filters := filtersOf(listArgs{Status: "any", Tag: "billing"})

		assert.True(t, filters.AnyStatus)
		assert.Empty(t, filters.TagsAny)
	})
}
