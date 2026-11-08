package events_test

import (
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
contact adds a contact and returns its id.
*/
func contact(d tests.Desk, email string, fields testkit.JSON) string {
	body := testkit.JSON{"email": email}
	maps.Copy(body, fields)

	return d.Owner.PostJSON("/api/v1/contacts", body).AssertCreated().String("data.id")
}

/*
track sends one event and returns the response.
*/
func track(d tests.Desk, event testkit.JSON) *testkit.Response {
	return d.Owner.PostJSON("/api/v1/contact-events", event)
}

func TestTracking(t *testing.T) {
	t.Parallel()

	t.Run("records an event, defines its name and sums it up for the person", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", nil)

		track(d, testkit.JSON{"name": "report.exported", "contact_id": ana, "metadata": testkit.JSON{"format": "csv"}}).
			AssertOK().AssertJSONPath("data.accepted", float64(1)).AssertJSONPath("data.events.0.contact_id", ana)
		track(d, testkit.JSON{"name": "report.exported", "email": "ANA@acme.dev"}).AssertOK()

		d.Owner.GetJSON("/api/v1/event-definitions").AssertOK().AssertJSONPath("data.0.name", "report.exported").
			AssertJSONPath("data.0.label", "Report exported").AssertJSONPath("data.0.total_count", float64(2)).
			AssertJSONPath("data.0.origin", "observed")
		d.Owner.GetJSON("/api/v1/contacts/"+ana+"/event-summaries").AssertOK().
			AssertJSONPath("data.0.count", float64(2)).AssertJSONPath("data.0.last_7_days", float64(2)).
			AssertJSONPath("data.0.last_metadata", map[string]any{})
	})

	t.Run("records a batch by external id, reporting people it cannot find", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(d, "bo@acme.dev", testkit.JSON{"external_id": "u_1"})

		track(d, testkit.JSON{"events": []testkit.JSON{
			{"name": "signed_in", "external_id": "u_1"},
			{"name": "signed_in", "external_id": "u_404"},
		}}).AssertOK().AssertJSONPath("data.accepted", float64(1)).
			AssertJSONPath("data.events.1.status", "contact_not_found")
		track(d, testkit.JSON{"name": "signed_in", "external_id": "u_404"}).AssertStatus(404)
	})

	t.Run("counts a retried event once", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", nil)
		event := testkit.JSON{"name": "paid", "contact_id": ana, "idempotency_key": "inv_1"}

		track(d, event).AssertOK().AssertJSONPath("data.events.0.status", "accepted")
		track(d, event).AssertOK().AssertJSONPath("data.events.0.status", "duplicate")
		d.Owner.GetJSON("/api/v1/contacts/"+ana+"/event-summaries").AssertOK().
			AssertJSONPath("data.0.count", float64(1))
	})

	t.Run("refuses names, people and details it cannot record", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", nil)

		track(d, testkit.JSON{"name": "Report", "contact_id": ana}).AssertStatus(422).
			AssertJSONPath("errors.0.message", "Event names are lowercase: send report.")
		track(d, testkit.JSON{"name": "x", "contact_id": ana, "email": "ana@acme.dev"}).AssertStatus(422)
		deep := testkit.JSON{"deep": testkit.JSON{}}
		track(d, testkit.JSON{"name": "x", "contact_id": ana, "metadata": deep}).
			AssertStatus(422).AssertJSONPath("errors.0.field", "metadata")
	})

	t.Run("moves times out of range and stops archived names", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", nil)
		future := d.TC.Clock.Now().Add(3 * time.Hour).Format(time.RFC3339)

		track(d, testkit.JSON{"name": "x", "contact_id": ana, "occurred_at": future}).AssertOK().
			AssertJSONPath("data.events.0.occurred_at_clamped", true)

		id := d.Owner.GetJSON("/api/v1/event-definitions").AssertOK().String("data.0.id")
		d.Owner.PatchJSON("/api/v1/event-definitions/"+id, testkit.JSON{"status": "archived"}).AssertOK()
		track(d, testkit.JSON{"name": "x", "contact_id": ana}).AssertOK().
			AssertJSONPath("data.events.0.status", "archived")
	})

	t.Run("lets members read events but not change their names", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", nil)
		track(d, testkit.JSON{"name": "x", "contact_id": ana}).AssertOK()
		id := d.Owner.GetJSON("/api/v1/event-definitions").AssertOK().String("data.0.id")
		member, _ := d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/event-definitions").AssertOK()
		member.PatchJSON("/api/v1/event-definitions/"+id, testkit.JSON{"label": "X"}).AssertStatus(403)
	})
}

