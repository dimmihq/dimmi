package tickets_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
desk is a workspace with its default ticket setup read once.
*/
type desk struct {
	tests.Desk
	types map[string]string
}

/*
newDesk starts a workspace and reads its default ticket types by name.
*/
func newDesk(t *testing.T) desk {
	t.Helper()

	d := desk{Desk: tests.NewDesk(t), types: map[string]string{}}
	res := d.Owner.GetJSON("/api/v1/ticket-types").AssertOK()

	for i := range count(res) {
		d.types[res.String(path("data", i, "name"))] = res.String(path("data", i, "id"))
	}

	return d
}

/*
path joins a JSON path.
*/
func path(parts ...any) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "."
		}

		switch v := p.(type) {
		case string:
			out += v
		case int:
			out += itoa(v)
		}
	}

	return out
}

/*
count reads how many items a response's data holds.
*/
func count(res *testkit.Response) int {
	n, _ := strconv.Atoi(res.String("data.#"))

	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}

	return s
}

/*
state returns the id of the state named.
*/
func (d desk) state(t *testing.T, name string) string {
	t.Helper()

	res := d.Owner.GetJSON("/api/v1/ticket-states").AssertOK()
	for i := range count(res) {
		if res.String(path("data", i, "name")) == name {
			return res.String(path("data", i, "id"))
		}
	}

	t.Fatalf("no state %s", name)

	return ""
}

func TestTicketSetup(t *testing.T) {
	t.Parallel()

	t.Run("starts a workspace with a type of each kind and the five groups", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)

		for _, name := range []string{"Question", "Bug report", "Back-office task", "Tracker"} {
			assert.NotEmpty(t, d.types[name], name)
		}

		states := d.Owner.GetJSON("/api/v1/ticket-states").AssertOK()
		assert.GreaterOrEqual(t, count(states), 8)
	})

	t.Run("adds a field and refuses a second with the same key", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		fields := "/api/v1/ticket-types/" + d.types["Back-office task"] + "/fields"

		d.Owner.PostJSON(fields, testkit.JSON{
			"key": "amount", "label": "Amount", "data_type": "currency", "required_create": true,
		}).AssertCreated().AssertJSONPath("data.key", "amount")
		d.Owner.PostJSON(fields, testkit.JSON{"key": "amount", "label": "Again", "data_type": "number"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "key_taken")
	})

	t.Run("removes a state only with a target of the same group", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		info := d.state(t, "Needs info")

		d.Owner.Delete("/api/v1/ticket-states/" + info).AssertStatus(fhttp.StatusUnprocessableEntity)
		made := d.Owner.PostJSON("/api/v1/ticket-states", testkit.JSON{
			"group": "waiting_customer", "name": "Waiting on logs", "customer_label": "Waiting for your logs",
		}).AssertCreated().String("data.id")
		d.Owner.Delete("/api/v1/ticket-states/" + info + "?move_to=" + made).AssertStatus(fhttp.StatusNoContent)
	})
}

func TestTicketRules(t *testing.T) {
	t.Parallel()

	t.Run("refuses a create missing required fields and lists them, for the API too", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		backOffice := d.types["Back-office task"]
		d.Owner.PostJSON("/api/v1/ticket-types/"+backOffice+"/fields", testkit.JSON{
			"key": "order_number", "label": "Order", "data_type": "short_text", "required_create": true,
		}).AssertCreated()

		d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{"type_id": backOffice, "title": "Refund"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "required_fields_missing")

		d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": backOffice, "title": "Refund", "fields": testkit.JSON{"order_number": "8812"},
		}).AssertCreated().AssertJSONPath("data.fields.order_number", "8812")
	})

	t.Run("returns the first ticket when a create repeats its external id", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		body := testkit.JSON{"type_id": d.types["Tracker"], "title": "CSV export crashes", "external_id": "bug-1"}

		first := d.Owner.PostJSON("/api/v1/tickets", body).AssertCreated().String("data.id")
		again := d.Owner.PostJSON("/api/v1/tickets", body).AssertOK().String("data.id")

		assert.Equal(t, first, again)
		assert.Equal(t, "T-1", d.Owner.GetJSON("/api/v1/tickets/"+first).AssertOK().String("data.reference"))
		d.Owner.GetJSON("/api/v1/tickets/T-1").AssertOK().AssertJSONPath("data.id", first)
	})

	t.Run("refuses an edit made against an older version", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		id := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{"type_id": d.types["Tracker"], "title": "Slow"}).
			AssertCreated().String("data.id")

		d.Owner.PatchJSON("/api/v1/tickets/"+id, testkit.JSON{"title": "Slow pages", "version": 1}).AssertOK()
		d.Owner.PatchJSON("/api/v1/tickets/"+id, testkit.JSON{"title": "Stale", "version": 1}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "version_conflict")
	})

	t.Run("needs required-to-resolve fields before resolving, and closes the conversation", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		bug := d.types["Bug report"]
		d.Owner.PostJSON("/api/v1/ticket-types/"+bug+"/fields", testkit.JSON{
			"key": "root_cause", "label": "Root cause", "data_type": "long_text", "required_resolve": true,
		}).AssertCreated()
		conv := d.Customer(t, "ada", "Export broken").String("data.id")
		id := d.Owner.PostJSON("/api/v1/conversations/"+conv+"/ticket", testkit.JSON{"type_id": bug}).
			AssertCreated().AssertJSONPath("data.conversation_id", conv).String("data.id")
		resolved := d.state(t, "Resolved")

		d.Owner.PatchJSON("/api/v1/tickets/"+id, testkit.JSON{"state_id": resolved}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "required_fields_missing")
		d.Owner.PatchJSON("/api/v1/tickets/"+id, testkit.JSON{
			"state_id": resolved, "fields": testkit.JSON{"root_cause": "Encoding bug"},
		}).AssertOK().AssertJSONPath("data.group", "resolved")

		d.Owner.GetJSON("/api/v1/conversations/"+conv).AssertOK().AssertJSONPath("data.status", "closed")
	})

	t.Run("reopens a resolved ticket when the customer writes within the window", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		conv := d.Customer(t, "bo", "Invoice wrong").String("data.id")
		id := d.Owner.PostJSON("/api/v1/conversations/"+conv+"/ticket", testkit.JSON{"type_id": d.types["Question"]}).
			AssertCreated().String("data.id")
		d.Owner.PatchJSON("/api/v1/conversations/"+conv, testkit.JSON{"status": "closed"}).AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+id).AssertOK().AssertJSONPath("data.group", "resolved")

		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/messages", testkit.JSON{"body": "Still wrong"}).
			AssertCreated()

		d.Owner.GetJSON("/api/v1/tickets/"+id).AssertOK().AssertJSONPath("data.group", "in_progress")
	})
}

