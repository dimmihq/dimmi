package ai_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestAiPlacements(t *testing.T) {
	t.Parallel()

	t.Run("lists the inboxes an agent can take, and who takes each", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Anthropic")
		sales := d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Sales"}).AssertCreated().
			String("data.agent.id")

		list := d.Owner.GetJSON("/api/v1/ai/placements").AssertOK()
		var inbox string
		for i := 0; list.String("data."+strconv.Itoa(i)+".id") != ""; i++ {
			at := "data." + strconv.Itoa(i) + "."
			if list.String(at+"kind") == "inbox" {
				inbox = list.String(at + "id")
				assert.Empty(t, list.String(at+"agent_id"), "nobody but the default takes it yet")
				break
			}
		}
		require.NotEmpty(t, inbox)

		path := "/api/v1/ai/agents/" + sales
		d.Owner.PatchJSON(path, testkit.JSON{
			"answer_key_id": key, "answer_model": "claude-test",
			"placement": testkit.JSON{"inbox_ids": []string{inbox}},
		}).AssertOK()
		d.Owner.PostJSON(path+"/publications", nil).AssertOK()

		again := d.Owner.GetJSON("/api/v1/ai/placements").AssertOK()
		for i := 0; again.String("data."+strconv.Itoa(i)+".id") != ""; i++ {
			if at := "data." + strconv.Itoa(i) + "."; again.String(at+"id") == inbox {
				again.AssertJSONPath(at+"agent_id", sales)
			}
		}
	})
}
