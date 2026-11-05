package access_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestWorkbenchScope(t *testing.T) {
	t.Parallel()

	openOnes := testkit.JSON{"op": "and", "groups": []testkit.JSON{
		{"op": "and", "conditions": []testkit.JSON{{"field": "status", "op": "in", "value": []string{"open"}}}},
	}}

	t.Run("a saved view and its pinned count hold only what the member may see", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		mine, theirs := d.customer(t, "ann"), d.customer(t, "bob")
		d.assign(t, mine, testkit.JSON{"assignee_id": m.public()})
		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		view := d.owner.PostJSON("/api/v1/conversation-views", testkit.JSON{
			"name": "Open", "scope": "shared", "definition": openOnes,
		}).AssertCreated().String("data.id")

		assert.Equal(t, []string{mine}, listed(t, m.client.GetJSON("/api/v1/conversations?filter[view]="+view)))
		assert.ElementsMatch(t, []string{mine, theirs},
			listed(t, d.owner.GetJSON("/api/v1/conversations?filter[view]="+view)))

		m.client.PatchJSON("/api/v1/inbox-preferences", testkit.JSON{
			"sidebar": testkit.JSON{"pinned_views": []string{view}},
		}).AssertOK()
		m.client.GetJSON("/api/v1/conversation-counts").AssertOK().
			AssertJSONPath("data.views.custom."+view+".open", 1.0)
	})

	t.Run("a draft hides while its conversation is out of sight and returns with it", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		conv := d.customer(t, "ann")
		d.assign(t, conv, testkit.JSON{"assignee_id": m.public()})
		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		id := m.client.PostJSON("/api/v1/drafts", testkit.JSON{
			"conversation_id": conv, "mode": "reply", "body": "Half written", "client_updated_at": d.tc.Clock.Now(),
		}).AssertCreated().String("data.id")

		d.assign(t, conv, testkit.JSON{"assignee_id": nil})
		m.client.GetJSON("/api/v1/drafts").AssertJSONPath("data.#", 0)
		m.client.PatchJSON("/api/v1/drafts/"+id, testkit.JSON{"body": "x", "client_updated_at": d.tc.Clock.Now()}).
			AssertNotFound()

		d.assign(t, conv, testkit.JSON{"assignee_id": m.public()})
		m.client.GetJSON("/api/v1/drafts").AssertJSONPath("data.#", 1).AssertJSONPath("data.0.body", "Half written")
	})

	t.Run("snooze choices, message moves and drafts on a hidden conversation are not found", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		hidden := d.customer(t, "bob")
		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		note := d.owner.PostJSON("/api/v1/conversations/"+hidden+"/notes", testkit.JSON{"body": "Private"}).
			AssertCreated().String("data.id")
		path := "/api/v1/conversations/" + hidden

		m.client.GetJSON("/api/v1/snooze-options?conversation=" + hidden).AssertNotFound()
		m.client.Send("DELETE", path+"/messages/"+note, nil).AssertNotFound()
		m.client.PatchJSON(path+"/messages/"+note, testkit.JSON{"body": "x"}).AssertNotFound()
		m.client.PostJSON("/api/v1/drafts", testkit.JSON{
			"conversation_id": hidden, "mode": "note", "body": "x", "client_updated_at": d.tc.Clock.Now(),
		}).AssertNotFound()
		m.client.GetJSON("/api/v1/drafts?filter[conversation]=" + hidden).AssertNotFound()
	})
}
