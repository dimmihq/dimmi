package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/secrets"
	"github.com/dimmihq/dimmi/cli/sources/intercom"
)

const migrateUsage = `dimmi migrate moves your history into dimmi from another support tool.

Usage:
  dimmi migrate <command> [flags]

Commands:
  connect      Connect Intercom (and sign in to dimmi if you haven't), in your browser
  plan         Count what will come across, and what won't
  run          Import everything, resuming where it stopped
  sync         Bring over what changed since the last run (--watch 15m repeats)
  verify       Compare counts and sample records against dimmi
  status       Show progress
  rollback     Undo everything this migration created in dimmi
  import FILE  Import a JSON Lines file already in dimmi's import format
  disconnect   Forget the saved Intercom token
  inventory    Report the shapes of the workspace's data, never its content

Every command takes --json and --profile.
`

/*
migrateCommand returns the migrate subcommand by name.
*/
func (a *app) migrateCommand(ctx context.Context, name string) (command, bool) {
	fs := newCommand("migrate "+name, a)

	switch name {
	case "connect":
		return a.connectCommand(ctx, fs), true
	case "plan":
		workflows := multi(fs, "workflow", "an Intercom workflow URL to bring, repeatable")

		return command{fs, func() error { return a.plan(ctx, *workflows) }}, true
	case "run":
		return a.runCommand(ctx, fs), true
	case "sync":
		watch := fs.Duration("watch", 0, "repeat the sync this often, such as 15m, until stopped")

		return command{fs, func() error { return a.run(ctx, runOptions{sync: true, watch: *watch}) }}, true
	case "verify":
		sample := fs.Int("sample", 50, "records to look up in dimmi per kind")

		return command{fs, func() error { return a.verify(ctx, *sample) }}, true
	case "status":
		return command{fs, func() error { return a.status(ctx) }}, true
	case "rollback":
		yes := fs.Bool("yes", false, "don't ask for confirmation")

		return command{fs, func() error { return a.rollback(ctx, rollbackOptions{yes: *yes}) }}, true
	case "import":
		return a.importCommand(ctx, fs), true
	case "disconnect":
		return command{fs, a.disconnect}, true
	case "inventory":
		out := fs.String("out", "", "a folder to write scrubbed payload shapes to")
		visitor := fs.String("visitor-id", "", "an anonymous visitor's intercom-id cookie to look for in the API")

		return command{fs, func() error { return a.inventory(ctx, *out, *visitor) }}, true
	}

	return command{}, false
}

/*
connectCommand is `dimmi migrate connect`.
*/
func (a *app) connectCommand(ctx context.Context, fs *flag.FlagSet) command {
	dimmiURL := fs.String("dimmi-url", "", "the dimmi to move into, https://trydimmi.com unless self-hosted")
	source := fs.String("source", "intercom", "the tool to move from")
	noBrowser := fs.Bool("no-browser", false, "print the address instead of opening the browser")

	return command{fs, func() error { return a.connect(ctx, *dimmiURL, *source, *noBrowser) }}
}

/*
runCommand is `dimmi migrate run`.
*/
func (a *app) runCommand(ctx context.Context, fs *flag.FlagSet) command {
	skip := fs.String("skip", "", "kinds to leave out, comma-separated, such as contact_note,ticket")
	only := fs.String("only", "", "import only these kinds, comma-separated")
	workflows := multi(fs, "workflow", "an Intercom workflow URL to bring, repeatable")

	return command{fs, func() error {
		return a.run(ctx, runOptions{skip: *skip, only: *only, workflows: *workflows})
	}}
}

/*
importCommand is `dimmi migrate import FILE`.
*/
func (a *app) importCommand(ctx context.Context, fs *flag.FlagSet) command {
	tool := fs.String("source-name", "", "the tool the file came from, such as zendesk")
	account := fs.String("account", "", "the account in that tool, so reruns resume")

	return command{fs, func() error {
		if fs.NArg() != 1 {
			return errors.New("name the file: dimmi migrate import records.jsonl")
		}

		return a.importFile(ctx, fs.Arg(0), *tool, *account)
	}}
}

