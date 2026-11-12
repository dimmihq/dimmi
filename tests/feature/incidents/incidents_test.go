package incidents_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

func TestSpikeDetection(t *testing.T) {
	t.Parallel()

	t.Run("suggests an incident at the threshold and grows it while the spike lasts", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		heard := tests.Record[appevents.IncidentSuggested](d.TC)
		d.customers(t, 8, "Cannot sign in")

		d.detect(t)

		list := d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK()
		require.Equal(t, 1, num(list))
		list.AssertJSONPath("data.0.title", "Failed sign-ins").AssertJSONPath("data.0.area", "sign-in").
			AssertJSONPath("data.0.conversation_count", float64(8)).AssertJSONPath("data.0.status", "open")
		first := list.String("data.0.id")

		d.TC.Clock.Travel(time.Minute)
		d.customers(t, 2, "Login broken")
		d.detect(t)

		grown := d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK()
		require.Equal(t, 1, num(grown))
		grown.AssertJSONPath("data.0.id", first).AssertJSONPath("data.0.conversation_count", float64(10))
		assert.Len(t, heard.All(), 2)
	})

	t.Run("stays quiet below the threshold without asking the model", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.customers(t, 7, "Cannot sign in")

		d.detect(t)

		assert.Equal(t, 0, num(d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK()))
		assert.Equal(t, 0, d.ai.groups)
	})

	t.Run("accepting declares the incident with its area and every conversation linked", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.customers(t, 8, "Cannot sign in")
		d.detect(t)
		id := d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK().String("data.0.id")

		res := d.Owner.PatchJSON("/api/v1/incident-suggestions/"+id, testkit.JSON{"status": "accepted"}).AssertOK()
		res.AssertJSONPath("data.status", "accepted")
		incident := res.String("data.ticket_id")
		require.NotEmpty(t, incident)

		show := d.Owner.GetJSON("/api/v1/tickets/" + incident).AssertOK()
		show.AssertJSONPath("data.kind", "incident").AssertJSONPath("data.title", "Failed sign-ins").
			AssertJSONPath("data.fields.areas.0", "Sign-in")
		reach := d.Owner.GetJSON("/api/v1/tickets/" + incident + "/conversations").AssertOK()
		reach.AssertJSONPath("meta.impact.conversations", float64(8))

		d.Owner.PatchJSON("/api/v1/incident-suggestions/"+id, testkit.JSON{"status": "dismissed"}).
			AssertStatus(fhttp.StatusConflict)
		assert.Equal(t, 0, num(d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK()))
	})

	t.Run("dismissing mutes the group for an hour", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.customers(t, 8, "Cannot sign in")
		d.detect(t)
		id := d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK().String("data.0.id")

		d.Owner.PatchJSON("/api/v1/incident-suggestions/"+id, testkit.JSON{"status": "dismissed"}).AssertOK().
			AssertJSONPath("data.status", "dismissed")

		d.TC.Clock.Travel(5 * time.Minute)
		d.customers(t, 3, "Still cannot sign in")
		d.detect(t)
		assert.Equal(t, 0, num(d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK()))

		d.TC.Clock.Travel(time.Hour)
		d.customers(t, 8, "Sign in down again")
		d.detect(t)
		assert.Equal(t, 1, num(d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK()))
	})

	t.Run("links a spike about an active incident to it instead of suggesting", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		incident := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Incident"], "title": "Sign-in outage",
		}).AssertCreated().String("data.id")
		d.ai.answer(uuid(t, d, incident))

		d.customers(t, 8, "Cannot sign in")
		d.detect(t)

		assert.Equal(t, 0, num(d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK()))
		d.Owner.GetJSON("/api/v1/tickets/"+incident+"/conversations").AssertOK().
			AssertJSONPath("meta.impact.conversations", float64(8))
	})

	t.Run("refuses members who may not work tickets and unknown statuses", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.customers(t, 8, "Cannot sign in")
		d.detect(t)
		id := d.Owner.GetJSON("/api/v1/incident-suggestions").AssertOK().String("data.0.id")

		d.Owner.PatchJSON("/api/v1/incident-suggestions/"+id, testkit.JSON{"status": "maybe"}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
		d.Owner.PatchJSON("/api/v1/incident-suggestions/incs_nope", testkit.JSON{"status": "accepted"}).
			AssertStatus(fhttp.StatusNotFound)
	})
}

