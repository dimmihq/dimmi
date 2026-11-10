//go:build dev

/*
|--------------------------------------------------------------------------
| Broadcast Demo
|--------------------------------------------------------------------------
|
| Fills a development workspace with what a real team's broadcasts look
| like, so the screens can be judged on believable data: accounts and
| their people on different plans, topics, a connected address on a
| verified domain, and a history of broadcasts on every channel in every
| state, with the results each would have had. It writes only to the
| workspace it is given and never to a release build.
|
*/

package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BroadcastDemoCommand seeds a workspace with demo broadcasts.
*/
type BroadcastDemoCommand struct {
	App *app.App
}

/*
Name returns broadcasts:demo.
*/
func (c BroadcastDemoCommand) Name() string {
	return "broadcasts:demo"
}

/*
Description says what the command does.
*/
func (c BroadcastDemoCommand) Description() string {
	return "Seed a development workspace with accounts, people and a history of broadcasts"
}

/*
Flags names the workspace and the teammate the broadcasts come from.
*/
func (c BroadcastDemoCommand) Flags(flags *console.Flags) {
	flags.String("workspace", "", "the workspace's public id, ws_...")
	flags.String("member", "", "the teammate's public id, mem_..., or row id")
}

/*
account is one demo company: its name, domain, plan and how many people
work there.
*/
type account struct {
	name, domain, plan string
	people             int
}

/*
demoAccounts are the companies the demo seeds.
*/
func demoAccounts() []account {
	return []account{
		{"Northwind Labs", "northwind.test", "enterprise", 48},
		{"Bluefin Logistics", "bluefin.test", "enterprise", 36},
		{"Cedar Health", "cedarhealth.test", "pro", 24},
		{"Kestrel Finance", "kestrel.test", "pro", 30},
		{"Orbit Studio", "orbit.test", "growth", 12},
		{"Tidewater Retail", "tidewater.test", "growth", 18},
		{"Quill & Co", "quill.test", "pro", 9},
		{"Granite Systems", "granite.test", "enterprise", 40},
	}
}

