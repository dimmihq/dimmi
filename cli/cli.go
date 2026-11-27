/*
Package cli is dimmi's command-line tool, built as the dimmi binary from
cmd/dimmi-cli. It signs in to a dimmi
workspace once, with `dimmi login`, and every command after that uses
the same sign-in, kept in the system keychain.

	dimmi login             sign in to dimmi in the browser
	dimmi logout            forget the sign-in
	dimmi migrate …         move history in from another support tool
	dimmi theme …           build, preview and publish a help center theme
	dimmi version           print the version

Output is short lines for people, or one JSON object a line with --json,
for AI agents. Customer content and keys are never printed.
*/
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/dimmihq/dimmi/cli/connect"
	"github.com/dimmihq/dimmi/cli/dimmi"
	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/secrets"
)

/*
Version is set at build time with
-ldflags "-X github.com/dimmihq/dimmi/cli.Version=...".
*/
var Version = "dev" //nolint:gochecknoglobals // set by the linker at release

var version = func() string { return Version }

const usage = `dimmi is dimmi's command-line tool.

Usage:
  dimmi <command> [flags]

Commands:
  login        Sign in to dimmi, in your browser
  logout       Forget the sign-in
  migrate      Move your history into dimmi from Intercom ("dimmi migrate -h")
  theme        Build, preview and publish your help center's theme ("dimmi theme -h")
  version      Print the version

Flags for every command:
  --json            one JSON object a line, for AI agents
  --profile NAME    keep separate sign-ins and progress per profile (default "default")
`

/*
Run runs the command the arguments name and returns the exit code.
*/
func Run(args []string) int {
	if len(args) < 1 {
		fmt.Fprint(os.Stderr, usage)

		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := dispatch(ctx, args[0], args[1:])
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		return 1
	}

	return 0
}

/*
app holds what every command shares: the profile's folder, its
settings, the keychain and the reporter.
*/
type app struct {
	json    bool
	profile string
	dir     string
	report  engine.Reporter
	keys    secrets.Store
	cfg     config
	out     io.Writer
}

/*
config is the profile's settings, none of them secret.
*/
type config struct {
	DimmiURL  string   `json:"dimmi_url"`
	Source    string   `json:"source"`
	Region    string   `json:"region,omitempty"`
	Account   string   `json:"account,omitempty"`
	Workflows []string `json:"workflows,omitempty"`
}

/*
command is one command: its flags and what it runs once they're read.
*/
type command struct {
	flags *flag.FlagSet
	run   func() error
}

/*
newCommand makes a command with the flags every command takes.
*/
func newCommand(name string, a *app) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.BoolVar(&a.json, "json", false, "one JSON object a line, for AI agents")
	fs.StringVar(&a.profile, "profile", "default", "keep separate sign-ins and progress per profile")

	return fs
}

/*
dispatch finds the command the name gives, reads its flags and runs it.
*/
func dispatch(ctx context.Context, name string, args []string) error {
	a := &app{out: os.Stdout}

	var cmd command

	switch name {
	case "-h", "--help", "help":
		fmt.Print(usage)

		return nil
	case "version", "--version":
		fmt.Println("dimmi", version())

		return nil
	case "login":
		cmd = a.loginCommand(ctx)
	case "logout":
		fs := newCommand("logout", a)
		cmd = command{fs, a.logout}
	case "migrate":
		var err error
		if cmd, args, err = a.migrate(ctx, args); err != nil || cmd.flags == nil {
			return err
		}
	case "theme":
		var err error
		if cmd, args, err = a.theme(ctx, args); err != nil || cmd.flags == nil {
			return err
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", name, usage)

		return errors.New("unknown command")
	}

	if err := cmd.flags.Parse(args); err != nil {
		return err
	}

	if err := a.setup(); err != nil {
		return a.fail(err)
	}

	return a.fail(cmd.run())
}

/*
migrate finds the migrate subcommand the arguments name and returns it
with the arguments left for its flags. It returns no command when it
printed the usage instead.
*/
func (a *app) migrate(ctx context.Context, args []string) (command, []string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Print(migrateUsage)

		return command{}, nil, nil
	}

	cmd, ok := a.migrateCommand(ctx, args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command \"migrate %s\"\n\n%s", args[0], migrateUsage)

		return command{}, nil, errors.New("unknown command")
	}

	return cmd, args[1:], nil
}

/*
multi is a repeatable string flag.
*/
func multi(fs *flag.FlagSet, name, help string) *[]string {
	var v []string

	fs.Func(name, help, func(s string) error {
		v = append(v, s)

		return nil
	})

	return &v
}

