/*
|--------------------------------------------------------------------------
| Operator Commands
|--------------------------------------------------------------------------
|
| Serving, maintenance and inspection. Taking production down asks the
| operator to confirm first, since every user is turned away until it
| comes back up; automation passes --force instead. Showing the
| configuration masks every secret, so its output is safe to paste into
| a support thread.
|
*/

package foundation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/pubsub"
	"github.com/dimmihq/dimmi/framework/redis"
)

/*
ErrNotConfirmed means a command that changes production was not
confirmed.
*/
var ErrNotConfirmed = errors.New("foundation: not confirmed")

func (k *kernel) serveCommand() console.Command {
	return command{
		name:        "serve",
		description: "Run the application's roles until stopped",
		flags: func(flags *console.Flags) {
			flags.String("roles", "", "comma-separated roles to run instead of app.roles")
			flags.Bool("no-migrate", "skip migrating on boot")
		},
		handle: func(ctx context.Context, in *console.Input, _ *console.Output) error {
			cfg, err := k.config(ctx)
			if err != nil {
				return err
			}

			served := *cfg
			if in.Bool("no-migrate") {
				served.Migrations.Auto = false
			}

			if roles := in.String("roles"); roles != "" {
				served.App.Roles = parseRoles(roles)
				if err := served.App.Roles.validate(); err != nil {
					return err
				}
			}

			ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
			defer stop()

			app, err := k.boot(ctx, &served)
			if err != nil {
				return err
			}

			return app.Serve(ctx)
		},
	}
}

func parseRoles(value string) Roles {
	var roles Roles
	for role := range strings.SplitSeq(value, ",") {
		roles = append(roles, Role(strings.TrimSpace(role)))
	}

	return roles
}

func (k *kernel) configShowCommand() console.Command {
	return command{
		name:        "config:show",
		description: "Print the effective configuration with secrets masked",
		handle: func(ctx context.Context, _ *console.Input, out *console.Output) error {
			cfg, err := k.config(ctx)
			if err != nil {
				return err
			}

			printConfig(out.Line, config.Redacted(cfg))

			return nil
		},
	}
}

func (k *kernel) routeListCommand() console.Command {
	return command{
		name:        "route:list",
		description: "List every route with its name and middleware",
		handle: func(ctx context.Context, _ *console.Input, out *console.Output) error {
			app, err := k.application(ctx)
			if err != nil {
				return err
			}

			rows := [][]string{}
			for _, route := range app.Router.Routes() {
				rows = append(rows, []string{
					route.Method(), route.Path(), route.Name(), strings.Join(route.MiddlewareNames(), ","),
				})
			}

			out.Table([]string{"Method", "Path", "Name", "Middleware"}, rows)

			return nil
		},
	}
}

func (k *kernel) downCommand() console.Command {
	return command{
		name:        "down",
		description: "Take every replica down for maintenance",
		flags: func(flags *console.Flags) {
			flags.String("message", "", "why the application is down")
			flags.Bool("force", "skip the production confirmation")
		},
		handle: func(ctx context.Context, in *console.Input, out *console.Output) error {
			return k.withMaintenance(ctx, func(cfg *Config, store *MaintenanceStore) error {
				if cfg.App.Env == EnvProduction {
					out.Warn("This turns every user of production away until you run up.")
				}

				err := confirmProduction(ctx, cfg.App.Env, prompt{in: in, out: out}, "Take production down?")
				if err != nil {
					return err
				}

				if err := store.Down(ctx, in.String("message")); err != nil {
					return err
				}

				out.Info("The application is down for maintenance.")

				return nil
			})
		},
	}
}

func (k *kernel) upCommand() console.Command {
	return command{
		name:        "up",
		description: "Bring every replica back from maintenance",
		handle: func(ctx context.Context, _ *console.Input, out *console.Output) error {
			return k.withMaintenance(ctx, func(_ *Config, store *MaintenanceStore) error {
				if err := store.Up(ctx); err != nil {
					return err
				}

				out.Info("The application is up.")

				return nil
			})
		},
	}
}

/*
withMaintenance opens the maintenance state, kept in the database and
announced over Valkey, without booting the application, and runs the
function with it.
*/
func (k *kernel) withMaintenance(ctx context.Context, fn func(cfg *Config, store *MaintenanceStore) error) error {
	cfg, err := k.config(ctx)
	if err != nil {
		return err
	}

	client, err := redis.Open(ctx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("open valkey: %w", err)
	}
	defer client.Close()

	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	store, err := NewMaintenanceStore(ctx, db, pubsub.New(client, cfg.PubSub), clock.System())
	if err != nil {
		return err
	}
	defer store.Close()

	return fn(cfg, store)
}

/*
prompt is the input and output of a command that asks a question.
*/
type prompt struct {
	in  *console.Input
	out *console.Output
}

/*
confirmProduction asks before a command changes production. It passes
outside production and with --force; without someone to answer it
refuses, naming the flag.
*/
func confirmProduction(ctx context.Context, env Environment, p prompt, question string) error {
	if env != EnvProduction || p.in.Bool("force") {
		return nil
	}

	if !p.in.Interactive() {
		return fmt.Errorf("%w: pass --force to run this in production without a prompt", ErrNotConfirmed)
	}

	confirmed, err := p.out.Confirm(ctx, question, false)
	if err != nil {
		return err
	}

	if !confirmed {
		return ErrNotConfirmed
	}

	return nil
}
