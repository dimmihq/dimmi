//go:build dev

/*
|--------------------------------------------------------------------------
| Demo Extras
|--------------------------------------------------------------------------
|
| What the other development demos know how to build, offered to the
| demo workspace seed in one call: broadcasts with the results each
| would have had, the demo series sent from the workspace's address,
| tickets with service levels and trackers, and the backlog of every
| past day for the reports. The series and broadcasts carry the brand
| they are given, so a seeded company mails as itself.
|
*/

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/auth/authz"
)

/*
DemoBrand is the company the extras speak for.
*/
type DemoBrand struct {
	Name, Slug, Domain string
}

/*
DemoExtrasInput is the workspace's owner, its brand, how many tickets to
seed and where to report.
*/
type DemoExtrasInput struct {
	Owner   authz.Membership
	Brand   DemoBrand
	Tickets int
	Out     *console.Output
}

/*
DemoExtras seeds broadcasts, series, tickets and backlog history in the
owner's workspace. Without a sending address, which needs inbound mail
set up, email broadcasts go in without one and the series are skipped.
*/
func DemoExtras(ctx context.Context, a *app.App, in DemoExtrasInput) error {
	ctx = tenancy.With(ctx, in.Owner.Workspace.ID.String())
	sending := true
	if err := demoSender(ctx, a, in.Owner.Workspace.ID, in.Owner.Member.ID); err != nil {
		sending = false
		in.Out.Warn("No sending address (%v); email broadcasts are seeded without one and series are skipped.", err)
	}
	if err := brandBroadcasts(ctx, a, in); err != nil {
		return err
	}
	if sending {
		if err := brandSeries(ctx, a, in); err != nil {
			return err
		}
	}
	if err := demoTickets(ctx, a, in); err != nil {
		return err
	}
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := tx.Exec(ctx, backlogHistorySQL, in.Owner.Workspace.ID, 365)
		return err
	})
}

/*
DemoTeam returns the workspace's active teammates, the owner first.
*/
func DemoTeam(ctx context.Context, a *app.App, owner authz.Membership) ([]authz.Membership, error) {
	var team []authz.Membership
	scoped := tenancy.With(ctx, owner.Workspace.ID.String())
	err := a.DB.Transaction(scoped, func(ctx context.Context, _ *database.Tx) error {
		var err error
		team, err = InboxDemoCommand{App: a}.existingTeam(ctx, owner)
		return err
	})
	return team, err
}

/*
brandBroadcasts seeds the topics and the brand's broadcasts with their
recipients, then counts their results.
*/
func brandBroadcasts(ctx context.Context, a *app.App, in DemoExtrasInput) error {
	ws, member := in.Owner.Workspace.ID, in.Owner.Member.ID
	seed := demoSeed{ws: ws, member: member, now: a.Clock.Now().UTC().Format("2006-01-02 15:04:05")}
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if err := seed.topics(ctx, tx); err != nil {
			return err
		}
		for _, b := range demoBrandBroadcasts(in.Brand) {
			if err := seed.send(ctx, tx, b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("seed broadcasts: %w", err)
	}
	for _, id := range seed.made {
		if err := a.Actions.Broadcasts.Tally(ctx, ws, id); err != nil {
			return err
		}
	}
	return nil
}

/*
send seeds one broadcast and, when it went out, who it went to.
*/
func (s *demoSeed) send(ctx context.Context, tx *database.Tx, b demoBroadcast) error {
	id, err := s.broadcast(ctx, tx, b)
	if err != nil {
		return err
	}
	if b.sentShare > 0 {
		if err := s.recipients(ctx, tx, id, b); err != nil {
			return err
		}
	}
	s.made = append(s.made, id)
	return nil
}

/*
brandSeries builds the demo series under the brand's name, sent from
the workspace's first address.
*/
func brandSeries(ctx context.Context, a *app.App, in DemoExtrasInput) error {
	var demos []demoJourney
	raw := strings.NewReplacer("Acme", in.Brand.Name, "acme", in.Brand.Slug).Replace(string(demoSeriesJSON))
	if err := json.Unmarshal([]byte(raw), &demos); err != nil {
		return fmt.Errorf("read demo series: %w", err)
	}
	var address ids.UUID
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return tx.QueryRow(ctx, `SELECT id FROM email_addresses WHERE workspace_id = $1 AND deleted_at IS NULL
			ORDER BY created_at LIMIT 1`, in.Owner.Workspace.ID).Scan(&address)
	})
	if err != nil {
		return fmt.Errorf("find sender address: %w", err)
	}
	for _, demo := range demos {
		if _, err := (SeriesDemoCommand{App: a}).build(ctx, in.Owner, demo, address); err != nil {
			return fmt.Errorf("build series %s: %w", demo.Name, err)
		}
	}
	return nil
}

