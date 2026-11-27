//go:build dev

/*
|--------------------------------------------------------------------------
| Database Seed
|--------------------------------------------------------------------------
|
| The single development seeder (`db:seed`). Seeds the Stowline workspace
| (help center with analytics, team, customers, a year of multi-channel
| conversations, AI agents and past runs, broadcasts, series, tickets and
| reports) and the Lumen workspace (curated stories, billing procedures and
| workflows used for product demos and screenshots). Pass --only stowline
| or --only lumen to seed a single workspace, and --embed to also build
| help center vector search with the configured embedding model.
|
*/

package commands

import (
	"context"
	"errors"
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
DBSeedCommand seeds the local development database.
*/
type DBSeedCommand struct {
	App *app.App
}

/*
Name returns db:seed.
*/
func (c DBSeedCommand) Name() string {
	return "db:seed"
}

/*
Description says what the command seeds and how big each size is.
*/
func (c DBSeedCommand) Description() string {
	sizes := make([]string, 0, len(stowline.Sizes()))
	for _, s := range stowline.Sizes() {
		sizes = append(sizes, fmt.Sprintf("%s: %d conversations, %d people, %d companies, %d teammates",
			s.Name, s.Conversations, s.Contacts, s.Companies, s.Teammates))
	}
	return "Seed development workspaces (Stowline and Lumen). Refuses to run in production or over an earlier " +
		"seeded workspace unless --reset. Stowline sizes: " + strings.Join(sizes, "; ")
}

/*
Flags configures workspace selection, Stowline size, seed, reset, credentials
and optional vector embedding.
*/
func (c DBSeedCommand) Flags(flags *console.Flags) {
	flags.String("size", "small", "Stowline size: small, medium or large")
	flags.Int("seed", 7, "random seed for Stowline; the same seed gives the same workspace")
	flags.String("only", "", "seed only one workspace: stowline or lumen (default: both)")
	flags.Bool("reset", "purge earlier seeded workspace(s), and only those, before seeding")
	flags.String("owner-email", "", "owner email for the seeded workspace(s) instead of the defaults")
	flags.String("password", stowline.DefaultPassword, "password every seeded teammate signs in with")
	flags.Bool("embed", "also build vector search for the Stowline help center (requires built-in embeddings)")
	flags.Bool("now", "with --embed, embed in this process instead of queueing on the worker")
}

/*
seedOptions holds validated flags for one db:seed run.
*/
type seedOptions struct {
	stowline   bool
	lumen      bool
	size       stowline.Size
	seed       uint64
	reset      bool
	ownerEmail string
	password   string
	embed      bool
	now        bool
}

/*
parseOptions validates the command's flags before any database write runs.
*/
func (c DBSeedCommand) parseOptions(in *console.Input) (seedOptions, error) {
	only := strings.ToLower(strings.TrimSpace(in.String("only")))
	if only != "" && only != stowline.Slug && only != lumenSlug {
		return seedOptions{}, fmt.Errorf("unknown --only %q: use %s or %s", only, stowline.Slug, lumenSlug)
	}

	opts := seedOptions{
		stowline:   only == "" || only == stowline.Slug,
		lumen:      only == "" || only == lumenSlug,
		seed:       uint64(max(in.Int("seed"), 0)),
		reset:      in.Bool("reset"),
		ownerEmail: in.String("owner-email"),
		password:   in.String("password"),
		embed:      in.Bool("embed"),
		now:        in.Bool("now"),
	}
	if opts.password == "" {
		opts.password = stowline.DefaultPassword
	}
	if opts.embed {
		if !opts.stowline {
			return seedOptions{}, errors.New("--embed requires seeding Stowline; omit --only lumen or run db:embed")
		}
		if err := requireEmbeddingCredentials(c.App); err != nil {
			return seedOptions{}, err
		}
	}
	if opts.stowline {
		size, err := stowline.SizeNamed(in.String("size"))
		if err != nil {
			return seedOptions{}, err
		}
		opts.size = size
	}
	return opts, nil
}

/*
Handle validates inputs and runs the requested workspace seeders.
*/
func (c DBSeedCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	if c.App.Application.Config.App.Env == foundation.EnvProduction {
		return stowline.ErrProduction
	}

	opts, err := c.parseOptions(in)
	if err != nil {
		return err
	}

	if opts.stowline {
		if err := c.runStowline(ctx, opts, out); err != nil {
			return err
		}
	}

	if opts.lumen {
		lumen := lumenSeeder{
			App: c.App, Reset: opts.reset, OwnerEmail: opts.ownerEmail,
			Password: opts.password, Out: out,
		}
		if err := lumen.run(ctx); err != nil {
			return err
		}
	}

	return nil
}

/*
runStowline seeds Stowline, its extras (broadcasts, series, tickets, backlog
history and help center analytics) and optional vector embeddings.
*/
func (c DBSeedCommand) runStowline(ctx context.Context, opts seedOptions, out *console.Output) error {
	began := c.App.Clock.Now()
	seeder := &stowline.Seeder{
		App: c.App, Size: opts.size, Seed: opts.seed, Reset: opts.reset,
		OwnerEmail: opts.ownerEmail, Password: opts.password, Log: out.Line,
	}
	res, err := seeder.Run(ctx)
	if err != nil {
		return err
	}

	at := c.App.Clock.Now()
	if err := DemoExtras(ctx, c.App, DemoExtrasInput{
		Owner: res.Owner, Tickets: opts.size.Tickets, Out: out,
		Brand: DemoBrand{Name: stowline.Name, Slug: stowline.Slug, Domain: stowline.Domain},
	}); err != nil {
		return err
	}
	out.Line("seeded broadcasts, series, tickets, backlog history and help center analytics in %s",
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
	out.Info("Stowline (%s, seed %d) is ready at /w/%s after %s. Sign in as %s with %q.", opts.size.Name, seeder.Seed,
		res.Workspace.Slug, c.App.Clock.Now().Sub(began).Round(time.Second), owner, seeder.Password)

	if opts.embed {
		return embedStowline(ctx, c.App, embedOptions{Out: out, Now: opts.now})
	}
	out.Line("The agent searches the help center by keyword. To build vector search, run db:embed or pass --embed.")
	return nil
}

/*
counts reads back how many rows each seeded Stowline table holds.
*/
func (c DBSeedCommand) counts(ctx context.Context, res *stowline.Result) (map[string]int64, error) {
	out := map[string]int64{}
	tables := append(stowline.CountedTables(), "broadcasts", "broadcast_recipients", "series", "tickets",
		"backlog_snapshots", "conversation_facts", "article_views", "help_search_terms", "changelog_entry_views")
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
