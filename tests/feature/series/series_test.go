package series_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestBuildingSeries(t *testing.T) {
	t.Parallel()

	t.Run("a draft saves while half built and lists what it misses before going live", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		id := d.series("Onboarding", "contact", testkit.JSON{
			"entry": testkit.JSON{"rules": proRules(), "reentry": testkit.JSON{"kind": "once"}},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "welcome", "kind": "email", "settings": testkit.JSON{"body": "Hello"}},
			},
			"edges": []testkit.JSON{edge("entry", "next", "welcome")},
		})

		show := d.owner.GetJSON("/api/v1/series/" + id).AssertOK()
		show.AssertJSONPath("data.status", "draft")
		show.AssertJSONPath("data.problems.0.code", "message_subject_missing")
		show.AssertJSONPath("data.problems.0.node_id", "welcome")

		d.owner.PostJSON("/api/v1/series/"+id+"/publications", testkit.JSON{}).AssertStatus(422)
	})

	t.Run("refuses a loop and a path to a step that does not exist", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		id := d.series("Loop", "contact", testkit.JSON{
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "a", "kind": "wait", "settings": testkit.JSON{"mode": "duration", "amount": 1, "unit": "days"}},
				{"id": "b", "kind": "wait", "settings": testkit.JSON{"mode": "duration", "amount": 1, "unit": "days"}},
			},
			"edges": []testkit.JSON{edge("entry", "", "a"), edge("a", "", "b"), edge("b", "", "a"), edge("b",
				"no_date", "zz")},
		})

		body := string(d.owner.PostJSON("/api/v1/series/"+id+"/check", testkit.JSON{}).AssertOK().Body)
		assert.Contains(t, body, `"loop"`)
		assert.Contains(t, body, `"path_to_unknown"`)
	})

	t.Run("a save over someone else's newer save is refused", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		id := d.series("Trial", "contact", testkit.JSON{})

		d.owner.PatchJSON("/api/v1/series/"+id, testkit.JSON{"lock_version": 1, "name": "Stale"}).AssertStatus(409)
	})

	t.Run("members build series but only admins set them live", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		id := d.series("Members", "contact", testkit.JSON{})

		user := d.tc.CreateUser()
		_, err := factories.Member(t.Context(), d.tc.App, factories.Membership{
			Workspace: d.tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)

		member := d.tc.ActingAs(user)
		member.GetJSON("/api/v1/series/" + id).AssertOK()
		member.PostJSON("/api/v1/series/"+id+"/publications", testkit.JSON{}).AssertForbidden()
	})
}