func TestEventRules(t *testing.T) {
	t.Parallel()

	t.Run("matches people by how often they did something lately", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", nil)
		contact(d, "bo@acme.dev", nil)
		old := d.TC.Clock.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
		track(d, testkit.JSON{"events": []testkit.JSON{
			{"name": "report.exported", "contact_id": ana},
			{"name": "report.exported", "contact_id": ana},
			{"name": "report.exported", "contact_id": ana, "occurred_at": old},
		}}).AssertOK()

		preview := func(rule testkit.JSON) *testkit.Response {
			return d.Owner.PostJSON("/api/v1/audience-previews", testkit.JSON{"rules": testkit.JSON{"rules": rule}}).
				AssertOK()
		}

		preview(testkit.JSON{"field": "event.report.exported.count", "op": "gte", "value": 2, "within_days": 7}).
			AssertJSONPath("data.count", float64(1)).AssertJSONPath("data.sample.0.email", "ana@acme.dev")
		preview(testkit.JSON{"field": "event.report.exported.count", "op": "gte", "value": 3, "within_days": 7}).
			AssertJSONPath("data.count", float64(0))
		preview(testkit.JSON{"field": "event.report.exported.count", "op": "eq", "value": 0}).
			AssertJSONPath("data.count", float64(1)).AssertJSONPath("data.sample.0.email", "bo@acme.dev")
		preview(testkit.JSON{"field": "event.report.exported.last_at", "op": "within_last_days", "value": 1}).
			AssertJSONPath("data.count", float64(1))
	})

	t.Run("offers event fields to the builder", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", nil)
		track(d, testkit.JSON{"name": "invited", "contact_id": ana}).AssertOK()

		var page struct {
			Data []struct {
				Key    string `json:"key"`
				Label  string `json:"label"`
				Window bool   `json:"window"`
			} `json:"data"`
		}
		d.Owner.GetJSON("/api/v1/audience-fields").AssertOK().Decode(&page)

		labels := map[string]string{}
		for _, f := range page.Data {
			labels[f.Key] = f.Label
			if f.Key == "event.invited.count" {
				assert.True(t, f.Window)
			}
		}

		assert.Equal(t, "Invited: times", labels["event.invited.count"])
		assert.Equal(t, "Invited: last time", labels["event.invited.last_at"])
	})

	t.Run("runs a workflow for people who did something", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", nil)
		track(d, testkit.JSON{"name": "upgraded", "contact_id": ana}).AssertOK()
		wf := d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Upgraded", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"audience": testkit.JSON{"rules": testkit.JSON{"field": "event.upgraded.count", "op": "gte", "value": 1}},
			"steps": []testkit.JSON{
				{"id": "s_note0001", "kind": "note", "settings": testkit.JSON{"text": "upgraded customer"}},
			},
		}}).AssertCreated().String("data.id")
		d.Owner.PostJSON("/api/v1/workflows/"+wf+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertOK()

		notes := func(email string) []string {
			id := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
				"inbox_id": d.InboxID(), "author": "contact", "subject": "Hi", "body": "Hi",
				"contact": testkit.JSON{"email": email},
			}).AssertCreated().String("data.id")
			d.TC.Drain()

			var page struct {
				Data []struct {
					Body *string `json:"body"`
				} `json:"data"`
			}
			d.Owner.GetJSON("/api/v1/conversations/" + id + "/messages?order=asc").AssertOK().Decode(&page)
			var out []string
			for _, m := range page.Data {
				if m.Body != nil {
					out = append(out, *m.Body)
				}
			}

			return out
		}

		assert.Contains(t, notes("ana@acme.dev"), "upgraded customer")
		assert.NotContains(t, notes("bo@acme.dev"), "upgraded customer")
	})
}
