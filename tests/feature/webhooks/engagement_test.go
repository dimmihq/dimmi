package webhooks_test

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
hook is one delivered webhook: its type and its data.
*/
type hook struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

/*
received reads every webhook the fake was sent, by type, keeping the
last of each.
*/
func received(t *testing.T, fake *httpclienttest.Fake) map[string]hook {
	t.Helper()

	out := map[string]hook{}
	for _, req := range fake.Sent() {
		var h hook
		require.NoError(t, json.Unmarshal(req.Body, &h))
		out[h.Type] = h
	}

	return out
}

/*
object returns the object a webhook carries.
*/
func (h hook) object(t *testing.T) map[string]any {
	t.Helper()

	obj, ok := h.Data["object"].(map[string]any)
	require.True(t, ok, "webhook %s carries no object", h.Type)

	return obj
}

/*
field returns a nested object of a webhook's data.
*/
func field(t *testing.T, data map[string]any, key string) map[string]any {
	t.Helper()

	out, ok := data[key].(map[string]any)
	require.True(t, ok, "no %s in the webhook's data", key)

	return out
}

/*
sameShape asserts the webhook's object has the fields the API answers
with for the same object.
*/
func sameShape(t *testing.T, h hook, api *testkit.Response) {
	t.Helper()

	var body struct {
		Data map[string]any `json:"data"`
	}
	api.Decode(&body)

	got := h.object(t)
	assert.Equal(t, body.Data["id"], got["id"], h.Type)
	assert.Equal(t, body.Data["object"], got["object"], h.Type)
	assert.ElementsMatch(t, slices.Collect(maps.Keys(body.Data)), slices.Collect(maps.Keys(got)), h.Type)
}

/*
subscribe adds an endpoint for the event types.
*/
func subscribe(d tests.Desk, types ...string) {
	d.Owner.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{"url": hookURL, "event_types": types}).
		AssertCreated()
}

