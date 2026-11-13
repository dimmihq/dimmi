//go:build dev

/*
|--------------------------------------------------------------------------
| Workspace Demo
|--------------------------------------------------------------------------
|
| One command a new developer runs after cloning to get a workspace that
| looks like a real, busy support team: Stowline, a made-up shipping
| software company, with its help center, team, customers, a year of
| conversations on every channel, its AI agent and the agent's past
| runs, broadcasts, series, tickets and the reports built from all of
| it. The content comes from hand-written text and templates in
| database/seeders/stowline, picked by a seeded random source, so the
| same seed gives the same workspace. It never calls a model or a paid
| embeddings API, never runs in production, and never touches any
| workspace but its own.
|
*/

package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/demo/stowline"
)

/*
WorkspaceDemoCommand seeds the Stowline demo workspace.
*/
type WorkspaceDemoCommand struct {
	App *app.App
}

/*
Name returns workspace:demo.
*/
func (c WorkspaceDemoCommand) Name() string {
	return "workspace:demo"
}

/*
Description says what the command does and how big each size is.
*/
func (c WorkspaceDemoCommand) Description() string {
	sizes := make([]string, 0, len(stowline.Sizes()))
	for _, s := range stowline.Sizes() {
		sizes = append(sizes, fmt.Sprintf("%s: %d conversations, %d people, %d companies, %d teammates",
			s.Name, s.Conversations, s.Contacts, s.Companies, s.Teammates))
	}
	return "Seed Stowline, a realistic demo workspace with a help center, team, customers, a year of conversations " +
		"and an AI agent. The same seed gives the same data. Refuses to run in production or over an earlier " +
		"Stowline workspace unless --reset. Sizes: " + strings.Join(sizes, "; ")
}

/*
Flags sets the size, the seed, whether to replace an earlier workspace
and who owns the new one.
*/
func (c WorkspaceDemoCommand) Flags(flags *console.Flags) {
	flags.String("size", "small", "how much to seed: small, medium or large")
	flags.Int("seed", 7, "the random seed; the same seed gives the same workspace")
	flags.Bool("reset", "purge the earlier Stowline workspace, and only that one, before seeding")
	flags.String("owner-email", "", "make this existing or new account the owner instead of maya@stowline.test")
	flags.String("password", stowline.DefaultPassword, "the password every seeded teammate signs in with")
}

/*
Handle seeds the workspace and the extras the other development demos
build, then prints what it wrote.
*/
func (c WorkspaceDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	began := c.App.Clock.Now()
	size, err := stowline.SizeNamed(in.String("size"))
	if err != nil {
		return err
	}
	seeder := &stowline.Seeder{
		App: c.App, Size: size, Seed: uint64(max(in.Int("seed"), 0)), Reset: in.Bool("reset"),
		OwnerEmail: in.String("owner-email"), Password: in.String("password"), Log: out.Line,
	}
	res, err := seeder.Run(ctx)
	if err != nil {
		return err
	}

	at := c.App.Clock.Now()
	if err := DemoExtras(ctx, c.App, DemoExtrasInput{
		Owner: res.Owner, Tickets: size.Tickets, Out: out,
		Brand: DemoBrand{Name: stowline.Name, Slug: stowline.Slug, Domain: stowline.Domain},
	}); err != nil {
		return err
	}
	out.Line("seeded broadcasts, series, tickets and backlog history in %s",
		c.App.Clock.Now().Sub(at).Round(time.Millisecond))

	counts, err := c.counts(ctx, res)
	if err != nil {
		return err
	}
	rows := make([][]string, 0, len(counts))
	for _, t := range slices.Sorted(func(yield func(string) bool) {
		for k := range counts {
			if !yield(k) {
				return
			}
		}
	}) {
		rows = append(rows, []string{t, fmt.Sprint(counts[t])})
	}
	out.Table([]string{"table", "rows"}, rows)
	owner := seeder.OwnerEmail
	if owner == "" {
		owner = res.Plan.Team[0].Email
	}
	out.Info("Stowline (%s, seed %d) is ready at /w/%s after %s. Sign in as %s with %q.", size.Name, seeder.Seed,
		res.Workspace.Slug, c.App.Clock.Now().Sub(began).Round(time.Second), owner, seeder.Password)
	out.Line("The agent searches the help center by keyword. To build its vectors, which calls the paid " +
		"embeddings API, run knowledge:demo-embed.")
	return nil
}

/*
counts reads back how many rows each seeded table holds.
*/
func (c WorkspaceDemoCommand) counts(ctx context.Context, res *stowline.Result) (map[string]int64, error) {
	out := map[string]int64{}
	tables := append(stowline.CountedTables(), "broadcasts", "broadcast_recipients", "series", "tickets",
		"backlog_snapshots", "conversation_facts")
	err := c.App.DB.Transaction(tenancy.With(ctx, res.Workspace.ID.String()),
		func(ctx context.Context, tx *database.Tx) error {
			for _, t := range tables {
				var n int64
				if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+t+" WHERE workspace_id = $1",
					res.Workspace.ID).Scan(&n); err != nil {
					return fmt.Errorf("count %s: %w", t, err)
				}
				out[t] = n
			}
			return nil
		})
	return out, err
}
