package ticketimports_test

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/actions/ticketimports"
	"github.com/dimmihq/dimmi/app/models"
)

/*
data reads an event's data.
*/
func data(t *testing.T, e models.TicketEvent) map[string]any {
	t.Helper()

	var out map[string]any
	require.NoError(t, json.Unmarshal(e.Data, &out))

	return out
}

/*
fields reads a ticket's values.
*/
func fields(t *testing.T, tk models.Ticket) map[string]any {
	t.Helper()

	var out map[string]any
	require.NoError(t, json.Unmarshal(tk.Fields, &out))

	return out
}

func TestIntercomImport(t *testing.T) {
	t.Parallel()

	t.Run("maps types, attributes, states and tickets with their history", func(t *testing.T) {
		t.Parallel()
		c := newImportCase(t)

		row := c.run(t, c.intercom())
		assert.EqualValues(t, 3, row.TicketsCreated)
		assert.EqualValues(t, 2, row.TypesCreated)
		assert.EqualValues(t, 3, row.FieldsCreated)
		assert.EqualValues(t, 1, row.LinksCreated)
		assert.Empty(t, c.created.All(), "an import sets nothing off")

		billing := c.state(t, "Billing review")
		assert.Equal(t, "in_progress", billing.StateGroup)
		assert.Equal(t, "With our billing team", billing.CustomerLabel)
		assert.Equal(t, "waiting_customer", c.state(t, "Waiting on customer").StateGroup)

		refund := c.ticket(t, "intercom:101")
		assert.Equal(t, "Refund request", refund.Type.Name)
		assert.Equal(t, "customer", refund.Type.Kind)
		assert.Equal(t, "Billing review", refund.State.Name)
		assert.Equal(t, "Refund for order 4417", refund.Ticket.Title)
		assert.Equal(t, map[string]any{"order_number": "4417", "refund_amount": 49.5, "reason": "Damaged"},
			fields(t, refund.Ticket))
		assert.True(t, refund.Ticket.CreatedAt.Equal(time.Unix(1793000000, 0)))
		require.NotNil(t, refund.Ticket.AssigneeMemberID)
		assert.Equal(t, c.TC.Workspace.Owner.ID, *refund.Ticket.AssigneeMemberID)
		require.NotNil(t, refund.Ticket.ContactID)

		require.Len(t, refund.Events, 2)
		assert.Equal(t, "created", refund.Events[0].Kind)
		assert.Equal(t, "Submitted", data(t, refund.Events[0])["to_name"])
		assert.True(t, refund.Events[0].CreatedAt.Equal(time.Unix(1793000000, 0)))
		assert.Equal(t, "state", refund.Events[1].Kind)
		assert.Equal(t, "Billing review", data(t, refund.Events[1])["to_name"])
		assert.True(t, refund.Events[1].CreatedAt.Equal(time.Unix(1793003600, 0)))

		late := c.ticket(t, "intercom:102")
		assert.Equal(t, "resolved", late.Ticket.StateGroup)
		require.NotNil(t, late.Ticket.ResolvedAt)
		assert.True(t, late.Ticket.ResolvedAt.Equal(time.Unix(1793190000, 0)))
		require.Len(t, late.Events, 3)
		assert.Equal(t, "Waiting on customer", data(t, late.Events[1])["to_name"])
		assert.Nil(t, late.Ticket.AssigneeMemberID, "a teammate who is not a member stays unassigned")

		tracker := c.ticket(t, "intercom:103")
		assert.Equal(t, "tracker", tracker.Type.Kind)
		assert.Equal(t, 1, c.count(t,
			"SELECT count(*) FROM ticket_links WHERE workspace_id = $1 AND from_ticket_id = $2 AND to_ticket_id = $3",
			tracker.Ticket.ID, refund.Ticket.ID))

		assert.Equal(t, "Damaged", fields(t, refund.Ticket)["reason"])
		reason := c.count(t, "SELECT count(*) FROM ticket_fields WHERE workspace_id = $1 AND key = 'reason' "+
			"AND data_type = 'single_choice' AND options = ARRAY['Damaged', 'Arrived late']")
		assert.Equal(t, 1, reason, "archived options stay out")
	})

	t.Run("a second run finds everything the first brought in", func(t *testing.T) {
		t.Parallel()
		c := newImportCase(t)

		c.run(t, c.intercom())
		again := c.run(t, c.intercom())

		assert.EqualValues(t, 0, again.TicketsCreated)
		assert.EqualValues(t, 3, again.TicketsFound)
		assert.EqualValues(t, 0, again.TypesCreated)
		assert.EqualValues(t, 0, again.StatesCreated)
		assert.EqualValues(t, 0, again.FieldsCreated)
		assert.EqualValues(t, 0, again.LinksCreated)
		assert.Equal(t, 3, c.count(t, "SELECT count(*) FROM tickets WHERE workspace_id = $1"))
		assert.Equal(t, 1, c.count(t, "SELECT count(*) FROM ticket_types WHERE workspace_id = $1 "+
			"AND name = 'Refund request'"))
		assert.Len(t, c.ticket(t, "intercom:101").Events, 2)
	})

	t.Run("a refused token fails the import with why", func(t *testing.T) {
		t.Parallel()
		c := newImportCase(t)

		src := c.intercom()
		src.BaseURL = "https://api.intercom.io/missing"

		row, err := c.TC.App.Actions.TicketImports.Import(t.Context(), c.owner(), src)
		require.ErrorIs(t, err, ticketimports.ErrSourceRefused)
		assert.Equal(t, "failed", row.Status)
		require.NotNil(t, row.Error)
		assert.Contains(t, *row.Error, "refused")
	})
}