func TestTicketTeams(t *testing.T) {
	t.Parallel()

	t.Run("escalates to back-office work and closes the loop when it resolves", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		conv := d.Customer(t, "cy", "Charged twice").String("data.id")
		customer := d.Owner.PostJSON("/api/v1/conversations/"+conv+"/ticket", testkit.JSON{
			"type_id": d.types["Question"],
		}).AssertCreated().String("data.id")

		work := d.Owner.PostJSON("/api/v1/tickets/"+customer+"/escalations", testkit.JSON{
			"type_id": d.types["Back-office task"], "title": "Billing: refund 49",
		}).AssertCreated().String("data.id")

		d.Owner.GetJSON("/api/v1/tickets/"+customer).AssertOK().AssertJSONPath("data.group", "waiting_team").
			AssertJSONPath("data.links.0.kind", "back_office")

		d.Owner.PatchJSON("/api/v1/tickets/"+work, testkit.JSON{"state_id": d.state(t, "Resolved")}).AssertOK()

		d.Owner.GetJSON("/api/v1/tickets/"+customer).AssertOK().AssertJSONPath("data.group", "in_progress").
			AssertJSONPath("data.close_loop", true)
	})

	t.Run("links conversations to a tracker and messages them, holding who wrote since", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		tracker := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Tracker"], "title": "CSV export crashes",
		}).AssertCreated().String("data.id")

		var convs []string
		for _, name := range []string{"ann", "ben", "cat"} {
			c := d.Customer(t, name, "CSV export").String("data.id")
			convs = append(convs, c)
			link := testkit.JSON{"kind": "tracker", "conversation_id": c}
			d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/links", link).
				AssertOK()
		}

		d.TC.Clock.Travel(time.Minute)
		d.Owner.PostJSON("/api/v1/conversations/"+convs[2]+"/messages", testkit.JSON{"body": "any news?"}).
			AssertCreated()

		dry := d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/updates", testkit.JSON{
			"body": "The fix is live.", "dry_run": true,
		}).AssertOK()
		dry.AssertJSONPath("data.recipients", float64(3)).AssertJSONPath("data.held.#", float64(1))

		d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/updates", testkit.JSON{"body": "The fix is live."}).AssertOK()
		d.TC.Drain()

		updates := d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/updates").AssertOK()
		updates.AssertJSONPath("data.0.status", "sent").AssertJSONPath("data.0.delivered", float64(2)).
			AssertJSONPath("data.0.held", float64(1))

		reach := d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/conversations").AssertOK()
		reach.AssertJSONPath("meta.impact.customers", float64(3))
		require.Equal(t, 3, count(reach))
	})

	t.Run("shows a conversation its ticket and the trackers it joined", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		conv := d.Customer(t, "dee", "Login fails").String("data.id")
		tracker := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Tracker"], "title": "Sign-in outage",
			"link_to": []testkit.JSON{{"kind": "tracker", "conversation_id": conv}},
		}).AssertCreated().String("data.id")

		d.Owner.GetJSON("/api/v1/conversations/"+conv+"/tickets").AssertOK().
			AssertJSONPath("data.trackers.0.id", tracker)
		d.Owner.GetJSON("/api/v1/tickets?kind=tracker&active=true").AssertOK().AssertJSONPath("data.0.id", tracker)
	})
}