func (a *app) setup() error {
	base, err := os.UserConfigDir()
	if err != nil {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}

	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`).MatchString(a.profile) {
		return errors.New("a profile name is letters, digits, - and _")
	}

	a.dir = filepath.Join(base, "dimmi", a.profile)
	if err := os.MkdirAll(a.dir, 0o700); err != nil {
		return err
	}

	if a.json {
		a.report = &engine.JSONReporter{W: os.Stdout}
	} else {
		a.report = &engine.TextReporter{W: os.Stdout}
	}

	a.keys = secrets.Open(a.dir)

	raw, err := os.ReadFile(filepath.Join(a.dir, "config.json"))
	if err == nil {
		_ = json.Unmarshal(raw, &a.cfg)
	}

	if a.cfg.DimmiURL == "" {
		a.cfg.DimmiURL = dimmi.DefaultURL
	}

	if a.cfg.Source == "" {
		a.cfg.Source = "intercom"
	}

	return nil
}

func (a *app) saveConfig() error {
	raw, err := json.MarshalIndent(a.cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(a.dir, "config.json"), raw, 0o600)
}

/*
fail reports an error and returns it.
*/
func (a *app) fail(err error) error {
	if err == nil {
		return nil
	}

	a.report.Report(engine.Event{Event: "error", Message: err.Error()})

	return err
}

func (a *app) say(msg string) {
	a.report.Report(engine.Event{Event: "info", Message: msg})
}

/*
dimmiKey names the saved dimmi sign-in.
*/
func (a *app) dimmiKey() string { return "dimmi" }

/*
transport is the rate-limited transport for one API: share is the part
of its limit a command may use.
*/
func transport(share, rate float64) *http.Client {
	return &http.Client{Transport: engine.NewLimiter(http.DefaultTransport, share, rate), Timeout: 5 * time.Minute}
}

/*
destination builds the dimmi client from the saved sign-in, refreshing
it when it expires.
*/
func (a *app) destination() (*dimmi.Client, error) {
	var t dimmi.Token
	if err := secrets.GetJSON(a.keys, a.dimmiKey(), &t); err != nil || t.AccessToken == "" {
		return nil, errors.New("you're not signed in to dimmi: run `dimmi login` first")
	}

	tokens := &dimmi.SavedTokens{
		Load: func() (dimmi.Token, error) { return t, nil },
		Save: func(next dimmi.Token) error { t = next; return secrets.SetJSON(a.keys, a.dimmiKey(), next) },
	}

	return dimmi.NewClient(t.BaseURL, tokens, engine.NewLimiter(http.DefaultTransport, 0.8, 0), version()), nil
}

/*
signedIn reports whether the profile holds a sign-in to its dimmi.
*/
func (a *app) signedIn() bool {
	var t dimmi.Token

	return secrets.GetJSON(a.keys, a.dimmiKey(), &t) == nil && t.AccessToken != "" && t.BaseURL == a.cfg.DimmiURL
}

func (a *app) loginCommand(ctx context.Context) command {
	fs := newCommand("login", a)
	dimmiURL := fs.String("url", "", "the dimmi to sign in to, https://trydimmi.com unless self-hosted")
	noBrowser := fs.Bool("no-browser", false, "print the address instead of opening the browser")

	return command{fs, func() error {
		if err := a.useURL(*dimmiURL); err != nil {
			return err
		}

		// Logging in always signs in again: a saved sign-in dimmi no
		// longer takes would otherwise count as done, and every command
		// would keep asking for a login that never happens.
		if err := a.keys.Delete(a.dimmiKey()); err != nil {
			return err
		}

		return a.page(ctx, pageOptions{noBrowser: *noBrowser})
	}}
}

/*
useURL switches the profile to another dimmi, such as a self-hosted one.
*/
func (a *app) useURL(raw string) error {
	if raw == "" {
		return nil
	}

	if !connect.Valid(raw) {
		return errors.New("the dimmi address must be an http or https URL")
	}

	a.cfg.DimmiURL = strings.TrimRight(raw, "/")

	return a.saveConfig()
}

/*
pageOptions is what the sign-in page shows: the source's token step when
source is set, and whether to leave the browser closed.
*/
type pageOptions struct {
	source    *connect.Setup
	noBrowser bool
}

/*
page runs the local sign-in page: the dimmi sign-in, and the source's
token step when source is set. It returns once the person finishes.
*/
func (a *app) page(ctx context.Context, opts pageOptions) error {
	srv := &connect.Server{
		DimmiURL:  a.cfg.DimmiURL,
		SaveToken: func(t dimmi.Token) error { return secrets.SetJSON(a.keys, a.dimmiKey(), t) },
		DimmiDone: a.signedIn,
	}

	if opts.source != nil {
		srv.Source = *opts.source
	}

	addr, shutdown, err := srv.Start(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = shutdown() }()

	a.report.Report(engine.Event{
		Event: "connect_page", Message: "Open this page to finish in your browser: " + addr,
		Data: map[string]string{"url": addr, "keys_saved_in": a.keys.Where()},
	})

	if !opts.noBrowser {
		_ = connect.OpenBrowser(ctx, addr)
	}

	select {
	case <-srv.Done():
	case <-ctx.Done():
		return errors.New("stopped before the page was finished")
	}

	a.report.Report(engine.Event{
		Event: "connected", Message: "Done. Keys are saved in " + a.keys.Where() + ".",
		Data: map[string]any{"dimmi": a.cfg.DimmiURL},
	})

	return nil
}

func (a *app) logout() error {
	if err := a.keys.Delete(a.dimmiKey()); err != nil {
		return err
	}

	a.say("Signed out of dimmi.")

	return nil
}
