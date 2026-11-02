//go:build dev

/*
|--------------------------------------------------------------------------
| Development Commands
|--------------------------------------------------------------------------
|
| Generators, route linting and the message key check help while writing
| code and have no use on a server, so they are compiled only into
| development builds, made with the dev build tag. Release builds carry
| neither the commands nor the templates behind them.
|
*/

package foundation

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/console/make"
	"github.com/dimmihq/dimmi/framework/console/stubs"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/routing"
)

/*
devCommands returns the make generators, route:lint and lang:check.
*/
func devCommands(k *kernel) []console.Command {
	generator := make.Generator{
		Stubs:     stubs.FS(),
		Overrides: k.opts.Stubs,
		Module:    k.opts.Module,
		Root:      cmp.Or(k.opts.Root, "."),
		Clock:     clock.System(),
	}

	return append(generator.Commands(), k.routeLintCommand(), k.langCheckCommand())
}

func (k *kernel) routeLintCommand() console.Command {
	return command{
		name:        "route:lint",
		description: "Check every route against the routing rules",
		handle: func(ctx context.Context, _ *console.Input, out *console.Output) error {
			app, err := k.application(ctx)
			if err != nil {
				return err
			}

			problems := routing.Lint(app.Router.Routes(), k.opts.RouteRules)
			for _, problem := range problems {
				out.Error("%s", problem)
			}

			if len(problems) > 0 {
				err := fmt.Errorf("%d route problems", len(problems))

				return &console.ExitError{Code: console.ExitFailure, Err: err}
			}

			out.Info("Every route follows the rules.")

			return nil
		},
	}
}

/*
langCheckCommand fails when a shipped locale lacks a message another
has, or when no code uses a message. It reads the source under the
project root and needs neither the configuration nor the services.
*/
func (k *kernel) langCheckCommand() console.Command {
	return command{
		name:        "lang:check",
		description: "Check every locale has every message and every message is used",
		handle: func(_ context.Context, _ *console.Input, out *console.Output) error {
			report, err := checkLanguage(os.DirFS(cmp.Or(k.opts.Root, ".")), languageFiles(k.opts.Language))
			if err != nil {
				return err
			}

			for _, locale := range slices.Sorted(maps.Keys(report.Missing)) {
				for _, key := range report.Missing[locale] {
					out.Error("%s is missing %s", locale, key)
				}
			}

			for _, key := range report.Unused {
				out.Error("%s is not used", key)
			}

			if !report.Clean() {
				err := fmt.Errorf("%d missing and %d unused messages", count(report.Missing), len(report.Unused))

				return &console.ExitError{Code: console.ExitFailure, Err: err}
			}

			out.Info("Every locale has every message, and every message is used.")

			return nil
		},
	}
}

/*
checkLanguage checks the application's messages beside the framework's.
The framework's own keys count as used, since the framework builds them
at run time from rule names and error codes.
*/
func checkLanguage(code, app fs.FS) (i18n.Report, error) {
	used, err := i18n.ScanKeys(code)
	if err != nil {
		return i18n.Report{}, err
	}

	framework, err := i18n.Keys(frameworkLanguage()...)
	if err != nil {
		return i18n.Report{}, err
	}

	for _, keys := range framework {
		used = append(used, keys...)
	}

	return i18n.Check(used, append(frameworkLanguage(), app)...)
}

/*
count adds up the keys of every locale.
*/
func count(missing map[string][]string) int {
	total := 0
	for _, keys := range missing {
		total += len(keys)
	}

	return total
}
