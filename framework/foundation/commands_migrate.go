/*
|--------------------------------------------------------------------------
| Migration Commands
|--------------------------------------------------------------------------
|
| Migrating production is the change most likely to hurt, so before it
| runs the operator sees which database it will touch and every
| migration it will apply, and confirms. Rolling back can drop data, so
| production additionally wants the exact version being undone typed
| back. Automation skips the prompt with --force, and rolling back with
| --force still names the version, so a pipeline never undoes the wrong
| migration. Status and plan only read, and never ask.
|
*/

package foundation

import (
	"bufio"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database/migrate"
)

/*
migrator builds the runner from the configuration.
*/
func (k *kernel) migrator(ctx context.Context) (*Config, migrate.Runner, error) {
	cfg, err := k.config(ctx)
	if err != nil {
		return nil, migrate.Runner{}, err
	}

	logger, err := k.logger(cfg)
	if err != nil {
		return nil, migrate.Runner{}, err
	}

	runner, err := newMigrator(cfg, k.opts.Migrations, logger)

	return cfg, runner, err
}

func (k *kernel) migrateCommand() console.Command {
	return command{
		name:        "migrate",
		description: "Apply every pending migration",
		flags:       func(flags *console.Flags) { flags.Bool("force", "skip the production confirmation") },
		handle: func(ctx context.Context, in *console.Input, out *console.Output) error {
			cfg, runner, err := k.migrator(ctx)
			if err != nil {
				return err
			}

			if cfg.App.Env == EnvProduction && !in.Bool("force") {
				if err := confirmMigrate(ctx, runner, prompt{in: in, out: out}); err != nil {
					return err
				}
			}

			results, err := runner.Up(ctx)
			for _, result := range results {
				out.Info("Migrated %s %d %s (%s)", result.Set, result.Version, result.Name, result.Duration)
			}

			if err != nil {
				return fmt.Errorf("migrate: %w", err)
			}

			if len(results) == 0 {
				out.Info("Nothing to migrate.")
			}

			return nil
		},
	}
}

/*
confirmMigrate shows the database and the pending migrations and asks
before applying them in production.
*/
func confirmMigrate(ctx context.Context, runner migrate.Runner, p prompt) error {
	pending, err := runner.Plan(ctx)
	if err != nil {
		return fmt.Errorf("plan migrations: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	p.out.Warn("Migrating production database %s:", describeDatabase(runner.URL))

	for _, migration := range pending {
		p.out.Warn("  %s %d %s", migration.Set, migration.Version, migration.Name)
	}

	return confirmProduction(ctx, EnvProduction, p, "Apply these migrations?")
}

func (k *kernel) migrateStatusCommand() console.Command {
	return command{
		name:        "migrate:status",
		description: "List every migration and whether it ran",
		handle: func(ctx context.Context, _ *console.Input, out *console.Output) error {
			_, runner, err := k.migrator(ctx)
			if err != nil {
				return err
			}

			migrations, err := runner.Status(ctx)
			if err != nil {
				return fmt.Errorf("migration status: %w", err)
			}

			rows := make([][]string, 0, len(migrations))
			for _, m := range migrations {
				ran := "pending"
				if m.Applied {
					ran = "ran"
				}

				rows = append(rows, []string{m.Set, strconv.FormatInt(m.Version, 10), m.Name, ran})
			}

			out.Table([]string{"Set", "Version", "Name", "Status"}, rows)

			return nil
		},
	}
}

func (k *kernel) migratePlanCommand() console.Command {
	return command{
		name:        "migrate:plan",
		description: "Show what migrate would apply, without applying it",
		handle: func(ctx context.Context, _ *console.Input, out *console.Output) error {
			_, runner, err := k.migrator(ctx)
			if err != nil {
				return err
			}

			pending, err := runner.Plan(ctx)
			if err != nil {
				return fmt.Errorf("plan migrations: %w", err)
			}

			if len(pending) == 0 {
				out.Info("Nothing to migrate.")

				return nil
			}

			for _, p := range pending {
				out.Line("%s %d %s phase=%s transaction=%t", p.Set, p.Version, p.Name, p.Phase, p.Transactional)

				for _, finding := range p.Findings {
					out.Warn("  %s: %s", finding.Rule, finding.Message)
				}
			}

			return nil
		},
	}
}

func (k *kernel) rollbackCommand() console.Command {
	return command{
		name:        "migrate:rollback",
		description: "Undo the newest applied migration",
		flags: func(flags *console.Flags) {
			flags.Bool("force", "skip the production prompt; needs --confirm-version")
			flags.String("confirm-version", "", "the version being undone, required with --force in production")
		},
		handle: func(ctx context.Context, in *console.Input, out *console.Output) error {
			cfg, runner, err := k.migrator(ctx)
			if err != nil {
				return err
			}

			target, found, err := newestApplied(ctx, runner)
			if err != nil {
				return err
			}

			if !found {
				out.Info("Nothing to roll back.")

				return nil
			}

			if cfg.App.Env == EnvProduction {
				if err := k.confirmRollback(runner, target, in, out); err != nil {
					return err
				}

				runner.AllowRollback = true
			}

			result, err := runner.Rollback(ctx)
			if err != nil {
				return fmt.Errorf("roll back: %w", err)
			}

			out.Info("Rolled back %s %d %s", result.Set, result.Version, result.Name)

			return nil
		},
	}
}

/*
confirmRollback wants the version being undone: from --confirm-version
with --force, or typed at the prompt.
*/
func (k *kernel) confirmRollback(
	runner migrate.Runner, target migrate.Migration, in *console.Input, out *console.Output,
) error {
	out.Warn("Rolling back %s %d %s on production database %s. This can drop data.",
		target.Set, target.Version, target.Name, describeDatabase(runner.URL))

	answer := in.String("confirm-version")

	switch {
	case in.Bool("force"):
	case !in.Interactive():
		return fmt.Errorf("%w: pass --force and --confirm-version to roll back production", ErrNotConfirmed)
	default:
		out.Line("Type the version being rolled back to confirm:")

		typed, err := bufio.NewReader(k.streams.In).ReadString('\n')
		if err != nil && typed == "" {
			return fmt.Errorf("%w: no version typed", ErrNotConfirmed)
		}

		answer = typed
	}

	answer = strings.TrimSpace(answer)
	if answer != strconv.FormatInt(target.Version, 10) && answer != target.Name {
		return fmt.Errorf("%w: %q is not the version being rolled back", ErrNotConfirmed, answer)
	}

	return nil
}

/*
newestApplied returns the migration Rollback undoes: the newest applied
migration of the last set that has one.
*/
func newestApplied(ctx context.Context, runner migrate.Runner) (migrate.Migration, bool, error) {
	migrations, err := runner.Status(ctx)
	if err != nil {
		return migrate.Migration{}, false, fmt.Errorf("migration status: %w", err)
	}

	for _, migration := range slices.Backward(migrations) {
		if migration.Applied {
			return migration, true, nil
		}
	}

	return migrate.Migration{}, false, nil
}

/*
describeDatabase names the database at the URL as host:port/name,
without credentials.
*/
func describeDatabase(url string) string {
	cfg, err := pgconn.ParseConfig(url)
	if err != nil {
		return "(unreadable url)"
	}

	return fmt.Sprintf("%s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
}
