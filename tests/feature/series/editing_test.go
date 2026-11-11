package series_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
waitDays is a wait step of some days.
*/
func waitDays(id string, n int) testkit.JSON {
	return testkit.JSON{"id": id, "kind": "wait", "settings": testkit.JSON{
		"mode": "duration", "amount": n,
		"unit": "days",
	}}
}

/*
tagStep tags the traveler.
*/
func tagStep(id, tag string) testkit.JSON {
	return testkit.JSON{"id": id, "kind": "tag", "settings": testkit.JSON{"tag_ids": []string{tag}}}
}

/*
enroll adds contacts to a series and runs what it queues.
*/
func (d desk) enroll(series string, contacts ...string) {
	d.tc.T.Helper()

	d.owner.PostJSON("/api/v1/series/"+series+"/enrollments", testkit.JSON{"contact_ids": contacts}).AssertCreated()
	d.drain()
}

/*
save replaces the working copy of a series.
*/
func (d desk) save(id string, spec testkit.JSON) {
	d.tc.T.Helper()

	show := d.owner.GetJSON("/api/v1/series/" + id).AssertOK()
	d.owner.PatchJSON("/api/v1/series/"+id, testkit.JSON{
		"lock_version": num(show, "data.lock_version"), "name": show.String("data.name"), "spec": spec,
		"email_address_id": d.addressID, "sender_id": d.member,
	}).AssertOK()
}

func TestSeriesPaths(t *testing.T) {
	t.Parallel()

	t.Run("a check sends each traveler down the first path they match", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		pro := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Pro path"}).AssertCreated().String("data.id")
		other := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Other path"}).AssertCreated().String("data.id")

		id := d.series("Check", "contact", testkit.JSON{
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "plan", "kind": "condition", "settings": testkit.JSON{"paths": []testkit.JSON{
					{"id": "pro", "label": "Pro", "rules": proRules()},
				}}},
				tagStep("tag-pro", pro), tagStep("tag-other", other),
			},
			"edges": []testkit.JSON{
				edge("entry", "", "plan"), edge("plan", "pro", "tag-pro"), edge("plan", "else", "tag-other"),
			},
		})
		d.publish(id)

		a := d.contact(unique("a"), "A", testkit.JSON{"plan": "pro"})
		b := d.contact(unique("b"), "B", testkit.JSON{"plan": "free"})
		d.enroll(id, a, b)

		assert.Contains(t, string(d.owner.GetJSON("/api/v1/contacts/"+a).AssertOK().Body), "Pro path")
		assert.Contains(t, string(d.owner.GetJSON("/api/v1/contacts/"+b).AssertOK().Body), "Other path")

		report := d.report(id)
		assert.Equal(t, int64(1), num(report, "data.steps.plan.paths.pro"))
		assert.Equal(t, int64(1), num(report, "data.steps.plan.paths.else"))
	})

	t.Run("a split shares travelers by weight and sends everyone down the winner once picked", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		id := d.series("Split", "contact", testkit.JSON{
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "ab", "kind": "split", "settings": testkit.JSON{"variants": []testkit.JSON{
					{"id": "a", "label": "A", "weight": 50}, {"id": "b", "label": "B", "weight": 50},
				}}},
			},
			"edges": []testkit.JSON{edge("entry", "", "ab")},
		})
		d.publish(id)

		people := make([]string, 0, 20)
		for i := range 20 {
			people = append(people, d.contact(unique("p"), "Person", testkit.JSON{"n": i}))
		}
		d.enroll(id, people...)

		report := d.report(id)
		a, b := num(report, "data.steps.ab.paths.a"), num(report, "data.steps.ab.paths.b")
		assert.Equal(t, int64(20), a+b)
		assert.Positive(t, a)
		assert.Positive(t, b)

		d.owner.PostJSON("/api/v1/series/"+id+"/nodes/ab/winner", testkit.JSON{"path": "b"}).AssertOK()
		late := d.contact(unique("late"), "Late", testkit.JSON{})
		d.enroll(id, late)
		assert.Equal(t, b+1, num(d.report(id), "data.steps.ab.paths.b"))
	})
}

