package reports_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
attributeTable reads the AI report's table of conversations by AI
attribute: its title and rows, or false when the report has none.
*/
func attributeTable(t *testing.T, res *testkit.Response) (string, []agentRow, bool) {
	t.Helper()

	var body struct {
		Data struct {
			Charts []struct {
				Key   string     `json:"key"`
				Title string     `json:"title"`
				Rows  []agentRow `json:"rows"`
			} `json:"charts"`
		} `json:"data"`
	}
	res.Decode(&body)

	for _, c := range body.Data.Charts {
		if c.Key == "ai_attribute" {
			return c.Title, c.Rows, true
		}
	}

	return "", nil, false
}

/*
rowsByKey indexes a table's rows by key.
*/
func rowsByKey(rows []agentRow) map[string]agentRow {
	out := map[string]agentRow{}
	for _, r := range rows {
		out[r.Key] = r
	}

	return out
}

func TestReportAIAttributes(t *testing.T) {
	t.Parallel()

	t.Run("the AI report splits by an attribute and every report filters by its value", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.customer("ana")
		d.customer("bo")
		d.refresh()

		_, _, shown := attributeTable(t, d.Owner.GetJSON("/api/v1/reports/ai").AssertOK())
		assert.False(t, shown, "no attribute, no table")

		d.Owner.PostJSON("/api/v1/ai/conversation-attributes", testkit.JSON{
			"key": "reason", "label": "Reason", "data_type": "choice", "options": []string{"billing", "bug"},
		}).AssertCreated()
		d.Owner.PostJSON("/api/v1/ai/conversation-attributes", testkit.JSON{
			"key": "urgent", "label": "Urgent", "data_type": "boolean",
		}).AssertCreated()
		d.Owner.PostJSON("/api/v1/ai/conversation-attributes", testkit.JSON{
			"key": "order", "label": "Order number", "data_type": "text",
		}).AssertCreated()

		d.exec(`INSERT INTO conversation_ai_values (workspace_id, conversation_id, "values", checked_at)
			SELECT workspace_id, id, '{"reason": "billing", "urgent": true}', now() FROM conversations
			WHERE workspace_id = $1 AND number = (SELECT min(number) FROM conversations WHERE workspace_id = $1)`)
		d.exec(`INSERT INTO conversation_ai_values (workspace_id, conversation_id, "values", checked_at)
			SELECT workspace_id, id, '{"reason": null}', now() FROM conversations
			WHERE workspace_id = $1 AND number = (SELECT max(number) FROM conversations WHERE workspace_id = $1)`)

		title, rows, shown := attributeTable(t, d.Owner.GetJSON("/api/v1/reports/ai").AssertOK())
		require.True(t, shown)
		assert.Equal(t, "Conversations by Reason", title, "the first active choice or yes-or-no attribute")
		byKey := rowsByKey(rows)
		require.Len(t, rows, 3)
		assert.InDelta(t, 1, *byKey["billing"].Values[0], 0)
		assert.InDelta(t, 0, *byKey["bug"].Values[0], 0)
		assert.InDelta(t, 1, *byKey["none"].Values[0], 0)
		assert.Equal(t, "Not found yet", byKey["none"].Label)
		assert.Equal(t, "none", rows[2].Key, "not found comes last")

		title, rows, _ = attributeTable(t, d.Owner.GetJSON("/api/v1/reports/ai?ai_breakdown=urgent").AssertOK())
		assert.Equal(t, "Conversations by Urgent", title)
		byKey = rowsByKey(rows)
		assert.Equal(t, "Yes", byKey["true"].Label)
		assert.InDelta(t, 1, *byKey["true"].Values[0], 0)
		assert.InDelta(t, 0, *byKey["false"].Values[0], 0)
		assert.InDelta(t, 1, *byKey["none"].Values[0], 0)

		d.Owner.GetJSON("/api/v1/reports/ai?ai_breakdown=order").AssertNotFound()
		d.Owner.GetJSON("/api/v1/reports/ai?ai_breakdown=missing").AssertNotFound()

		overview := func(query string) any {
			return metric(t, d.Owner.GetJSON("/api/v1/reports/overview?"+query).AssertOK(), "new_conversations")
		}
		assert.InDelta(t, 2, overview(""), 0)
		assert.InDelta(t, 1, overview("filter[ai_attribute]=reason"), 0, "found on one, null on the other")
		assert.InDelta(t, 1, overview("filter[ai_attribute]=reason&filter[ai_attribute_value]=billing"), 0)
		assert.InDelta(t, 0, overview("filter[ai_attribute]=reason&filter[ai_attribute_value]=bug"), 0)
		assert.InDelta(t, 1, overview("filter[ai_attribute]=urgent&filter[ai_attribute_value]=true"), 0)

		d.Owner.GetJSON("/api/v1/reports/overview?filter[ai_attribute_value]=bug").AssertNotFound()
		d.Owner.GetJSON("/api/v1/reports/overview?filter[ai_attribute]=Not-A-Key").AssertStatus(422)

		d.Owner.GetJSON("/api/v1/reports/ai/conversations?metric=new_conversations"+
			"&filter[ai_attribute]=reason&filter[ai_attribute_value]=billing").AssertOK().AssertJSONPath("total", 1)
	})
}
