package widget_test

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
ticketTypes reads the workspace's ticket types by name, as the team
sees them.
*/
func ticketTypes(t *testing.T, tc *tests.TestCase) map[string]string {
	t.Helper()

	res := owner(tc).GetJSON("/api/v1/ticket-types").AssertOK()
	out := map[string]string{}

	var body struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	res.Decode(&body)

	for _, ty := range body.Data {
		out[ty.Name] = ty.ID
	}

	return out
}

/*
submit sends the visitor's form for a request.
*/
func (v *visitor) submit(typeID, client string, extra testkit.JSON) *testkit.Response {
	body := testkit.JSON{"client_id": client, "type_id": typeID, "title": "Export is broken"}
	maps.Copy(body, extra)

	return v.client().PostJSON("/widget/v1/tickets", body)
}

/*
sql runs a statement inside the workspace, for what the API cannot set.
*/
func sql(t *testing.T, tc *tests.TestCase, statement string, args ...any) {
	t.Helper()

	ws := tc.Workspace.Workspace.ID
	ctx := tenancy.With(t.Context(), ws.String())
	err := tc.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := tx.Exec(ctx, statement, args...)

		return err
	})
	require.NoError(t, err)
}

func TestWidgetTicketTypes(t *testing.T) {
	t.Parallel()

	t.Run("lists the customer types with only their visible fields and customer labels", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		res := v.client().GetJSON("/widget/v1/ticket-types").AssertOK()

		res.AssertJSONPath("data.#", 2)
		res.AssertJSONPath("data.0.name", "Question")
		res.AssertJSONPath("data.1.name", "Bug report")
		res.AssertJSONPath("data.1.fields.#", 1)
		res.AssertJSONPath("data.1.fields.0.key", "steps")
		res.AssertJSONMissing("data.1.states.0.name")
		assert.NotEmpty(t, res.String("data.1.states.0.label"))
	})

	t.Run("reads the types without a session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().GetJSON("/widget/v1/apps/"+appID(t, tc)+"/ticket-types").AssertOK().AssertJSONPath("data.#", 2)
	})
}

func TestWidgetTicketForm(t *testing.T) {
	t.Parallel()

	t.Run("opens a request on a new conversation and returns it once when sent twice", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		bug := ticketTypes(t, tc)["Bug report"]
		client := ids.New().String()

		first := v.submit(bug, client, testkit.JSON{
			"description": "The CSV is empty", "fields": testkit.JSON{"steps": "Open reports, export"},
		}).AssertCreated()
		again := v.submit(bug, client, testkit.JSON{
			"description": "The CSV is empty", "fields": testkit.JSON{"steps": "Open reports, export"},
		}).AssertOK()

		assert.Equal(t, first.String("data.id"), again.String("data.id"))
		first.AssertJSONPath("data.reference", "T-1")
		first.AssertJSONPath("data.mine", true)
		first.AssertJSONPath("data.state.group", "submitted")
		first.AssertJSONPath("data.fields.0.value", "Open reports, export")
		tc.Table("tickets").AssertCount(1, testkit.Row{})
		tc.Table("conversations").AssertCount(1, testkit.Row{})

		conv := first.String("data.conversation_id")
		v.client().GetJSON("/widget/v1/conversations/"+conv).AssertOK().
			AssertJSONPath("data.ticket.reference", "T-1").
			AssertJSONPath("data.ticket.title", "Export is broken")
		v.client().GetJSON("/widget/v1/conversations").AssertOK().AssertJSONPath("data.0.ticket.reference", "T-1")
		v.client().GetJSON("/widget/v1/conversations/"+conv+"/messages").AssertOK().
			AssertJSONPath("data.0.body", "Export is broken\n\nThe CSV is empty")
	})

	t.Run("refuses a form missing a required field and starts no conversation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		types := ticketTypes(t, tc)
		owner(tc).PostJSON("/api/v1/ticket-types/"+types["Question"]+"/fields", testkit.JSON{
			"key": "order", "label": "Order number", "data_type": "short_text", "visible_customer": true,
			"editable_customer": true, "required_create_customer": true,
		}).AssertCreated()

		v.submit(types["Question"], ids.New().String(), nil).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "required_fields_missing")
		v.submit(types["Question"], ids.New().String(), testkit.JSON{"fields": testkit.JSON{"internal": "x"}}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
		v.submit(types["Tracker"], ids.New().String(), nil).AssertNotFound()

		tc.Table("conversations").AssertCount(0, testkit.Row{})
		v.submit(types["Question"], ids.New().String(), testkit.JSON{"fields": testkit.JSON{"order": "8812"}}).
			AssertCreated()
	})

	t.Run("shows a request with its states by customer label, and hides anyone else's", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		made := v.submit(ticketTypes(t, tc)["Question"], ids.New().String(), nil).AssertCreated()
		id := made.String("data.id")

		owner(tc).PatchJSON("/api/v1/tickets/"+id, testkit.JSON{"state_id": stateID(t, tc, "Investigating")}).
			AssertOK()

		res := v.client().GetJSON("/widget/v1/tickets/T-1").AssertOK()
		res.AssertJSONPath("data.id", id)
		res.AssertJSONPath("data.state.group", "in_progress")
		res.AssertJSONPath("data.history.#", 2)
		res.AssertJSONMissing("data.state.name")

		other := startVisitor(t, tc)
		other.client().GetJSON("/widget/v1/tickets/" + id).AssertNotFound()
		other.client().GetJSON("/widget/v1/tickets").AssertOK().AssertJSONPath("data.#", 0)
		v.client().GetJSON("/widget/v1/tickets?open=true").AssertOK().AssertJSONPath("data.#", 1)
	})
}

/*
stateID returns the id of the state named.
*/
func stateID(t *testing.T, tc *tests.TestCase, name string) string {
	t.Helper()

	var body struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	owner(tc).GetJSON("/api/v1/ticket-states").AssertOK().Decode(&body)

	for _, s := range body.Data {
		if s.Name == name {
			return s.ID
		}
	}

	t.Fatalf("no state %s", name)

	return ""
}

func TestWidgetCompanyTickets(t *testing.T) {
	t.Parallel()

	t.Run("shows the company's shared requests only to a verified visitor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		types := ticketTypes(t, tc)
		owner(tc).PatchJSON("/api/v1/ticket-types/"+types["Question"], testkit.JSON{"company_sharing": true}).
			AssertOK()

		ada, bo := startVisitor(t, tc), startVisitor(t, tc)
		company := ids.New()
		sql(t, tc, `INSERT INTO companies (id, workspace_id, name, custom_attributes, created_at, updated_at)
			VALUES ($1, $2, 'Acme', '{}', now(), now())`, company, tc.Workspace.Workspace.ID)
		sql(t, tc, `UPDATE contacts SET company_id = $1`, company)

		ada.submit(types["Question"], ids.New().String(), nil).AssertCreated()

		bo.client().GetJSON("/widget/v1/tickets?scope=company").AssertOK().AssertJSONPath("data.#", 0)

		sql(t, tc, `UPDATE widget_sessions SET kind = 'identified', verified_at = now(), expires_at = $1`,
			tc.Clock.Now().Add(time.Hour))

		res := bo.client().GetJSON("/widget/v1/tickets?scope=company").AssertOK()
		res.AssertJSONPath("data.#", 1)
		res.AssertJSONPath("data.0.mine", false)
		bo.client().GetJSON("/widget/v1/tickets").AssertOK().AssertJSONPath("data.#", 0)
	})
}