func TestSeriesJourneys(t *testing.T) {
	t.Parallel()

	t.Run("someone who starts matching enters, gets the email, waits and is tagged", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		tag := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Onboarded"}).AssertCreated().String("data.id")

		id := d.series("Onboarding", "contact", testkit.JSON{
			"entry": testkit.JSON{"rules": proRules(), "reentry": testkit.JSON{"kind": "once"}},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				email("welcome", "Welcome to Acme"),
				{"id": "wait", "kind": "wait", "settings": testkit.JSON{
					"mode": "duration", "amount": 2,
					"unit": "days",
				}},
				{"id": "tagged", "kind": "tag", "settings": testkit.JSON{"tag_ids": []string{tag}}},
			},
			"edges": []testkit.JSON{edge("entry", "", "welcome"), edge("welcome", "", "wait"), edge("wait", "",
				"tagged")},
		})
		d.publish(id)

		ada := unique("ada")
		contact := d.contact(ada, "Ada Okafor", testkit.JSON{"plan": "pro"})
		d.drain()

		assert.Equal(t, 1, mailCount(t, ada))
		d.journeys(id).AssertJSONPath("data.0.status", "waiting").AssertJSONPath("data.0.node_id", "wait")

		d.tc.Clock.Travel(49 * time.Hour)
		d.drain()

		d.journeys(id).AssertJSONPath("data.0.status", "finished").AssertJSONPath("data.0.end_reason", "finished")
		tags := string(d.owner.GetJSON("/api/v1/contacts/" + contact).AssertOK().Body)
		assert.Contains(t, tags, "Onboarded")

		report := d.report(id)
		assert.Equal(t, int64(1), num(report, "data.entered"))
		assert.Equal(t, int64(1), num(report, "data.finished"))
		assert.Equal(t, int64(1), num(report, "data.steps.welcome.paths.next"))
		assert.Equal(t, int64(1), num(report, "data.steps.welcome.message.sent"))
	})

	t.Run("people who matched before going live stay out, and enter when included", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		early := unique("early")
		d.contact(early, "Early Bird", testkit.JSON{"plan": "pro"})
		d.drain()

		spec := func(include bool) testkit.JSON {
			return testkit.JSON{
				"entry": testkit.JSON{
					"rules": proRules(), "include_existing": include,
					"reentry": testkit.JSON{"kind": "once"},
				},
				"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, email("hello", "Hello")},
				"edges": []testkit.JSON{edge("entry", "", "hello")},
			}
		}

		out := d.series("Future only", "contact", spec(false))
		d.publish(out)
		d.tc.Clock.Travel(16 * time.Minute)
		d.owner.GetJSON("/api/v1/series/" + out).AssertOK()
		d.drain()
		assert.Equal(t, int64(0), num(d.report(out), "data.entered"))

		in := d.series("Everyone", "contact", spec(true))
		d.publish(in)
		assert.Equal(t, int64(1), num(d.report(in), "data.entered"))
		assert.Equal(t, 1, mailCount(t, early))
	})

	t.Run("matching the exit rules ends the journey at once", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		id := d.series("Trial ending", "contact", testkit.JSON{
			"entry": testkit.JSON{"rules": proRules(), "reentry": testkit.JSON{"kind": "once"}},
			"exit": testkit.JSON{"rules": testkit.JSON{"rules": testkit.JSON{
				"field": "contact.custom.paid", "op": "eq", "value": "yes",
			}}},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "wait", "kind": "wait", "settings": testkit.JSON{
					"mode": "duration", "amount": 3,
					"unit": "days",
				}},
				email("nudge", "Your trial ends soon"),
			},
			"edges": []testkit.JSON{edge("entry", "", "wait"), edge("wait", "", "nudge")},
		})
		d.publish(id)

		bob := unique("bob")
		contact := d.contact(bob, "Bob", testkit.JSON{"plan": "pro"})
		d.drain()
		d.journeys(id).AssertJSONPath("data.0.status", "waiting")

		d.owner.PatchJSON("/api/v1/contacts/"+contact, testkit.JSON{
			"custom_attributes": testkit.JSON{"plan": "pro", "paid": "yes"},
		}).AssertOK()
		d.drain()

		d.journeys(id).AssertJSONPath("data.0.status", "exited").AssertJSONPath("data.0.end_reason", "exit_rules")
		d.tc.Clock.Travel(4 * 24 * time.Hour)
		d.drain()
		assert.Equal(t, 0, mailCount(t, bob))
	})

	t.Run("a message nobody can receive ends the journey unless there is a could-not-send path", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		tag := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "No email"}).AssertCreated().String("data.id")

		ends := d.series("Ends", "contact", testkit.JSON{
			"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, email("hello", "Hello")},
			"edges": []testkit.JSON{edge("entry", "", "hello")},
		})
		falls := d.series("Falls back", "contact", testkit.JSON{
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				email("hello", "Hello"),
				{"id": "mark", "kind": "tag", "settings": testkit.JSON{"tag_ids": []string{tag}}},
			},
			"edges": []testkit.JSON{edge("entry", "", "hello"), edge("hello", "failed", "mark")},
		})
		d.publish(ends)
		d.publish(falls)

		nobody := d.contact("", "No Email", testkit.JSON{})
		for _, id := range []string{ends, falls} {
			d.owner.PostJSON("/api/v1/series/"+id+"/enrollments", testkit.JSON{"contact_ids": []string{nobody}}).
				AssertCreated()
		}
		d.drain()

		d.journeys(ends).AssertJSONPath("data.0.end_reason", "undeliverable")
		d.journeys(falls).AssertJSONPath("data.0.end_reason", "finished")
		assert.Contains(t, string(d.owner.GetJSON("/api/v1/contacts/"+nobody).AssertOK().Body), "No email")
	})
}

func TestSeriesReplies(t *testing.T) {
	t.Parallel()

	t.Run("a reply opens a conversation and ends the journey when the series stops on replies", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		id := d.series("Replies", "contact", testkit.JSON{
			"exit": testkit.JSON{"on_reply": true},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"}, email("hello", "Hello"), waitDays("wait", 3), email("later", "Later"),
			},
			"edges": []testkit.JSON{edge("entry", "", "hello"), edge("hello", "", "wait"), edge("wait", "", "later")},
		})
		d.publish(id)

		ada := unique("ada")
		d.enroll(id, d.contact(ada, "Ada Okafor", testkit.JSON{}))
		d.journeys(id).AssertJSONPath("data.0.node_id", "wait")

		d.reply(ada, "Can we talk on Thursday?")

		d.journeys(id).AssertJSONPath("data.0.end_reason", "replied")
		assert.Equal(t, int64(1), num(d.report(id), "data.steps.hello.message.replied"))

		var conversations int
		require.NoError(t, d.tc.Operator().QueryRow(t.Context(),
			"SELECT count(*) FROM conversations WHERE workspace_id = $1",
			d.tc.Workspace.Workspace.ID).Scan(&conversations))
		assert.Equal(t, 1, conversations)

		d.tc.Clock.Travel(4 * 24 * time.Hour)
		d.drain()
		assert.Equal(t, 1, mailCount(t, ada))
	})
}

