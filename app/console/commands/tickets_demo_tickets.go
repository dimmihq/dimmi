//go:build dev

/*
|--------------------------------------------------------------------------
| Tickets Demo Customer Tickets
|--------------------------------------------------------------------------
|
| Recent conversations become customer tickets a while after they
| started, each of a type with its fields filled, and then live a few
| days: picked up, waiting on the customer, handed to billing or
| engineering, resolved or closed as won't fix, by different teammates
| hours apart. A few carry field values the AI proposed for a teammate
| to accept. A ticket the conversation already has is left alone.
|
*/

package commands

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
)

/*
demoConv is a conversation the demo turns into a ticket.
*/
type demoConv struct {
	id      ids.UUID
	at      time.Time
	subject string
}

/*
recentConversations returns conversations of the last weeks that are
not tickets yet, in a stable shuffled order.
*/
func (d *ticketDemo) recentConversations(ctx context.Context, n int, since time.Time) ([]demoConv, error) {
	var out []demoConv

	err := d.app.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := tx.Query(ctx, `SELECT c.id, c.created_at, coalesce(c.subject, '') FROM conversations c
			WHERE c.workspace_id = $1 AND c.deleted_at IS NULL AND c.created_at >= $2
			  AND NOT EXISTS (SELECT 1 FROM tickets t WHERE t.conversation_id = c.id)
			  AND NOT EXISTS (SELECT 1 FROM ticket_links l WHERE l.conversation_id = c.id)
			ORDER BY md5(c.id::text) LIMIT $3`, d.owner.Workspace.ID, since, n)
		if err != nil {
			return fmt.Errorf("list recent conversations: %w", err)
		}

		defer rows.Close()

		for rows.Next() {
			var c demoConv
			if err := rows.Scan(&c.id, &c.at, &c.subject); err != nil {
				return fmt.Errorf("read conversation: %w", err)
			}

			out = append(out, c)
		}

		return rows.Err()
	})

	return out, err
}

/*
customerTickets turns recent conversations into tickets with history.
*/
func (d *ticketDemo) customerTickets(ctx context.Context, n int) error {
	now := d.app.Clock.Now()

	seeded, err := d.seededTickets(ctx)
	if err != nil || seeded >= n {
		return err
	}

	convs, err := d.recentConversations(ctx, n-seeded, now.AddDate(0, 0, -45))
	if err != nil {
		return err
	}

	for i, c := range convs {
		if err := d.customerTicket(ctx, i, c, now); err != nil {
			return fmt.Errorf("ticket %d: %w", i, err)
		}

		if (i+1)%50 == 0 {
			d.out.Line("seeded %d customer tickets", i+1)
		}
	}

	return nil
}

/*
seededTickets counts the customer tickets an earlier run made, so a
second run only tops them up.
*/
func (d *ticketDemo) seededTickets(ctx context.Context) (int, error) {
	var n int

	err := d.app.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM tickets WHERE workspace_id = $1
			AND external_id LIKE 'demo:ticket:%'`, d.owner.Workspace.ID).Scan(&n)
	})
	if err != nil {
		return 0, fmt.Errorf("count seeded tickets: %w", err)
	}

	return n, nil
}

/*
customerTicket makes one ticket and plays its history.
*/
func (d *ticketDemo) customerTicket(ctx context.Context, i int, c demoConv, now time.Time) error {
	kind := demoKind(i)
	at := c.at.Add(time.Duration(5+roll(85, "open", i)) * time.Minute)

	if latest := now.Add(-time.Hour); at.After(latest) {
		at = latest
	}

	in := d.ticketInput(i, kind, c)

	var made tickets.Created

	err := d.write(ctx, at, d.mate("opened", i), func(ctx context.Context, w *tickets.Writer) error {
		var err error
		made, err = w.Create(ctx, d.owner.Workspace.ID, in)

		return err
	})
	if err != nil || !made.New {
		return err
	}

	run := demoRun{i: i, kind: kind, at: at, now: now}

	t, err := d.playHistory(ctx, run, made.Ticket)
	if err != nil {
		return err
	}

	d.made = append(d.made, t)

	return d.clocks(ctx, run, t)
}

/*
demoKind picks a ticket's type for its place in the run.
*/
func demoKind(i int) string {
	switch r := roll(100, "kind", i); {
	case r < 32:
		return "Refund request"
	case r < 62:
		return "Bug report"
	case r < 80:
		return "Account access"
	default:
		return "Question"
	}
}

/*
ticketInput fills a ticket of the type for the conversation.
*/
func (d *ticketDemo) ticketInput(i int, kind string, c demoConv) tickets.CreateInput {
	external := "demo:ticket:" + c.id.String()
	in := tickets.CreateInput{
		TypeID: d.types[kind], ConversationID: &c.id, Title: c.subject, ExternalID: &external,
		Fields: d.demoFields(i, kind), Priority: tickets.Priorities()[roll(5, "prio", i)],
	}

	if in.Title == "" {
		in.Title = kind + " from a customer"
	}

	if len(d.policies) > 0 {
		in.SLAPolicyID = &d.policies[roll(len(d.policies), "policy", i)]
	}

	if i%37 == 5 {
		in.Suggestions = demoSuggestions(kind)
		for key := range in.Suggestions {
			delete(in.Fields, key)
		}
	}

	return in
}

/*
demoFields returns field values a ticket of the type is opened with,
choices picked from the options the workspace's fields have.
*/
func (d *ticketDemo) demoFields(i int, kind string) map[string]any {
	f := map[string]any{}

	switch kind {
	case "Refund request":
		f["order_number"] = "ORD-" + strconv.Itoa(100000+roll(899999, "order", i))
		f["amount"] = float64(19+roll(480, "amount", i)) + 0.99
		f["reason"] = d.choice(kind, "reason", i)

		if f["reason"] == "Other" {
			f["reason_detail"] = "The size chart was wrong, so nothing fits."
		}
	case "Bug report":
		f["severity"] = d.choice(kind, "severity", i)
		f["platform"] = []any{d.choice(kind, "platform", i)}
		f["steps"] = "1. Open the export page\n2. Pick last quarter\n3. Press Export and wait"
	case "Account access":
		f["issue"] = d.choice(kind, "issue", i)

		if f["issue"] == "SSO error" {
			f["identity_provider"] = []string{"Okta", "Azure AD", "Google Workspace"}[roll(3, "idp", i)]
		}
	}

	for key, v := range f {
		if v == "" || fmt.Sprint(v) == "[]" {
			delete(f, key)
		}
	}

	return f
}

/*
demoSuggestions are values the AI proposed for a ticket of the type.
*/
func demoSuggestions(kind string) map[string]any {
	switch kind {
	case "Refund request":
		return map[string]any{"reason": "Other"}
	case "Bug report":
		return map[string]any{"severity": "high"}
	}

	return nil
}