/*
sourceKey names the saved source token.
*/
func (a *app) sourceKey() string { return a.cfg.Source }

/*
connect saves the source's token, signing in to dimmi first on the same
page when the profile isn't signed in yet.
*/
func (a *app) connect(ctx context.Context, dimmiURL, source string, noBrowser bool) error {
	if source != "intercom" {
		return fmt.Errorf("dimmi migrate connects to intercom; for other tools, use `dimmi migrate import`")
	}

	if err := a.useURL(dimmiURL); err != nil {
		return err
	}

	a.cfg.Source = source
	if err := a.saveConfig(); err != nil {
		return err
	}

	setup := intercom.Setup(
		func(region, token string) error {
			c, err := intercom.NewClient(nil, region, token)
			if err != nil {
				return err
			}

			acct, err := intercom.New(c, region, nil).Account(ctx)
			if err != nil {
				return err
			}

			a.cfg.Region, a.cfg.Account = region, acct.ID
			if err := a.saveConfig(); err != nil {
				return err
			}

			return secrets.SetJSON(a.keys, a.sourceKey(), sourceToken{Region: region, Token: token, Account: acct.Name})
		},
		func() (string, bool) {
			var t sourceToken
			if secrets.GetJSON(a.keys, a.sourceKey(), &t) != nil || t.Token == "" {
				return "", false
			}

			return t.Account, true
		},
	)

	return a.page(ctx, pageOptions{source: &setup, noBrowser: noBrowser})
}

/*
disconnect forgets the source token. `dimmi logout` forgets the dimmi
sign-in.
*/
func (a *app) disconnect() error {
	if err := a.keys.Delete(a.sourceKey()); err != nil {
		return err
	}

	a.say("Forgot the saved Intercom token.")

	return nil
}

/*
sourceToken is what's saved for a source.
*/
type sourceToken struct {
	Region  string `json:"region"`
	Token   string `json:"token"`
	Account string `json:"account"`
}

/*
source builds the Intercom source from the saved token. It stays under
30% of Intercom's shared limit, so the workspace's other apps keep
working.
*/
func (a *app) source(workflows []string) (*intercom.Source, error) {
	var t sourceToken
	if err := secrets.GetJSON(a.keys, a.sourceKey(), &t); err != nil {
		return nil, errors.New("intercom isn't connected: run `dimmi migrate connect` first")
	}

	c, err := intercom.NewClient(transport(0.3, 0), t.Region, t.Token)
	if err != nil {
		return nil, err
	}

	ids := append([]string{}, a.cfg.Workflows...)

	for _, w := range workflows {
		id, ok := intercom.WorkflowID(w)
		if !ok {
			return nil, fmt.Errorf("%q isn't a workflow URL: copy it from the workflow's page in Intercom", w)
		}

		if !contains(ids, id) {
			ids = append(ids, id)
		}
	}

	if len(workflows) > 0 {
		a.cfg.Workflows = ids
		_ = a.saveConfig()
	}

	return intercom.New(c, t.Region, ids), nil
}

func contains(list []string, s string) bool {
	return slices.Contains(list, s)
}

/*
engineFor builds the engine with the checkpoint of the source's account,
sending to the signed-in dimmi.
*/
func (a *app) engineFor(ctx context.Context, src engine.Source) (*engine.Engine, error) {
	acct, err := src.Account(ctx)
	if err != nil {
		return nil, err
	}

	store, err := engine.OpenStore(filepath.Join(a.dir, "migrations", safe(src.Name()+"-"+acct.ID)))
	if err != nil {
		return nil, err
	}

	dest, err := a.destination()
	if err != nil {
		return nil, err
	}

	return &engine.Engine{Source: src, Store: store, Report: a.report, Dest: dest}, nil
}

/*
closeStore closes the engine's checkpoint once a command is done with it.
*/
func closeStore(e *engine.Engine) {
	_ = e.Store.Close()
}

func safe(s string) string {
	return regexp.MustCompile(`[^A-Za-z0-9_.-]+`).ReplaceAllString(s, "_")
}