func TestActiveIncidents(t *testing.T) {
	t.Parallel()

	t.Run("links a new matching conversation and tells the customer the latest at once", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		incident := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Incident"], "title": "Sign-in outage",
		}).AssertCreated().String("data.id")
		d.Owner.PostJSON("/api/v1/tickets/"+incident+"/public-updates", testkit.JSON{
			"body": "A fix is rolling out.",
		}).AssertCreated()
		d.ai.answer(uuid(t, d, incident))

		conv := d.customers(t, 1, "Cannot sign in")[0]
		d.TC.Drain()

		d.Owner.GetJSON("/api/v1/conversations/"+conv+"/tickets").AssertOK().
			AssertJSONPath("data.trackers.0.id", incident)

		msgs := d.Owner.GetJSON("/api/v1/conversations/" + conv + "/messages").AssertOK()
		assert.Contains(t, string(msgs.Body), "A fix is rolling out.")
		assert.Contains(t, string(msgs.Body), "Sign-in outage")
	})

	t.Run("leaves a conversation the model does not match alone", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		incident := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Incident"], "title": "Sign-in outage",
		}).AssertCreated().String("data.id")

		d.customers(t, 1, "How do I change my plan?")
		d.TC.Drain()

		d.Owner.GetJSON("/api/v1/tickets/"+incident+"/conversations").AssertOK().
			AssertJSONPath("meta.impact.conversations", float64(0))
	})
}

func TestTrackerDuplicates(t *testing.T) {
	t.Parallel()

	t.Run("suggests the tracker a new conversation reports", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		tracker := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Tracker"], "title": "CSV export crashes",
		}).AssertCreated()
		d.ai.answer(uuid(t, d, tracker.String("data.id")))

		conv := d.customers(t, 1, "Export crashes")[0]
		d.TC.Drain()

		res := d.Owner.GetJSON("/api/v1/conversations/" + conv + "/tracker-suggestions").AssertOK()
		require.Equal(t, 1, num(res))
		res.AssertJSONPath("data.0.ticket_id", tracker.String("data.id")).
			AssertJSONPath("data.0.reference", tracker.String("data.reference")).
			AssertJSONPath("data.0.title", "CSV export crashes").AssertJSONPath("data.0.linked_count", float64(0))

		d.Owner.PostJSON("/api/v1/tickets/"+tracker.String("data.id")+"/links", testkit.JSON{
			"kind": "tracker", "conversation_id": conv,
		}).AssertOK()
		assert.Equal(t, 0, num(d.Owner.GetJSON("/api/v1/conversations/"+conv+"/tracker-suggestions")))
	})

	t.Run("links the tracker at once when the workspace says so", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		tracker := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Tracker"], "title": "CSV export crashes",
		}).AssertCreated().String("data.id")
		d.ai.answer(uuid(t, d, tracker))
		_, err := d.TC.Operator().Exec(t.Context(),
			`UPDATE workspaces SET ticket_settings = '{"auto_link_duplicates": true}' WHERE id = $1`,
			d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)

		conv := d.customers(t, 1, "Export crashes")[0]
		d.TC.Drain()

		d.Owner.GetJSON("/api/v1/conversations/"+conv+"/tickets").AssertOK().
			AssertJSONPath("data.trackers.0.id", tracker)
		assert.Equal(t, 0, num(d.Owner.GetJSON("/api/v1/conversations/"+conv+"/tracker-suggestions")))
	})

	t.Run("summarizes what a tracker's customers report", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		tracker := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Tracker"], "title": "CSV export crashes",
		}).AssertCreated().String("data.id")

		for _, conv := range d.customers(t, 3, "Export crashes") {
			d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/links", testkit.JSON{
				"kind": "tracker", "conversation_id": conv,
			}).AssertOK()
		}

		res := d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/summary").AssertOK()
		res.AssertJSONPath("data.object", "ticket_summary").AssertJSONPath("data.reports", float64(3)).
			AssertJSONPath("data.summary", "Customers cannot export CSV files larger than 10 MB.").
			AssertJSONPath("data.plans.0.count", float64(3))

		d.Owner.GetJSON("/api/v1/tickets/" + d.customerTicket(t) + "/summary").
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})
}

/*
customerTicket makes a customer ticket and returns its id.
*/
func (d *desk) customerTicket(t *testing.T) string {
	t.Helper()

	conv := d.customers(t, 1, "Question")[0]

	return d.Owner.PostJSON("/api/v1/conversations/"+conv+"/ticket", testkit.JSON{"type_id": d.types["Question"]}).
		AssertCreated().String("data.id")
}
