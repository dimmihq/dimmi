package widget_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
sendEvents posts a batch from the visitor's messenger.
*/
func (v *visitor) sendEvents(events ...testkit.JSON) *testkit.Response {
	return v.client().PostJSON("/widget/v1/events", testkit.JSON{"events": events})
}

/*
serverEvent records an event of the name through the server API, which
defines the name, closed to the messenger.
*/
func serverEvent(tc *tests.TestCase, name, contact string) {
	owner(tc).PostJSON("/api/v1/contact-events", testkit.JSON{"name": name, "contact_id": contact}).AssertOK()
}

/*
eventID returns the id of the workspace's only event name.
*/
func eventID(tc *tests.TestCase) string {
	return owner(tc).GetJSON("/api/v1/event-definitions").AssertOK().String("data.0.id")
}

func TestMessengerEvents(t *testing.T) {
	t.Parallel()

	t.Run("records only the names opened to the messenger", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		serverEvent(tc, "plan.upgraded", v.contact)

		v.sendEvents(testkit.JSON{"name": "plan.upgraded"}, testkit.JSON{"name": "never.seen"}).AssertOK().
			AssertJSONPath("data.accepted", float64(0)).
			AssertJSONPath("data.events.0.status", "not_allowed").
			AssertJSONPath("data.events.1.status", "not_allowed")

		owner(tc).PatchJSON("/api/v1/event-definitions/"+eventID(tc), testkit.JSON{"client_writable": true}).
			AssertOK().AssertJSONPath("data.client_writable", true)
		v.sendEvents(testkit.JSON{"name": "plan.upgraded", "metadata": testkit.JSON{"plan": "pro"}}).AssertOK().
			AssertJSONPath("data.accepted", float64(1)).AssertJSONPath("data.events.0.status", "accepted")

		owner(tc).GetJSON("/api/v1/contacts/"+v.contact+"/event-summaries").AssertOK().
			AssertJSONPath("data.0.count", float64(2)).AssertJSONPath("data.0.last_metadata.plan", "pro")
		owner(tc).GetJSON("/api/v1/event-definitions").AssertOK().AssertJSONPath("data.#", float64(1))
	})

	t.Run("defines new names when the workspace lets the messenger send any", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		owner(tc).GetJSON("/api/v1/event-settings").AssertOK().AssertJSONPath("data.messenger_new_events", false)
		owner(tc).PatchJSON("/api/v1/event-settings", testkit.JSON{"messenger_new_events": true}).AssertOK().
			AssertJSONPath("data.messenger_new_events", true)

		v.sendEvents(testkit.JSON{"name": "video.played"}).AssertOK().AssertJSONPath("data.accepted", float64(1))
		owner(tc).GetJSON("/api/v1/event-definitions").AssertOK().
			AssertJSONPath("data.0.name", "video.played").AssertJSONPath("data.0.client_writable", true)

		owner(tc).PatchJSON("/api/v1/event-settings", testkit.JSON{"messenger_new_events": false}).AssertOK()
		v.sendEvents(testkit.JSON{"name": "video.played"}, testkit.JSON{"name": "other"}).AssertOK().
			AssertJSONPath("data.events.0.status", "accepted").AssertJSONPath("data.events.1.status", "not_allowed")
	})

	t.Run("counts a retried event once and refuses what it cannot record", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		owner(tc).PatchJSON("/api/v1/event-settings", testkit.JSON{"messenger_new_events": true}).AssertOK()

		v.sendEvents(testkit.JSON{"name": "clicked", "id": "e1"}).AssertOK().AssertJSONPath("data.accepted", float64(1))
		v.sendEvents(testkit.JSON{"name": "clicked", "id": "e1"}).AssertOK().
			AssertJSONPath("data.events.0.status", "duplicate")
		v.sendEvents(testkit.JSON{"name": "Clicked"}).AssertStatus(422)
		v.sendEvents(testkit.JSON{"name": "x", "metadata": testkit.JSON{"deep": testkit.JSON{}}}).AssertStatus(422)

		batch := make([]testkit.JSON, 21)
		for i := range batch {
			batch[i] = testkit.JSON{"name": "clicked"}
		}
		v.sendEvents(batch...).AssertStatus(422)
		tc.Client().PostJSON("/widget/v1/events", testkit.JSON{"events": []testkit.JSON{{"name": "x"}}}).
			AssertStatus(401)
	})

	t.Run("lets only managers open names to the messenger", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		serverEvent(tc, "x", v.contact)
		_, member := memberClient(t, tc, "member")

		member.GetJSON("/api/v1/event-settings").AssertOK()
		member.PatchJSON("/api/v1/event-settings", testkit.JSON{"messenger_new_events": true}).AssertStatus(403)
		member.PatchJSON("/api/v1/event-definitions/"+eventID(tc), testkit.JSON{"client_writable": true}).
			AssertStatus(403)
	})
}