func TestSeriesPacing(t *testing.T) {
	t.Parallel()

	t.Run("an email held back while the address warms up goes out the next day", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t, "DIMMI_BROADCASTS__WARMUP_FIRST_DAY=1")

		id := d.series("Warm-up", "contact", testkit.JSON{
			"nodes": []testkit.JSON{{"id": "entry", "kind": "entry"}, email("hello", "Hello")},
			"edges": []testkit.JSON{edge("entry", "", "hello")},
		})
		d.publish(id)

		first, second := unique("first"), unique("second")
		d.owner.PostJSON("/api/v1/series/"+id+"/enrollments", testkit.JSON{"contact_ids": []string{
			d.contact(first, "First", testkit.JSON{}), d.contact(second, "Second", testkit.JSON{}),
		}}).AssertCreated()
		d.drain()

		assert.Equal(t, 1, mailCount(t, first)+mailCount(t, second))
		assert.Equal(t, int64(1), num(d.report(id), "data.active"))

		d.tc.Clock.Travel(25 * time.Hour)
		d.drain()

		assert.Equal(t, 1, mailCount(t, second))
		assert.Equal(t, int64(2), num(d.report(id), "data.finished"))
	})
}

func TestSeriesEventsAndGoals(t *testing.T) {
	t.Parallel()

	t.Run("an event starts the series once per event, and the goal ends it", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		id := d.series("Activation", "contact", testkit.JSON{
			"entry": testkit.JSON{"event": "signed_up", "reentry": testkit.JSON{"kind": "again"}},
			"exit":  testkit.JSON{"on_goal": true},
			"goal":  testkit.JSON{"event": "installed", "days": 7},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "wait", "kind": "wait_for", "settings": testkit.JSON{
					"event": "invited", "amount": 3, "unit": "days",
				}},
			},
			"edges": []testkit.JSON{edge("entry", "", "wait")},
		})
		d.publish(id)

		cara := unique("cara")
		contact := d.contact(cara, "Cara", testkit.JSON{})
		track := func(name, key string) {
			d.owner.PostJSON("/api/v1/contact-events", testkit.JSON{
				"name": name, "contact_id": contact, "idempotency_key": key,
			}).AssertOK()
			d.drain()
		}

		track("signed_up", "k1")
		track("signed_up", "k1")
		assert.Equal(t, int64(1), num(d.report(id), "data.entered"))
		d.journeys(id).AssertJSONPath("data.0.status", "waiting")

		track("installed", "k2")
		d.journeys(id).AssertJSONPath("data.0.end_reason", "goal")
		assert.Equal(t, int64(1), num(d.report(id), "data.goal"))
	})

	t.Run("a wait for an event takes the matched path when it happens and times out otherwise", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		yes := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Invited"}).AssertCreated().String("data.id")
		no := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Quiet"}).AssertCreated().String("data.id")

		id := d.series("Invite", "contact", testkit.JSON{
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "wait", "kind": "wait_for", "settings": testkit.JSON{
					"event": "invited", "amount": 2,
					"unit": "days",
				}},
				{"id": "yes", "kind": "tag", "settings": testkit.JSON{"tag_ids": []string{yes}}},
				{"id": "no", "kind": "tag", "settings": testkit.JSON{"tag_ids": []string{no}}},
			},
			"edges": []testkit.JSON{edge("entry", "", "wait"), edge("wait", "matched", "yes"), edge("wait",
				"timed_out", "no")},
		})
		d.publish(id)

		active := d.contact(unique("active"), "Active", testkit.JSON{})
		quiet := d.contact(unique("quiet"), "Quiet", testkit.JSON{})
		d.owner.PostJSON("/api/v1/series/"+id+"/enrollments", testkit.JSON{"contact_ids": []string{active, quiet}}).
			AssertCreated()
		d.drain()

		d.owner.PostJSON("/api/v1/contact-events", testkit.JSON{"name": "invited", "contact_id": active}).AssertOK()
		d.drain()
		assert.Contains(t, string(d.owner.GetJSON("/api/v1/contacts/"+active).AssertOK().Body), "Invited")

		d.tc.Clock.Travel(49 * time.Hour)
		d.drain()
		assert.Contains(t, string(d.owner.GetJSON("/api/v1/contacts/"+quiet).AssertOK().Body), "Quiet")
		require.Equal(t, int64(2), num(d.report(id), "data.finished"))
	})
}
