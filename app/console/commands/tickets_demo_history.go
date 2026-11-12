//go:build dev

/*
|--------------------------------------------------------------------------
| Tickets Demo History
|--------------------------------------------------------------------------
|
| Where a demo ticket ends up and how it got there. Most are resolved
| after someone investigated, some asked the customer for more first;
| others are still being investigated, wait on the customer, sit with
| billing or engineering as back-office work, are still new, or were
| closed as won't fix. Each move happens hours after the one before,
| often by another teammate, and stops short of now.
|
*/

package commands

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
demoPath returns the states a ticket moves through after it opens, and
whether it is handed to another team after the last.
*/
func demoPath(i int) ([]string, bool) {
	switch r := roll(100, "outcome", i); {
	case r < 26:
		return []string{"Investigating", "Resolved"}, false
	case r < 38:
		return []string{"Investigating", "Needs info", "Investigating", "Resolved"}, false
	case r < 52:
		return []string{"Investigating"}, false
	case r < 64:
		return []string{"Investigating", "Needs info"}, false
	case r < 78:
		return []string{"Investigating"}, true
	case r < 88:
		return nil, false
	case r < 94:
		return []string{"Investigating", "Won't fix"}, false
	default:
		return []string{"Being worked on"}, false
	}
}

/*
demoRun is one demo ticket being played: its place in the run, its
type's name, when it opened and the moment the run started.
*/
type demoRun struct {
	i       int
	kind    string
	at, now time.Time
}

/*
playHistory moves the ticket along its path, hours apart, and hands it
on when its path says so.
*/
func (d *ticketDemo) playHistory(ctx context.Context, run demoRun, t models.Ticket) (models.Ticket, error) {
	i, at, now := run.i, run.at, run.now
	path, handOff := demoPath(i)

	for step, name := range path {
		at = at.Add(time.Duration(1+roll(30, "gap", i, step)) * time.Hour)
		if at.After(now.Add(-5 * time.Minute)) {
			return t, nil
		}

		by := d.mate("step", i, step)
		state := d.states[name]

		err := d.write(ctx, at, by, func(ctx context.Context, w *tickets.Writer) error {
			locked, err := w.Q.LockTicket(ctx, models.LockTicketParams{WorkspaceID: t.WorkspaceID, ID: t.ID})
			if err != nil {
				return err
			}

			t, err = w.Apply(ctx, locked, tickets.Change{StateID: &state, Assignee: &by.id})

			return err
		})
		if err != nil {
			return t, err
		}
	}

	if !handOff || at.Add(time.Hour).After(now) {
		return t, nil
	}

	run.at = at.Add(time.Hour)

	return d.handOff(ctx, run, t)
}

/*
handOff makes back-office work for a refund or a bug, which leaves the
customer's ticket waiting on that team.
*/
func (d *ticketDemo) handOff(ctx context.Context, run demoRun, t models.Ticket) (models.Ticket, error) {
	i := run.i
	in := tickets.EscalateInput{
		TypeID: d.types["Engineering"], Title: "Fix for " + t.Title,
		Fields: map[string]any{"component": []string{"Exports", "Mobile", "Auth"}[roll(3, "component", i)]},
	}

	switch run.kind {
	case "Refund request":
		in = tickets.EscalateInput{
			TypeID: d.types["Billing"], Title: "Refund " + t.Title,
			Description: "Refund the order in full and send the credit note.",
		}
	case "Bug report":
	default:
		return t, nil
	}

	assignee := d.mate("backoffice", i).id
	in.Assignee = &assignee

	err := d.write(ctx, run.at, d.mate("escalate", i), func(ctx context.Context, w *tickets.Writer) error {
		locked, err := w.Q.LockTicket(ctx, models.LockTicketParams{WorkspaceID: t.WorkspaceID, ID: t.ID})
		if err != nil {
			return err
		}

		if _, err := w.Escalate(ctx, locked, in); err != nil {
			return err
		}

		t, err = w.Q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: t.WorkspaceID, ID: t.ID})

		return err
	})

	return t, err
}
