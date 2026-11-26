/*
Command dimmi is dimmi's command-line tool. It signs in to a dimmi
workspace once, with `dimmi login`, and every command after that uses
the same sign-in, kept in the system keychain.

	dimmi login             sign in to dimmi in the browser
	dimmi logout            forget the sign-in
	dimmi migrate …         move history in from another support tool
	dimmi version           print the version

Output is short lines for people, or one JSON object a line with --json,
for AI agents. Customer content and keys are never printed.
*/
package main

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

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/connect"
	"github.com/dimmihq/dimmi/cmd/dimmi-cli/dimmi"
	"github.com/dimmihq/dimmi/cmd/dimmi-cli/engine"
	"github.com/dimmihq/dimmi/cmd/dimmi-cli/secrets"
)

/*
version is set at build time with -ldflags "-X main.version=...".
*/
var version = "dev"

const usage = `dimmi is dimmi's command-line tool.

Usage:
  dimmi <command> [flags]

Commands:
  login        Sign in to dimmi, in your browser
  logout       Forget the sign-in
  migrate      Move your history into dimmi from Intercom ("dimmi migrate -h")
  version      Print the version

Flags for every command:
  --json            one JSON object a line, for AI agents
  --profile NAME    keep separate sign-ins and progress per profile (default "default")
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := dispatch(ctx, os.Args[1], os.Args[2:])
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		os.Exit(1)
	}
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

func dispatch(ctx context.Context, name string, args []string) error {
	a := &app{out: os.Stdout}

	var cmd command

	switch name {
	case "-h", "--help", "help":
		fmt.Print(usage)

		return nil
	case "version", "--version":
		fmt.Println("dimmi", version)

		return nil
	case "login":
		cmd = a.loginCommand(ctx)
	case "logout":
		fs := newCommand("logout", a)
		cmd = command{fs, a.logout}
	case "migrate":
		if len(args) == 0 || strings.HasPrefix(args[0], "-") {
			fmt.Print(migrateUsage)

			return nil
		}

		var ok bool
		if cmd, ok = a.migrateCommand(ctx, args[0]); !ok {
			fmt.Fprintf(os.Stderr, "unknown command \"migrate %s\"\n\n%s", args[0], migrateUsage)

			return errors.New("unknown command")
		}

		args = args[1:]
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

func (a *app) say(msg string, data any) {
	a.report.Report(engine.Event{Event: "info", Message: msg, Data: data})
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

	return dimmi.NewClient(t.BaseURL, tokens, engine.NewLimiter(http.DefaultTransport, 0.8, 0), version), nil
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

		return a.page(ctx, nil, *noBrowser)
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
page runs the local sign-in page: the dimmi sign-in, and the source's
token step when source is set. It returns once the person finishes.
*/
func (a *app) page(ctx context.Context, source *connect.Setup, noBrowser bool) error {
	srv := &connect.Server{
		DimmiURL:  a.cfg.DimmiURL,
		SaveToken: func(t dimmi.Token) error { return secrets.SetJSON(a.keys, a.dimmiKey(), t) },
		DimmiDone: a.signedIn,
	}

	if source != nil {
		srv.Source = *source
	}

	addr, shutdown, err := srv.Start(ctx)
	if err != nil {
		return err
	}
	defer shutdown()

	a.report.Report(engine.Event{Event: "connect_page", Message: "Open this page to finish in your browser: " + addr,
		Data: map[string]string{"url": addr, "keys_saved_in": a.keys.Where()}})

	if !noBrowser {
		_ = connect.OpenBrowser(addr)
	}

	select {
	case <-srv.Done():
	case <-ctx.Done():
		return errors.New("stopped before the page was finished")
	}

	a.report.Report(engine.Event{Event: "connected", Message: "Done. Keys are saved in " + a.keys.Where() + ".",
		Data: map[string]any{"dimmi": a.cfg.DimmiURL}})

	return nil
}

func (a *app) logout() error {
	if err := a.keys.Delete(a.dimmiKey()); err != nil {
		return err
	}

	a.say("Signed out of dimmi.", nil)

	return nil
}