/*
demoTickets sets up the ticket types and service levels and seeds the
tickets and trackers, on the workspace's own companies.
*/
func demoTickets(ctx context.Context, a *app.App, in DemoExtrasInput) error {
	d := &ticketDemo{app: a, owner: in.Owner, out: in.Out}
	steps := []func(context.Context) error{
		d.loadTeam, d.setUpTypes, d.seedPolicies,
		func(ctx context.Context) error { return d.customerTickets(ctx, in.Tickets) },
		d.trackers,
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return fmt.Errorf("seed tickets: %w", err)
		}
	}
	return nil
}

/*
demoBrandBroadcasts are the broadcasts the brand's team sent this year.
*/
func demoBrandBroadcasts(brand DemoBrand) []demoBroadcast {
	link := func(path string) string { return "https://www." + brand.Domain + path }
	first := `{{ contact.first_name | default: "there" }}`
	return append([]demoBroadcast{
		{
			name: "October product update", channel: "email", status: "sent", daysAgo: 34,
			subject: "New in " + brand.Name + ": faster batches and QR returns",
			preview: "Batches of 1,000 labels in under ten minutes.",
			body: "Hi " + first + ",\n\nBatches now buy labels in parallel, and UK customers can return parcels " +
				"with a QR code.\n\n[See what's new](" + link("/changelog") + ")",
			audience: "paying", bounce: 0.008, open: 0.44, click: 0.12, reply: 0.021, unsub: 0.003, sentShare: 1,
			links: []string{link("/changelog")},
		},
		{
			name: "Peak season checklist", channel: "email", status: "sent", daysAgo: 12,
			subject: "Your peak season shipping checklist", preview: "Five things to set up before Black Friday.",
			body: "Carrier cut-offs move earlier in late November. Here is how to get ready.\n\n" +
				"[Read the checklist](" + link("/help/peak") + ")",
			audience: "paying", bounce: 0.01, open: 0.51, click: 0.19, reply: 0.03, unsub: 0.002, sentShare: 1,
			links: []string{link("/help/peak")},
		},
		{
			name: "Webinar: automation rules in 30 minutes", channel: "email", status: "sent", daysAgo: 3,
			subject: "Live Thursday: let rules pick the carrier for you", preview: "Bring your trickiest orders.",
			body: "Join our support lead live on Thursday at 17:00 UTC.\n\n" +
				"[Save your seat](" + link("/webinars/rules") + ")",
			audience: "free", bounce: 0.04, open: 0.27, click: 0.06, reply: 0.008, unsub: 0.009, sentShare: 1,
			links: []string{link("/webinars/rules")},
		},
		{
			name: "Need a hand with your first labels?", channel: "chat", status: "sent", daysAgo: 6,
			body: "Hi " + first + ", want a quick walkthrough of buying your first labels? " +
				"Reply here and I'll show you.",
			audience: "free", open: 0.58, reply: 0.17, sentShare: 1,
		},
		{
			name: "New plans for 2027", channel: "post", status: "sent", daysAgo: 20,
			subject:  "Our plans for 2027, and what changes for you",
			body:     "Nothing changes for current plans until March.\n\n[Compare plans](" + link("/pricing") + ")",
			audience: "admins", open: 0.61, click: 0.22, reply: 0.05, sentShare: 1, links: []string{link("/pricing")},
		},
	}, plannedBroadcasts()...)
}

/*
plannedBroadcasts are the broadcasts the team has scheduled or is
still writing.
*/
func plannedBroadcasts() []demoBroadcast {
	return []demoBroadcast{
		{
			name: "Annual plan offer", channel: "email", status: "scheduled", daysAgo: -3,
			subject: "Two months free when you switch to annual", preview: "Lock in this year's price.",
			body: "Switch to annual before December and get two months free.", audience: "pro",
		},
		{
			name: "Cyber Monday carrier cut-offs", channel: "email", status: "draft",
			subject: "Ship by these dates for delivery before the holidays",
			body:    "Every carrier's last day to ship for delivery before December 24.", audience: "paying",
		},
	}
}
