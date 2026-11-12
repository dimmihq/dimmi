package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetSeenByTeam(t *testing.T) {
	t.Parallel()

	t.Run("a conversation tells the visitor when the team read their latest message", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "is anyone there?")

		before := v.client().GetJSON("/widget/v1/conversations").AssertOK()
		assert.NotContains(t, string(before.Body), "seen_by_team_at")

		owner(tc).PatchJSON("/api/v1/conversations/"+id+"/read-state", testkit.JSON{}).AssertOK()

		list := v.client().GetJSON("/widget/v1/conversations").AssertOK()
		assert.NotEmpty(t, list.String("data.0.seen_by_team_at"))
		one := v.client().GetJSON("/widget/v1/conversations/" + id).AssertOK()
		assert.NotEmpty(t, one.String("data.seen_by_team_at"))

		v.client().PostJSON("/widget/v1/conversations/"+id+"/messages", testkit.JSON{
			"client_id": "11111111-1111-4111-8111-111111111111", "body": "one more thing",
		}).AssertCreated()
		after := v.client().GetJSON("/widget/v1/conversations").AssertOK()
		assert.NotContains(t, string(after.Body), "seen_by_team_at")
	})

	t.Run("a workspace that turned receipts off never says", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner(tc).PatchJSON("/api/v1/workspace", testkit.JSON{"seen_receipts": false}).AssertOK()
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")

		owner(tc).PatchJSON("/api/v1/conversations/"+id+"/read-state", testkit.JSON{}).AssertOK()

		list := v.client().GetJSON("/widget/v1/conversations").AssertOK()
		assert.NotContains(t, string(list.Body), "seen_by_team_at")
	})
}