func TestEditingLiveSeries(t *testing.T) {
	t.Parallel()

	t.Run("publishing again moves people on a removed step to the next step that remains", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		done := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Done"}).AssertCreated().String("data.id")

		id := d.series("Edit", "contact", testkit.JSON{
			"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, waitDays("long", 10), tagStep("done", done)},
			"edges": []testkit.JSON{edge("entry", "", "long"), edge("long", "", "done")},
		})
		d.publish(id)

		person := d.contact(unique("edit"), "Edit", testkit.JSON{})
		d.enroll(id, person)
		d.journeys(id).AssertJSONPath("data.0.node_id", "long")

		d.save(id, testkit.JSON{
			"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, tagStep("done", done)},
			"edges": []testkit.JSON{edge("entry", "", "done")},
		})
		d.publish(id)

		d.journeys(id).AssertJSONPath("data.0.status", "finished")
		assert.Contains(t, string(d.owner.GetJSON("/api/v1/contacts/"+person).AssertOK().Body), "Done")
		d.owner.GetJSON("/api/v1/series/"+id).AssertOK().AssertJSONPath("data.has_unpublished_changes", false)

		versions := d.owner.GetJSON("/api/v1/series/" + id + "/versions").AssertOK()
		assert.Equal(t, int64(2), num(versions, "data.0.number"))
	})

	t.Run("a shorter wait counts from when people arrived", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		done := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Waited"}).AssertCreated().String("data.id")

		spec := func(days int) testkit.JSON {
			return testkit.JSON{
				"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, waitDays("wait", days), tagStep("done",
					done)},
				"edges": []testkit.JSON{edge("entry", "", "wait"), edge("wait", "", "done")},
			}
		}

		id := d.series("Shorter", "contact", spec(10))
		d.publish(id)
		person := d.contact(unique("short"), "Short", testkit.JSON{})
		d.enroll(id, person)

		d.save(id, spec(2))
		d.publish(id)
		d.tc.Clock.Travel(3 * 24 * time.Hour)
		d.drain()

		d.journeys(id).AssertJSONPath("data.0.status", "finished")
	})

	t.Run("pausing holds every journey and resuming moves on those whose wait ended", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		done := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Resumed"}).AssertCreated().String("data.id")

		id := d.series("Pause", "contact", testkit.JSON{
			"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, waitDays("wait", 1), tagStep("done", done)},
			"edges": []testkit.JSON{edge("entry", "", "wait"), edge("wait", "", "done")},
		})
		d.publish(id)
		person := d.contact(unique("pause"), "Pause", testkit.JSON{})
		d.enroll(id, person)

		d.owner.PostJSON("/api/v1/series/"+id+"/pause", testkit.JSON{}).AssertOK().AssertJSONPath("data.status",
			"paused")
		d.tc.Clock.Travel(2 * 24 * time.Hour)
		d.drain()
		d.journeys(id).AssertJSONPath("data.0.status", "waiting")

		d.owner.PostJSON("/api/v1/series/"+id+"/resume", testkit.JSON{}).AssertOK().AssertJSONPath("data.status",
			"live")
		d.drain()
		d.journeys(id).AssertJSONPath("data.0.status", "finished")
	})

	t.Run("the people on a step can be found by name or email", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		id := d.series("Find", "contact", testkit.JSON{
			"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, waitDays("wait", 3)},
			"edges": []testkit.JSON{edge("entry", "", "wait")},
		})
		d.publish(id)
		ada := unique("ada")
		d.enroll(id, d.contact(ada, "Ada Lovelace", testkit.JSON{}))
		d.enroll(id, d.contact(unique("grace"), "Grace Hopper", testkit.JSON{}))
		d.drain()

		list := "/api/v1/series/" + id + "/enrollments?node=wait&q="
		d.owner.GetJSON(list+"hopp").AssertOK().AssertJSONPath("data.0.contact_name", "Grace Hopper").
			AssertJSONMissing("data.1")
		d.owner.GetJSON(list+ada[:8]).AssertOK().AssertJSONPath("data.0.contact_name", "Ada Lovelace").
			AssertJSONMissing("data.1")
		d.owner.GetJSON(list + "nobody").AssertOK().AssertJSONMissing("data.0")
	})

	t.Run("archiving ends every journey and keeps the report", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		id := d.series("Archive", "contact", testkit.JSON{
			"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, waitDays("wait", 5)},
			"edges": []testkit.JSON{edge("entry", "", "wait")},
		})
		d.publish(id)
		d.enroll(id, d.contact(unique("arch"), "Arch", testkit.JSON{}))

		d.owner.PostJSON("/api/v1/series/"+id+"/archive", testkit.JSON{}).AssertOK()
		d.journeys(id).AssertJSONPath("data.0.end_reason", "series_stopped")
		assert.Equal(t, int64(1), num(d.report(id), "data.exited"))
		d.owner.Delete("/api/v1/series/" + id).AssertStatus(409)
	})
}

