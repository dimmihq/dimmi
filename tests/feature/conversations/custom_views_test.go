package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
oneCondition is a definition of a single condition.
*/
func oneCondition(field, op string, value any) testkit.JSON {
	return testkit.JSON{"op": "and", "groups": []testkit.JSON{
		{"op": "and", "conditions": []testkit.JSON{{"field": field, "op": op, "value": value}}},
	}}
}

/*
saveView creates a view through the API and returns its id.
*/
func saveView(t *testing.T, c *testkit.Client, scope string, definition testkit.JSON) string {
	t.Helper()

	return c.PostJSON("/api/v1/conversation-views", testkit.JSON{
		"name": "View " + testkit.UniqueName("v"), "scope": scope, "definition": definition,
	}).AssertCreated().String("data.id")
}

func TestCustomViews(t *testing.T) {
	t.Parallel()

	const views = "/api/v1/conversation-views"

	t.Run("keeps a member's personal views to themselves", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, _ := d.join(t, enums.RoleMember)
		id := saveView(t, bob, "personal", oneCondition("status", "in", []string{"open"}))

		bob.GetJSON(views).AssertOK().AssertJSONPath("data.0.id", id).AssertJSONPath("data.0.scope", "personal")
		bob.GetJSON(views+"/"+id).AssertOK().AssertJSONPath("data.warnings", []any{})
		d.owner.GetJSON(views + "/" + id).AssertNotFound()
		d.owner.PatchJSON(views+"/"+id, testkit.JSON{"name": "Mine now"}).AssertNotFound()
		assert.Empty(t, sliceOf(d.owner.GetJSON(views), "data.#.id"))

		bob.PatchJSON(views+"/"+id, testkit.JSON{"name": "Renamed", "icon": "star"}).AssertOK().
			AssertJSONPath("data.name", "Renamed").AssertJSONPath("data.icon", "star")
		bob.Delete(views + "/" + id).AssertNoContent()
		bob.GetJSON(views + "/" + id).AssertNotFound()
	})

	t.Run("takes views:manage_shared for shared views and their order", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, _ := d.join(t, enums.RoleMember)
		definition := oneCondition("status", "in", []string{"open"})

		bob.PostJSON(views, testkit.JSON{"name": "Team", "scope": "shared", "definition": definition}).
			AssertForbidden()
		first, second := saveView(t, d.owner, "shared", definition), saveView(t, d.owner, "shared", definition)
		bob.PatchJSON(views+"/"+first, testkit.JSON{"name": "Mine"}).AssertForbidden()
		bob.PatchJSON(views, testkit.JSON{"order": []string{second, first}}).AssertForbidden()

		d.owner.PatchJSON(views, testkit.JSON{"order": []string{second, first}}).AssertOK()

		assert.Equal(t, []string{second, first}, sliceOf(bob.GetJSON(views), "data.#.id"))
	})

	t.Run("refuses a duplicate name, a bad icon and definitions past the limits", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		definition := oneCondition("status", "in", []string{"open"})
		d.owner.PostJSON(views, testkit.JSON{"name": "Twice", "scope": "personal", "definition": definition}).
			AssertCreated()

		d.owner.PostJSON(views, testkit.JSON{"name": "Twice", "scope": "personal", "definition": definition}).
			AssertStatus(422).AssertJSONPath("code", "view_name_taken")
		d.owner.PostJSON(views, testkit.JSON{
			"name": "Icon", "scope": "personal", "definition": definition, "icon": "unicorn",
		}).AssertStatus(422)

		group := testkit.JSON{"op": "and", "conditions": []testkit.JSON{{"field": "unread", "op": "eq", "value": true}}}
		tooManyGroups := testkit.JSON{"op": "or", "groups": []testkit.JSON{group, group, group, group, group, group}}
		conditions := make([]testkit.JSON, 21)
		for i := range conditions {
			conditions[i] = testkit.JSON{"field": "number", "op": "gt", "value": i}
		}
		tooManyConditions := testkit.JSON{
			"op": "and", "groups": []testkit.JSON{{"op": "and", "conditions": conditions}},
		}

		for name, def := range map[string]testkit.JSON{
			"groups":     tooManyGroups,
			"conditions": tooManyConditions,
			"field":      oneCondition("nonsense", "eq", 1),
			"op":         oneCondition("status", "gte", []string{"open"}),
			"value":      oneCondition("priority", "eq", "extreme"),
			"key":        oneCondition("contact.attr.x'; drop", "eq", 1),
			"duration":   oneCondition("created_at", "older_than", "two hours"),
		} {
			d.owner.PostJSON(views, testkit.JSON{"name": name, "scope": "personal", "definition": def}).
				AssertStatus(422).AssertJSONPath("code", "invalid_definition")
		}
	})

	t.Run("stops at fifty personal views", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, err := d.tc.Operator().Exec(t.Context(), `
			INSERT INTO conversation_views (workspace_id, name, scope, owner_member_id, definition)
			SELECT $1, 'v' || n, 'personal', $2, '{"op":"and","groups":[]}'
			FROM generate_series(1, 50) n`, d.tc.Workspace.Workspace.ID, d.tc.Workspace.Owner.ID)
		require.NoError(t, err)

		d.owner.PostJSON(views, testkit.JSON{
			"name": "One more", "scope": "personal", "definition": oneCondition("unread", "eq", true),
		}).AssertStatus(422).AssertJSONPath("code", "view_limit_reached")
	})

	t.Run("warns about a condition naming a deleted team", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		team := d.owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().String("data.id")
		id := saveView(t, d.owner, "shared", oneCondition("team", "in", []string{team}))
		d.owner.Delete("/api/v1/teams/" + team).AssertNoContent()

		d.owner.GetJSON(views+"/"+id).AssertOK().
			AssertJSONPath("data.warnings.0.path", "groups.0.conditions.0").
			AssertJSONPath("data.warnings.0.code", "reference_missing")
		assert.Empty(t, listed(d.owner.GetJSON("/api/v1/conversations?filter[view]="+id)))
	})

	t.Run("sends a cursor back to the first page once the view changes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.customer(t, "ann", "A", "a")
		d.customer(t, "bea", "B", "b")
		id := saveView(t, d.owner, "personal", oneCondition("status", "in", []string{"open"}))
		list := "/api/v1/conversations?filter[view]=" + id + "&limit=1"
		cursor := d.owner.GetJSON(list).AssertOK().String("meta.next_cursor")
		require.NotEmpty(t, cursor)

		d.owner.GetJSON(list + "&cursor=" + cursor).AssertOK()
		wider := oneCondition("status", "in", []string{"open", "closed"})
		d.owner.PatchJSON(views+"/"+id, testkit.JSON{"definition": wider}).AssertOK()

		d.owner.GetJSON(list+"&cursor="+cursor).AssertStatus(400).AssertJSONPath("code", "cursor_mismatch")
	})

	t.Run("counts the open conversations of pinned views", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.customer(t, "ann", "Refund", "a")
		id := saveView(t, d.owner, "personal", oneCondition("subject", "contains", "fund"))
		pinned := testkit.JSON{"sidebar": testkit.JSON{"pinned_views": []string{id}}}
		d.owner.PatchJSON("/api/v1/inbox-preferences", pinned).AssertOK()

		d.owner.GetJSON("/api/v1/conversation-counts").AssertOK().AssertJSONPath("data.views.custom."+id+".open", 1)
	})

	t.Run("lets another member's personal view list nothing for others", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, _ := d.join(t, enums.RoleMember)
		id := saveView(t, bob, "personal", oneCondition("status", "in", []string{"open"}))

		d.owner.GetJSON("/api/v1/conversations?filter[view]=" + id).AssertNotFound()
	})
}