/*
Handle seeds the workspace.
*/
func (c BroadcastDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	ws, err := c.App.IDs.Decode(string(models.PrefixWorkspace), in.String("workspace"))
	if err != nil {
		return fmt.Errorf("--workspace: %w", err)
	}

	member, err := c.App.IDs.Decode(string(models.PrefixMember), in.String("member"))
	if err != nil {
		if member, err = ids.Parse(in.String("member")); err != nil {
			return fmt.Errorf("--member: %w", err)
		}
	}

	ctx = tenancy.With(ctx, ws.String())
	if err := demoSender(ctx, c.App, ws, member); err != nil {
		return err
	}

	seed := demoSeed{ws: ws, member: member, now: c.App.Clock.Now().UTC().Format("2006-01-02 15:04:05")}

	err = c.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		steps := []func(context.Context, *database.Tx) error{seed.people, seed.topics, seed.history}
		for _, step := range steps {
			if err := step(ctx, tx); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, id := range seed.made {
		if err := c.App.Actions.Broadcasts.Tally(ctx, ws, id); err != nil {
			return err
		}
	}

	out.Line("seeded %d people in %d accounts and %d broadcasts", seed.contacts, len(demoAccounts()), len(seed.made))

	return nil
}

/*
demoSeed is one run: the workspace, the sender, when it runs, and what
it made.
*/
type demoSeed struct {
	ws, member ids.UUID
	now        string
	contacts   int
	made       []ids.UUID
}

/*
people seeds the accounts, their people and a crowd of individual
customers on the free and pro plans.
*/
func (s *demoSeed) people(ctx context.Context, tx *database.Tx) error {
	first := []string{
		"Ada", "Bola", "Chen", "Dara", "Eli", "Fatima", "Gus", "Hana", "Ivan", "Jade", "Kofi", "Lena",
		"Mateo", "Nia", "Omar", "Priya", "Quinn", "Rosa", "Sven", "Tara", "Uche", "Vera", "Wes", "Yara", "Zane",
	}
	last := []string{
		"Okafor", "Lindqvist", "Moreau", "Tanaka", "Silva", "Novak", "Haddad", "Kowalski", "Mensah",
		"Reyes", "Fischer", "Ibrahim", "Larsen", "Costa", "Nakamura", "Adeyemi",
	}
	n := 0
	name := func() string {
		n++

		return first[(n*7)%len(first)] + " " + last[(n*5+3)%len(last)]
	}

	for _, a := range demoAccounts() {
		var company ids.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO companies (workspace_id, name, domain, custom_attributes)
			VALUES ($1, $2, $3, jsonb_build_object('plan', $4::text, 'seats', $5::int)) RETURNING id`,
			s.ws, a.name, a.domain, a.plan, a.people).Scan(&company); err != nil {
			return fmt.Errorf("seed company: %w", err)
		}

		if err := s.staff(ctx, tx, staffing{account: a, company: company, name: name}); err != nil {
			return err
		}
	}

	for i := range 720 {
		p := person{name: name(), plan: []string{"free", "free", "free", "pro"}[i%4], role: "owner"}
		if i%37 != 0 {
			p.email = strings.ToLower(strings.ReplaceAll(p.name, " ", ".")) + fmt.Sprint(i) + "@mail.test"
		}

		if err := s.contact(ctx, tx, p); err != nil {
			return err
		}
	}

	return nil
}

/*
staff seeds the people of one account, its first two as admins.
*/
func (s *demoSeed) staff(ctx context.Context, tx *database.Tx, in staffing) error {
	a := in.account
	for i := range a.people {
		p := person{name: in.name(), plan: a.plan, role: "member", company: &in.company}
		if i < 2 {
			p.role = "admin"
		}

		p.email = strings.ToLower(strings.ReplaceAll(p.name, " ", ".")) + fmt.Sprint(i) + "@" + a.domain

		if err := s.contact(ctx, tx, p); err != nil {
			return err
		}
	}

	return nil
}

/*
staffing is one account to fill: the account, its company row and where
names come from.
*/
type staffing struct {
	account account
	company ids.UUID
	name    func() string
}

/*
person is one demo contact.
*/
type person struct {
	name, email, plan, role string
	company                 *ids.UUID
}

/*
contact seeds one person.
*/
func (s *demoSeed) contact(ctx context.Context, tx *database.Tx, p person) error {
	var address *string
	if p.email != "" {
		address = &p.email
	}

	_, err := tx.Exec(ctx, `WITH added AS (
			INSERT INTO contacts (workspace_id, kind, email, name, company_id, custom_attributes, first_seen_at,
				last_seen_at)
			VALUES ($1, 'user', $2, $3, $4, jsonb_build_object('plan', $5::text, 'role', $6::text),
				now() - (random() * interval '400 days'), now() - (random() * interval '20 days'))
			RETURNING id, email)
		INSERT INTO contact_identities (workspace_id, contact_id, type, value)
		SELECT $1, id, 'email', lower(email::text) FROM added WHERE email IS NOT NULL`,
		s.ws, address, p.name, p.company, p.plan, p.role)
	if err != nil {
		return fmt.Errorf("seed contact: %w", err)
	}

	s.contacts++

	return nil
}

/*
topics seeds the lists people subscribe to.
*/
func (s *demoSeed) topics(ctx context.Context, tx *database.Tx) error {
	_, err := tx.Exec(ctx, `INSERT INTO broadcast_topics (workspace_id, name, description, consent) VALUES
		($1, 'Product updates', 'New features and improvements, about once a month.', 'opt_out'),
		($1, 'Webinars and events', 'Invitations to live sessions and meetups.', 'opt_in'),
		($1, 'Offers', 'Discounts and plan offers.', 'opt_in')
		ON CONFLICT DO NOTHING`, s.ws)
	if err != nil {
		return fmt.Errorf("seed topics: %w", err)
	}

	return nil
}