func TestAccountSeries(t *testing.T) {
	t.Parallel()

	t.Run("a company enrolls once and only the people the step picks get the email", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		company := d.owner.PostJSON("/api/v1/companies", testkit.JSON{
			"name": "Globex", "custom_attributes": testkit.JSON{"plan": "enterprise"},
		}).AssertCreated().String("data.id")

		admin, member := unique("admin"), unique("member")
		for _, p := range []struct{ email, role string }{{admin, "admin"}, {member, "member"}} {
			c := d.contact(p.email, "Person", testkit.JSON{"role": p.role})
			d.owner.PatchJSON("/api/v1/contacts/"+c, testkit.JSON{"company_id": company}).AssertOK()
		}

		id := d.series("Renewal", "company", testkit.JSON{
			"entry": testkit.JSON{"rules": testkit.JSON{"rules": testkit.JSON{
				"field": "company.custom.plan", "op": "eq", "value": "enterprise",
			}}, "include_existing": true, "reentry": testkit.JSON{"kind": "once"}},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "renewal", "kind": "email", "settings": testkit.JSON{
					"subject": "Your renewal", "body": "Hi {{ contact.first_name }}, your plan renews soon.",
					"recipients": testkit.JSON{"rules": testkit.JSON{
						"field": "contact.custom.role", "op": "eq",
						"value": "admin",
					}},
				}},
			},
			"edges": []testkit.JSON{edge("entry", "", "renewal")},
		})
		d.publish(id)

		report := d.report(id)
		assert.Equal(t, int64(1), num(report, "data.entered"))
		assert.Equal(t, int64(1), num(report, "data.finished"))
		assert.Equal(t, 1, mailCount(t, admin))
		assert.Equal(t, 0, mailCount(t, member))

		journeys := d.owner.GetJSON("/api/v1/companies/" + company + "/series").AssertOK()
		journeys.AssertJSONPath("data.0.series_name", "Renewal")
	})
}

func TestTryingSeries(t *testing.T) {
	t.Parallel()

	t.Run("try it on me runs a test journey whose waits can be skipped and that never counts", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		ownerEmail := d.owner.GetJSON("/api/v1/user").AssertOK().String("data.email")
		d.contact(ownerEmail, "Me", testkit.JSON{})

		id := d.series("Try", "contact", testkit.JSON{
			"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, waitDays("wait", 3), email("hello", "Hello me")},
			"edges": []testkit.JSON{edge("entry", "", "wait"), edge("wait", "", "hello")},
		})
		d.publish(id)

		journey := d.owner.PostJSON("/api/v1/series/"+id+"/tests", testkit.JSON{}).AssertCreated().String("data.id")
		d.drain()
		d.owner.GetJSON("/api/v1/series/"+id+"/enrollments/"+journey).AssertOK().AssertJSONPath("data.status",
			"waiting")

		d.owner.PostJSON("/api/v1/series/"+id+"/enrollments/"+journey+"/skip", testkit.JSON{}).AssertNoContent()
		d.drain()

		trace := d.owner.GetJSON("/api/v1/series/" + id + "/enrollments/" + journey).AssertOK()
		trace.AssertJSONPath("data.status", "finished")
		trace.AssertJSONPath("data.steps.2.node_id", "hello")
		require.Equal(t, 1, mailCount(t, ownerEmail))
		assert.Equal(t, int64(0), num(d.report(id), "data.entered"))

		again := d.owner.PostJSON("/api/v1/series/"+id+"/tests", testkit.JSON{}).AssertCreated().String("data.id")
		assert.NotEqual(t, journey, again)
		d.owner.PostJSON("/api/v1/series/"+id+"/tests", testkit.JSON{}).AssertCreated()
		replaced := d.owner.GetJSON("/api/v1/series/" + id + "/enrollments/" + again).AssertOK()
		replaced.AssertJSONPath("data.end_reason", "replaced")
	})
}