func TestEngagementWebhooks(t *testing.T) {
	t.Parallel()

	t.Run("lists the new areas in the event picker", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		body := string(d.Owner.GetJSON("/api/v1/webhook-event-types").AssertOK().Body)

		for _, name := range []string{
			"changelog_entry.published", "feedback_post.linked_conversation", "broadcast.sent",
			"series.journey_completed", "contact.merged",
		} {
			assert.Contains(t, body, `"`+name+`"`)
		}
	})

	t.Run("tells about changelog entries going live, changing and coming down", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP)
		subscribe(d, "changelog_entry.*")
		hc := d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		id := d.Owner.PostJSON("/api/v1/help-centers/"+hc+"/changelog-entries", testkit.JSON{"title": "Dark mode"}).
			AssertCreated().String("data.id")
		path := "/api/v1/changelog-entries/" + id

		d.Owner.PostJSON(path+"/publications", testkit.JSON{}).AssertOK()
		d.Owner.PatchJSON(path, testkit.JSON{"title": "Dark mode everywhere"}).AssertOK()
		d.Owner.PostJSON(path+"/publications", testkit.JSON{}).AssertOK()
		d.Owner.Delete(path + "/publications").AssertOK()
		d.TC.Drain()

		got := received(t, fake)
		require.Len(t, got, 3)
		for _, name := range []string{"changelog_entry.published", "changelog_entry.updated"} {
			require.Contains(t, got, name)
			assert.Equal(t, id, got[name].object(t)["id"])
		}
		require.Contains(t, got, "changelog_entry.unpublished")
		sameShape(t, got["changelog_entry.unpublished"], d.Owner.GetJSON(path).AssertOK())
	})

	t.Run("tells about feature requests posted, voted, moved, merged and linked", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP)
		subscribe(d, "feedback_post.*")
		hc := d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		d.Owner.PatchJSON("/api/v1/help-centers/"+hc, testkit.JSON{"settings": testkit.JSON{"feedback_board": true}}).
			AssertOK()
		posts := "/api/v1/help-centers/" + hc + "/feedback-posts"
		kept := d.Owner.PostJSON(posts, testkit.JSON{"title": "SAML"}).AssertCreated().String("data.id")
		dup := d.Owner.PostJSON(posts, testkit.JSON{"title": "SAML please"}).AssertCreated().String("data.id")
		conv := d.Customer(t, "cy", "Need SAML")

		d.Owner.PostJSON("/api/v1/feedback-posts/"+kept+"/votes", testkit.JSON{
			"contact_id": conv.String("data.contact.id"), "priority": "must",
		}).AssertOK()
		d.Owner.PatchJSON("/api/v1/feedback-posts/"+kept, testkit.JSON{"status": "planned"}).AssertOK()
		d.Owner.PatchJSON("/api/v1/feedback-posts/"+dup, testkit.JSON{"merge_into_id": kept}).AssertOK()
		d.Owner.PostJSON("/api/v1/conversations/"+conv.String("data.id")+"/feedback-posts", testkit.JSON{
			"post_id": kept, "priority": "must",
		}).AssertOK()
		d.TC.Drain()

		got := received(t, fake)
		for _, name := range []string{
			"feedback_post.created", "feedback_post.voted", "feedback_post.status_changed",
			"feedback_post.merged", "feedback_post.linked_conversation",
		} {
			require.Contains(t, got, name)
		}

		moved := got["feedback_post.status_changed"]
		assert.Equal(t, kept, moved.object(t)["id"])
		status := field(t, moved.Data, "changes")["status"]
		assert.Equal(t, map[string]any{"from": "under_review", "to": "planned"}, status)
		assert.Equal(t, dup, got["feedback_post.merged"].Data["merged_id"])
		assert.Equal(t, "must", field(t, got["feedback_post.voted"].Data, "vote")["priority"])
		assert.Equal(t, conv.String("data.id"), got["feedback_post.linked_conversation"].Data["conversation_id"])
		created := got["feedback_post.created"]
		createdID, _ := created.object(t)["id"].(string)
		sameShape(t, created, d.Owner.GetJSON("/api/v1/feedback-posts/"+createdID).AssertOK())
	})

	t.Run("tells about a contact merged into another", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP)
		subscribe(d, "contact.merged")
		target := d.Customer(t, "ann", "Billing").String("data.contact.id")
		source := d.Customer(t, "anna", "Login").String("data.contact.id")

		d.Owner.PostJSON("/api/v1/contacts/"+target+"/merges", testkit.JSON{"source_id": source}).AssertOK()
		d.TC.Drain()

		got := received(t, fake)
		require.Contains(t, got, "contact.merged")
		assert.Equal(t, source, got["contact.merged"].Data["merged_id"])
		sameShape(t, got["contact.merged"], d.Owner.GetJSON("/api/v1/contacts/"+target).AssertOK())
	})

	t.Run("tells about a broadcast paused and then sent", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP)
		subscribe(d, "broadcast.*")
		id := d.Owner.PostJSON("/api/v1/broadcasts", testkit.JSON{"name": "Hello", "channel": "chat"}).
			AssertCreated().String("data.id")
		path := "/api/v1/broadcasts/" + id
		d.Owner.PatchJSON(path, testkit.JSON{
			"body": "Hi there",
			"rules": testkit.JSON{"rules": testkit.JSON{
				"field": "contact.email", "op": "eq", "value": "no@one.test",
			}},
			"sender_id": models.PublicID(models.PrefixMember, d.TC.Workspace.Owner.ID),
		}).AssertOK()

		d.Owner.PostJSON(path+"/send", nil).AssertOK()
		d.Owner.PostJSON(path+"/pause", nil).AssertOK()
		d.TC.Drain()
		require.Contains(t, received(t, fake), "broadcast.paused")
		d.Owner.PostJSON(path+"/resume", nil).AssertOK()
		for range 4 {
			d.TC.DrainLater()
		}

		got := received(t, fake)
		require.Contains(t, got, "broadcast.sent")
		assert.Equal(t, "paused_by_teammate", got["broadcast.paused"].Data["reason"])
		sameShape(t, got["broadcast.sent"], d.Owner.GetJSON(path).AssertOK())
	})

	t.Run("tells about a series going live and a journey through it ending", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP)
		subscribe(d, "series.*")
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Welcomed"}).AssertCreated().String("data.id")
		spec := testkit.JSON{
			"entry": testkit.JSON{
				"rules": testkit.JSON{"rules": testkit.JSON{
					"field": "contact.custom.plan", "op": "eq", "value": "pro",
				}},
				"reentry": testkit.JSON{"kind": "once"},
			},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "tagged", "kind": "tag", "settings": testkit.JSON{"tag_ids": []string{tag}}},
			},
			"edges": []testkit.JSON{{"from": "entry", "path": "", "to": "tagged"}},
		}
		id := d.Owner.PostJSON("/api/v1/series", testkit.JSON{"name": "Welcome", "subject": "contact", "spec": spec}).
			AssertCreated().String("data.id")
		path := "/api/v1/series/" + id

		d.Owner.PostJSON(path+"/publications", testkit.JSON{"note": "go"}).AssertOK()
		d.TC.Clock.Travel(time.Second)
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{
			"email": "ada@example.org", "custom_attributes": testkit.JSON{"plan": "pro"},
		}).AssertCreated()
		got := map[string]hook{}
		for range 20 {
			d.TC.DrainLater()
			if got = received(t, fake); got["series.journey_completed"].Type != "" {
				break
			}
		}

		require.Contains(t, got, "series.published")
		sameShape(t, got["series.published"], d.Owner.GetJSON(path).AssertOK())
		require.Contains(t, got, "series.journey_completed")
		done := got["series.journey_completed"]
		assert.Equal(t, "finished", done.Data["reason"])
		assert.Equal(t, "series_enrollment", done.object(t)["object"])
		assert.Equal(t, id, done.object(t)["series_id"])
	})
}
