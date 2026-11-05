/*
|--------------------------------------------------------------------------
| Command Line
|--------------------------------------------------------------------------
|
| The binary is one command line: serve to run, plus the commands an
| operator needs around it, such as migrating, taking the application
| down for maintenance and showing the effective configuration. Each
| command loads only what it needs: showing the configuration touches no
| database, migrating builds no HTTP stack, and only commands that need
| the whole application boot it. Generators and route linting are for
| developers and exist only in development builds.
|
*/

package foundation

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
Run runs the command line with the arguments, which exclude the
binary's name, and returns the exit code. An application booted for a
command is shut down before Run returns.
*/
func Run(ctx context.Context, opts Options, args []string, streams console.IO) int {
	k := &kernel{opts: opts, streams: streams}
	defer k.close(ctx)

	cli := console.New(cmp.Or(opts.Name, "app"), opts.Version)
	cli.Register(k.commands()...)

	return cli.Run(ctx, args, streams)
}

/*
kernel loads configuration and boots the application at most once for
the commands of one run.
*/
type kernel struct {
	opts    Options
	streams console.IO
	cfg     *Config
	app     *Application
}

/*
commands returns every command of this build.
*/
func (k *kernel) commands() []console.Command {
	commands := []console.Command{
		k.serveCommand(),
		k.migrateCommand(),
		k.migrateStatusCommand(),
		k.migratePlanCommand(),
		k.rollbackCommand(),
		k.downCommand(),
		k.upCommand(),
		k.healthCommand(),
		k.configShowCommand(),
		k.routeListCommand(),
	}

	for _, command := range k.opts.Commands {
		commands = append(commands, k.appCommand(command))
	}

	return append(commands, devCommands(k)...)
}

/*
config loads the configuration once.
*/
func (k *kernel) config(ctx context.Context) (*Config, error) {
	if k.cfg != nil {
		return k.cfg, nil
	}

	cfg, err := LoadConfig(ctx, k.opts)
	if err != nil {
		return nil, err
	}

	k.cfg = cfg

	return cfg, nil
}

/*
application boots the application once, without migrating: only serve
and migrate change the schema.
*/
func (k *kernel) application(ctx context.Context) (*Application, error) {
	if k.app != nil {
		return k.app, nil
	}

	cfg, err := k.config(ctx)
	if err != nil {
		return nil, err
	}

	booted := *cfg
	booted.Migrations.Auto = false

	return k.boot(ctx, &booted)
}

/*
boot boots the application from the configuration and keeps it for
close.
*/
func (k *kernel) boot(ctx context.Context, cfg *Config) (*Application, error) {
	app, err := boot(ctx, k.opts, cfg)
	if err != nil {
		return nil, err
	}

	k.app = app

	return app, nil
}

/*
logger returns a logger for commands that boot no application.
*/
func (k *kernel) logger(cfg *Config) (log.Logger, error) {
	opts := cfg.Log
	opts.Writer = k.opts.LogWriter

	logger, err := log.New(opts)
	if err != nil {
		return nil, fmt.Errorf("start logger: %w", err)
	}

	return logger, nil
}

/*
close shuts a booted application down within its shutdown budget.
*/
func (k *kernel) close(ctx context.Context) {
	if k.app == nil {
		return
	}

	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), k.app.Config.HTTP.ShutdownTimeout)
	defer cancel()

	if err := k.app.Shutdown(stopCtx); err != nil {
		k.app.Log.ErrorContext(ctx, "shutdown failed", log.Err(err))
	}
}

/*
command is a built-in command made of functions.
*/
type command struct {
	name        string
	description string
	flags       func(flags *console.Flags)
	handle      func(ctx context.Context, in *console.Input, out *console.Output) error
}

/*
Name returns the command's name.
*/
func (c command) Name() string {
	return c.name
}

/*
Description returns the command's one-line description.
*/
func (c command) Description() string {
	return c.description
}

/*
Flags declares the command's flags.
*/
func (c command) Flags(flags *console.Flags) {
	if c.flags != nil {
		c.flags(flags)
	}
}

/*
Handle runs the command. A configuration that cannot work is printed
with its secrets masked beside the problems.
*/
func (c command) Handle(ctx context.Context, in *console.Input, out *console.Output) error {
	err := c.handle(ctx, in, out)

	if configErr, ok := errors.AsType[*ConfigError](err); ok {
		printConfig(out.Error, configErr.Redacted)
	}

	return err
}

/*
appCommand runs an application command against the booted application.
*/
func (k *kernel) appCommand(c Command) console.Command {
	return command{
		name:        c.Name(),
		description: c.Description(),
		flags:       c.Flags,
		handle: func(ctx context.Context, in *console.Input, out *console.Output) error {
			app, err := k.application(ctx)
			if err != nil {
				return err
			}

			return c.Handle(ctx, app, in, out)
		},
	}
}

/*
printConfig writes the configuration one dotted key per line, sorted.
*/
func printConfig(write func(format string, args ...any), redacted map[string]any) {
	flat := map[string]any{}
	flatten("", redacted, flat)

	for _, key := range slices.Sorted(maps.Keys(flat)) {
		write("%s = %v", key, flat[key])
	}
}

/*
flatten writes every leaf of the tree under its dotted key.
*/
func flatten(prefix string, tree, into map[string]any) {
	for key, value := range tree {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if branch, ok := value.(map[string]any); ok && len(branch) > 0 {
			flatten(path, branch, into)

			continue
		}

		into[path] = value
	}
}
