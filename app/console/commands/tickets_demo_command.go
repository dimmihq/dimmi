//go:build dev

/*
|--------------------------------------------------------------------------
| Tickets Demo
|--------------------------------------------------------------------------
|
| Fills a development workspace that already has conversations, such as
| one inbox:demo seeded, with tickets the way a busy team builds them up:
| customer types with their fields, back-office types, hundreds of
| tickets across states and teammates with weeks of history, trackers
| gathering dozens of reports from paying companies, tracker messages
| sent to everyone linked, service level policies with clocks met,
| breached and running, and AI suggestions waiting on a few tickets. Every ticket
| goes through the tickets writer, replayed at its own time like an
| import, under an external id, so running it again adds nothing twice.
|
*/

package commands

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
)

/*
TicketsDemoCommand seeds a workspace with tickets.
*/
type TicketsDemoCommand struct {
	App *app.App
}

/*
Name returns tickets:demo.
*/
func (c TicketsDemoCommand) Name() string {
	return "tickets:demo"
}

/*
Description says what the command does.
*/
func (c TicketsDemoCommand) Description() string {
	return "Seed a development workspace with ticket types, hundreds of tickets, trackers and SLAs"
}

/*
Flags names the workspace, the owner the demo acts as and how many
tickets to seed.
*/
func (c TicketsDemoCommand) Flags(flags *console.Flags) {
	flags.String("workspace", "", "the workspace's public id, ws_...")
	flags.String("member", "", "the owner's member public id, mem_...")
	flags.Int("tickets", 320, "how many customer tickets to seed")
}

/*
Handle seeds the workspace.
*/
func (c TicketsDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	owner, err := SeriesDemoCommand(c).membership(ctx, in.String("workspace"), in.String("member"))
	if err != nil {
		return err
	}

	ctx = tenancy.With(ctx, owner.Workspace.ID.String())
	d := &ticketDemo{app: c.App, owner: owner, out: out}

	if err := d.prepare(ctx); err != nil {
		return err
	}

	out.Line("ready: %d teammates, %d companies, ticket types set up", len(d.team), d.companies)

	steps := []struct {
		name string
		run  func(context.Context) error
	}{
		{"service level policies", d.seedPolicies},
		{"customer tickets", func(ctx context.Context) error { return d.customerTickets(ctx, in.Int("tickets")) }},
		{"trackers", d.trackers},
		{"tracker messages", d.trackerMessages},
	}

	for _, s := range steps {
		if err := s.run(ctx); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}

		out.Line("seeded %s", s.name)
	}

	return nil
}
