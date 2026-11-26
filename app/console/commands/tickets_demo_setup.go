//go:build dev

/*
|--------------------------------------------------------------------------
| Tickets Demo Setup
|--------------------------------------------------------------------------
|
| What the tickets demo needs before any ticket: the team it spreads
| work across, paying companies for the customers so trackers show the
| revenue at stake, and the ticket types a support team runs, each with
| its fields: refunds with an order number, amount and reason whose
| detail shows only for "other", bug reports with platforms, account
| access with the identity provider asked only for sign-in errors, and
| back-office types for billing and engineering. Each is found by name
| first, so a second run adds nothing.
|
*/

package commands

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
demoMate is a teammate the demo acts as.
*/
type demoMate struct {
	id   ids.UUID
	name string
}

/*
ticketDemo is one run of the tickets demo and what it found and made.
*/
type ticketDemo struct {
	app      *app.App
	owner    authz.Membership
	out      *console.Output
	team     []demoMate
	types    map[string]ids.UUID
	states   map[string]ids.UUID
	policies []ids.UUID
	made     []models.Ticket
	options  map[string][]string
}

/*
roll returns a number below n that is the same for the same seeds, so
every run spreads the work the same way.
*/
func roll(n int, seeds ...any) int {
	h := fnv.New32a()
	_, _ = fmt.Fprint(h, seeds...)

	return int(h.Sum32() % uint32(max(n, 1))) //nolint:gosec // n is small and positive
}

/*
mate returns a teammate for the seeds.
*/
func (d *ticketDemo) mate(seeds ...any) demoMate {
	return d.team[roll(len(d.team), seeds...)]
}

/*
write runs the function with a ticket writer acting as the teammate at
the time, replaying history: nothing that reacts to new work hears it.
*/
func (d *ticketDemo) write(ctx context.Context, at time.Time, by demoMate,
	fn func(ctx context.Context, w *tickets.Writer) error,
) error {
	return d.app.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		cw := &conversations.Writer{
			Q: models.New(tx), Tx: tx, Events: d.app.Events, Now: at, Imported: true,
			Actor: conversations.Actor{Type: string(enums.AuthorTypeMember), ID: by.id, Name: by.name},
		}

		w, err := d.app.Actions.Tickets.Wrap(ctx, cw, d.owner.Workspace.ID)
		if err != nil {
			return err
		}

		return fn(ctx, w)
	})
}

/*
loadTeam reads the workspace's active teammates.
*/
func (d *ticketDemo) loadTeam(ctx context.Context) error {
	return d.app.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := tx.Query(ctx, `SELECT m.id, u.name FROM workspace_members m JOIN users u ON u.id = m.user_id
			WHERE m.workspace_id = $1 AND m.status = 'active' ORDER BY m.id`, d.owner.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list teammates: %w", err)
		}

		defer rows.Close()

		for rows.Next() {
			var m demoMate
			if err := rows.Scan(&m.id, &m.name); err != nil {
				return fmt.Errorf("read teammate: %w", err)
			}

			d.team = append(d.team, m)
		}

		return rows.Err()
	})
}

/*
setUpTypes adds the demo's types and fields and reads every type and
state by name.
*/
func (d *ticketDemo) setUpTypes(ctx context.Context) error {
	setup, _, err := d.app.Actions.Tickets.Settings(ctx, d.owner)
	if err != nil {
		return err
	}

	d.types, d.states = map[string]ids.UUID{}, map[string]ids.UUID{}
	for _, s := range setup.States {
		d.states[s.Name] = s.ID
	}

	for _, t := range setup.Types {
		d.types[t.Name] = t.ID
	}

	for _, spec := range demoTypes() {
		if err := d.ensureType(ctx, setup, spec); err != nil {
			return fmt.Errorf("type %s: %w", spec.name, err)
		}
	}

	return d.loadOptions(ctx)
}

/*
loadOptions reads the options of every choice field by type and key,
as the workspace has them, which may differ from the demo's own for a
type that was there before.
*/
func (d *ticketDemo) loadOptions(ctx context.Context) error {
	setup, _, err := d.app.Actions.Tickets.Settings(ctx, d.owner)
	if err != nil {
		return err
	}

	d.options = map[string][]string{}

	for name, id := range d.types {
		for _, f := range setup.FieldsOf(id) {
			d.options[name+"."+f.Key] = f.Options
		}
	}

	return nil
}

/*
choice picks one of the field's options for the seeds, empty when the
type has no such field.
*/
func (d *ticketDemo) choice(kind, key string, seeds ...any) string {
	options := d.options[kind+"."+key]
	if len(options) == 0 {
		return ""
	}

	return options[roll(len(options), append([]any{key}, seeds...)...)]
}