func TestZendeskImport(t *testing.T) {
	t.Parallel()

	t.Run("maps forms, fields, custom statuses, problems and history", func(t *testing.T) {
		t.Parallel()
		c := newImportCase(t)

		row := c.run(t, c.zendesk())
		assert.EqualValues(t, 3, row.TicketsCreated)
		assert.EqualValues(t, 1, row.LinksCreated)
		assert.Empty(t, c.created.All())

		escalated := c.state(t, "Escalated to engineering")
		assert.Equal(t, "in_progress", escalated.StateGroup)
		assert.Equal(t, "With our engineers", escalated.CustomerLabel)
		assert.Equal(t, "waiting_team", c.state(t, "On-hold").StateGroup)
		assert.Equal(t, "waiting_customer", c.state(t, "Pending").StateGroup)

		problem := c.ticket(t, "zendesk:1001")
		assert.Equal(t, "tracker", problem.Type.Kind)
		assert.Equal(t, "Problem", problem.Type.Name)
		assert.Equal(t, "Escalated to engineering", problem.State.Name)
		assert.Equal(t, "high", problem.Ticket.Priority)

		incident := c.ticket(t, "zendesk:1002")
		assert.Equal(t, "Bug report", incident.Type.Name, "a form named like a type reuses it")
		assert.Equal(t, "Pending", incident.State.Name)
		assert.Equal(t, "medium", incident.Ticket.Priority)
		assert.Equal(t, map[string]any{
			"product_area": "Billing", "affected_plans": []any{"Pro"}, "blocking": true, "needed_by": "2026-11-02",
		}, fields(t, incident.Ticket))
		assert.Equal(t, 1, c.count(t,
			"SELECT count(*) FROM ticket_links WHERE workspace_id = $1 AND from_ticket_id = $2 AND to_ticket_id = $3",
			problem.Ticket.ID, incident.Ticket.ID))

		solved := c.ticket(t, "zendesk:1003")
		assert.Equal(t, "Order problem", solved.Type.Name)
		assert.Equal(t, map[string]any{"order_number": "A-77", "product_area": "Shipping", "seats": float64(3)},
			fields(t, solved.Ticket))
		assert.True(t, solved.Ticket.CreatedAt.Equal(at("2026-10-03T12:00:00Z")))
		require.Len(t, solved.Events, 3)
		assert.Equal(t, "New", data(t, solved.Events[0])["to_name"])
		assert.Equal(t, "Open", data(t, solved.Events[1])["to_name"])
		assert.True(t, solved.Events[1].CreatedAt.Equal(at("2026-10-03T13:00:00Z")))
		assert.Equal(t, "Solved", data(t, solved.Events[2])["to_name"])
		require.NotNil(t, solved.Ticket.ResolvedAt)
		assert.True(t, solved.Ticket.ResolvedAt.Equal(at("2026-10-05T12:00:00Z")))

		assert.Equal(t, 1, c.count(t, "SELECT count(*) FROM contacts WHERE workspace_id = $1 "+
			"AND email = 'ben@initech.test'"))
	})

	t.Run("undoing trashes what the import made and keeps what was there", func(t *testing.T) {
		t.Parallel()
		c := newImportCase(t)

		before := c.Owner.PostJSON("/api/v1/tickets", map[string]any{
			"type_id": c.typeID(t, "Back-office task"), "title": "Already here",
		}).AssertCreated()

		row := c.run(t, c.zendesk())

		_, err := c.TC.App.Actions.TicketImports.Rollback(t.Context(), c.owner(), row.ID)
		require.NoError(t, err)
		c.TC.Drain()

		_, err = c.TC.App.Actions.TicketImports.Rollback(t.Context(), c.owner(), row.ID)
		require.ErrorIs(t, err, ticketimports.ErrRolledBack)

		assert.Equal(t, 3, c.count(t, "SELECT count(*) FROM tickets WHERE workspace_id = $1 "+
			"AND external_id LIKE 'zendesk:%' AND deleted_at IS NOT NULL"))
		assert.Equal(t, 1, c.count(t, "SELECT count(*) FROM tickets WHERE workspace_id = $1 AND deleted_at IS NULL"))
		assert.Equal(t, "Already here", before.String("data.title"))
		assert.Equal(t, 0, c.count(t, "SELECT count(*) FROM ticket_links WHERE workspace_id = $1"))
		assert.Equal(t, 1, c.count(t, "SELECT count(*) FROM ticket_types WHERE workspace_id = $1 "+
			"AND name = 'Bug report' AND archived_at IS NULL"), "a type that was there stays")
		assert.Equal(t, 1, c.count(t, "SELECT count(*) FROM ticket_types WHERE workspace_id = $1 "+
			"AND name = 'Order problem' AND archived_at IS NOT NULL"))
		assert.Equal(t, 0, c.count(t, "SELECT count(*) FROM ticket_fields WHERE workspace_id = $1 "+
			"AND key = 'affected_plans' AND archived_at IS NULL"))
		assert.Equal(t, 1, c.count(t, "SELECT count(*) FROM ticket_states WHERE workspace_id = $1 "+
			"AND name = 'New' AND archived_at IS NULL"), "a state that was there stays")

		var status string
		require.NoError(t, c.TC.Operator().QueryRow(t.Context(),
			"SELECT rollback_status FROM ticket_imports WHERE id = $1", row.ID).Scan(&status))
		assert.Equal(t, "completed", status)
	})
}

/*
typeID returns the public id of the workspace's type with the name.
*/
func (c *importCase) typeID(t *testing.T, name string) string {
	t.Helper()

	res := c.Owner.GetJSON("/api/v1/ticket-types").AssertOK()
	for i := range 20 {
		if res.String("data."+itoa(i)+".name") == name {
			return res.String("data." + itoa(i) + ".id")
		}
	}

	t.Fatalf("no type %s", name)

	return ""
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
