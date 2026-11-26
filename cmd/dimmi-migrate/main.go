/*
Command dimmi-migrate moves a workspace's history into dimmi from
another support tool. It runs on the customer's own machine: it reads
the other tool with the customer's own token and writes to dimmi's
migrations API, so the data goes straight from one to the other.

	dimmi-migrate connect      sign in to dimmi and paste the source token, in the browser
	dimmi-migrate plan         count what will come across, and what won't
	dimmi-migrate run          import everything, resuming where it stopped
	dimmi-migrate sync         bring over what changed since; --watch 15m repeats it
	dimmi-migrate verify       compare counts and sample records against dimmi
	dimmi-migrate status       show progress
	dimmi-migrate rollback     undo everything the migration created in dimmi
	dimmi-migrate import FILE  import a JSON Lines file already in dimmi's format

Output is short lines for people, or one JSON object a line with --json,
for AI agents. Customer content and keys are never printed.
*/
package main

import (
	"bufio"
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
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/connect"
	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/dimmi"
	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/engine"
	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/format"
	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/secrets"
	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/sources/file"
	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/sources/intercom"
)

/*
version is set at build time with -ldflags "-X main.version=...".
*/
var version = "dev"

const usage = `dimmi-migrate moves your history into dimmi from another support tool.

Usage:
  dimmi-migrate <command> [flags]

Commands:
  connect      Sign in to dimmi and connect Intercom, in your browser
  plan         Count what will come across, and what won't
  run          Import everything, resuming where it stopped
  sync         Bring over what changed since the last run (--watch 15m repeats)
  verify       Compare counts and sample records against dimmi
  status       Show progress
  rollback     Undo everything this migration created in dimmi
  import FILE  Import a JSON Lines file already in dimmi's import format
  disconnect   Forget the saved keys
  version      Print the version

Flags for every command:
  --json            one JSON object a line, for AI agents
  --profile NAME    keep separate keys and progress per profile (default "default")

Run "dimmi-migrate <command> -h" for a command's own flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd, args := os.Args[1], os.Args[2:]

	err := dispatch(ctx, cmd, args)
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		os.Exit(1)
	}
}

/*
app holds what every command shares.
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
	Skip      []string `json:"skip,omitempty"`
}

func dispatch(ctx context.Context, cmd string, args []string) error {
	switch cmd {
	case "-h", "--help", "help":
		fmt.Print(usage)

		return nil
	case "version", "--version":
		fmt.Println("dimmi-migrate", version)

		return nil
	}

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	a := &app{out: os.Stdout}
	fs.BoolVar(&a.json, "json", false, "one JSON object a line, for AI agents")
	fs.StringVar(&a.profile, "profile", "default", "keep separate keys and progress per profile")

	var run func() error

	switch cmd {
	case "connect":
		dimmiURL := fs.String("dimmi-url", "", "the dimmi to move into, https://trydimmi.com unless self-hosted")
		source := fs.String("source", "intercom", "the tool to move from")
		noBrowser := fs.Bool("no-browser", false, "print the address instead of opening the browser")
		run = func() error { return a.connect(ctx, *dimmiURL, *source, *noBrowser) }
	case "plan":
		workflows := multi(fs, "workflow", "an Intercom workflow URL to bring, repeatable")
		run = func() error { return a.plan(ctx, *workflows) }
	case "run":
		skip := fs.String("skip", "", "kinds to leave out, comma-separated, such as contact_note,ticket")
		only := fs.String("only", "", "import only these kinds, comma-separated")
		workflows := multi(fs, "workflow", "an Intercom workflow URL to bring, repeatable")
		run = func() error { return a.run(ctx, *skip, *only, *workflows, false, 0) }
	case "sync":
		watch := fs.Duration("watch", 0, "repeat the sync this often, such as 15m, until stopped")
		run = func() error { return a.run(ctx, "", "", nil, true, *watch) }
	case "verify":
		sample := fs.Int("sample", 50, "records to look up in dimmi per kind")
		run = func() error { return a.verify(ctx, *sample) }
	case "status":
		run = func() error { return a.status(ctx) }
	case "rollback":
		yes := fs.Bool("yes", false, "don't ask for confirmation")
		run = func() error { return a.rollback(ctx, *yes) }
	case "import":
		tool := fs.String("source-name", "", "the tool the file came from, such as zendesk")
		account := fs.String("account", "", "the account in that tool, so reruns resume")
		run = func() error {
			if fs.NArg() != 1 {
				return errors.New("name the file: dimmi-migrate import records.jsonl")
			}

			return a.importFile(ctx, fs.Arg(0), *tool, *account)
		}
	case "disconnect":
		run = func() error { return a.disconnect() }
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)

		return errors.New("unknown command")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := a.setup(); err != nil {
		return a.fail(err)
	}

	return a.fail(run())
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

	a.dir = filepath.Join(base, "dimmi-migrate", a.profile)
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
The names the keys are saved under.
*/
func (a *app) dimmiKey() string  { return "dimmi" }
func (a *app) sourceKey() string { return a.cfg.Source }

/*
sourceToken is what's saved for a source.
*/
type sourceToken struct {
	Region  string `json:"region"`
	Token   string `json:"token"`
	Account string `json:"account"`
}

func (a *app) connect(ctx context.Context, dimmiURL, source string, noBrowser bool) error {
	if source != "intercom" {
		return fmt.Errorf("dimmi-migrate connects to intercom; for other tools, use `dimmi-migrate import`")
	}

	if dimmiURL != "" {
		if !connect.Valid(dimmiURL) {
			return errors.New("--dimmi-url must be an http or https address")
		}

		a.cfg.DimmiURL = strings.TrimRight(dimmiURL, "/")
	}

	a.cfg.Source = source
	if err := a.saveConfig(); err != nil {
		return err
	}

	srv := &connect.Server{
		DimmiURL: a.cfg.DimmiURL,
		SaveToken: func(t dimmi.Token) error {
			return secrets.SetJSON(a.keys, a.dimmiKey(), t)
		},
		DimmiDone: func() bool {
			var t dimmi.Token

			return secrets.GetJSON(a.keys, a.dimmiKey(), &t) == nil && t.AccessToken != "" && t.BaseURL == a.cfg.DimmiURL
		},
		Source: intercom.Setup(
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
		),
	}

	addr, shutdown, err := srv.Start(ctx)
	if err != nil {
		return err
	}
	defer shutdown()

	a.report.Report(engine.Event{Event: "connect_page", Message: "Open this page to connect dimmi and Intercom: " + addr,
		Data: map[string]string{"url": addr, "keys_saved_in": a.keys.Where()}})

	if !noBrowser {
		_ = connect.OpenBrowser(addr)
	}

	select {
	case <-srv.Done():
	case <-ctx.Done():
		return errors.New("connect stopped before both keys were saved")
	}

	a.report.Report(engine.Event{Event: "connected", Message: "Connected. Keys are saved in " + a.keys.Where() + ".",
		Data: map[string]any{"dimmi": a.cfg.DimmiURL, "source": a.cfg.Source}})

	return nil
}

func (a *app) disconnect() error {
	if err := a.keys.Delete(a.dimmiKey()); err != nil {
		return err
	}

	if err := a.keys.Delete(a.sourceKey()); err != nil {
		return err
	}

	a.say("Forgot the saved keys.", nil)

	return nil
}

/*
transport is the rate-limited transport for one API: share is the part
of its limit a migration may use.
*/
func transport(share, rate float64) *http.Client {
	return &http.Client{Transport: engine.NewLimiter(http.DefaultTransport, share, rate), Timeout: 5 * time.Minute}
}

/*
source builds the Intercom source from the saved token. It stays under
30% of Intercom's shared limit, so the workspace's other apps keep
working.
*/
func (a *app) source(workflows []string) (*intercom.Source, error) {
	var t sourceToken
	if err := secrets.GetJSON(a.keys, a.sourceKey(), &t); err != nil {
		return nil, errors.New("Intercom isn't connected: run `dimmi-migrate connect` first")
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
	for _, v := range list {
		if v == s {
			return true
		}
	}

	return false
}

/*
destination builds the dimmi client from the saved sign-in.
*/
func (a *app) destination() (*dimmi.Client, error) {
	var t dimmi.Token
	if err := secrets.GetJSON(a.keys, a.dimmiKey(), &t); err != nil || t.AccessToken == "" {
		return nil, errors.New("dimmi isn't connected: run `dimmi-migrate connect` first")
	}

	tokens := &dimmi.SavedTokens{
		Load: func() (dimmi.Token, error) { return t, nil },
		Save: func(next dimmi.Token) error { t = next; return secrets.SetJSON(a.keys, a.dimmiKey(), next) },
	}

	return dimmi.NewClient(t.BaseURL, tokens, engine.NewLimiter(http.DefaultTransport, 0.8, 0), version), nil
}

/*
engineFor builds the engine with the checkpoint of the source's account.
*/
func (a *app) engineFor(ctx context.Context, src engine.Source, needDest bool) (*engine.Engine, error) {
	acct, err := src.Account(ctx)
	if err != nil {
		return nil, err
	}

	store, err := engine.OpenStore(filepath.Join(a.dir, "migrations", safe(src.Name()+"-"+acct.ID)))
	if err != nil {
		return nil, err
	}

	e := &engine.Engine{Source: src, Store: store, Report: a.report}

	if needDest {
		dest, err := a.destination()
		if err != nil {
			return nil, err
		}

		e.Dest = dest
	}

	return e, nil
}

func safe(s string) string {
	return regexp.MustCompile(`[^A-Za-z0-9_.-]+`).ReplaceAllString(s, "_")
}

func (a *app) plan(ctx context.Context, workflows []string) error {
	src, err := a.source(workflows)
	if err != nil {
		return err
	}

	e := &engine.Engine{Source: src}

	inv, err := e.Plan(ctx)
	if err != nil {
		return err
	}

	if a.json {
		a.report.Report(engine.Event{Event: "plan", Data: inv})

		return nil
	}

	w := bufio.NewWriter(a.out)
	defer w.Flush()

	fmt.Fprintf(w, "Intercom workspace %s (%s, %s region)\n\n", inv.Account.Name, inv.Account.ID, strings.ToUpper(inv.Account.Region))
	fmt.Fprintln(w, "Comes across:")

	for _, k := range inv.Kinds {
		count := fmt.Sprint(k.Count)
		if k.Estimate {
			count = "about " + count
		}

		if len(k.Notes) > 0 && k.Count == 0 && !k.Estimate {
			count = "-"
		}

		fmt.Fprintf(w, "  %-24s %10s", strings.ReplaceAll(string(k.Kind), "_", " ")+"s", count)

		if len(k.Notes) > 0 {
			fmt.Fprintf(w, "   %s", strings.Join(k.Notes, "; "))
		}

		fmt.Fprintln(w)
	}

	if len(inv.Missing) > 0 {
		fmt.Fprintln(w, "\nThe Intercom token lacks these permissions; add them to the app and run plan again:")

		for _, m := range inv.Missing {
			fmt.Fprintln(w, "  - "+m)
		}
	}

	fmt.Fprintln(w, "\nStays behind:")

	for _, n := range inv.NotImported {
		fmt.Fprintln(w, "  - "+n)
	}

	for _, warn := range inv.Warnings {
		fmt.Fprintln(w, "\n! "+warn)
	}

	fmt.Fprintln(w, "\nNothing imported sends an email or notification, or starts a workflow.")
	fmt.Fprintln(w, "Leave out a kind with `dimmi-migrate run --skip contact_note,ticket`.")

	return nil
}

func kinds(list string) ([]format.Kind, error) {
	var out []format.Kind

	for _, k := range strings.Split(list, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}

		kind := format.Kind(strings.TrimSuffix(k, "s"))
		if !kind.Valid() {
			kind = format.Kind(k)
		}

		if !kind.Valid() {
			names := make([]string, 0)
			for _, known := range format.Kinds() {
				names = append(names, string(known))
			}

			sort.Strings(names)

			return nil, fmt.Errorf("unknown kind %q: use %s", k, strings.Join(names, ", "))
		}

		out = append(out, kind)
	}

	return out, nil
}

func (a *app) run(ctx context.Context, skip, only string, workflows []string, sync bool, watch time.Duration) error {
	skipKinds, err := kinds(skip)
	if err != nil {
		return err
	}

	onlyKinds, err := kinds(only)
	if err != nil {
		return err
	}

	src, err := a.source(workflows)
	if err != nil {
		return err
	}

	e, err := a.engineFor(ctx, src, true)
	if err != nil {
		return err
	}
	defer e.Store.Close()

	for {
		err := e.Run(ctx, engine.RunOptions{Skip: skipKinds, Only: onlyKinds, Sync: sync})
		if err != nil || watch <= 0 {
			return err
		}

		a.report.Report(engine.Event{Event: "waiting", Message: fmt.Sprintf("next sync in %s; Ctrl-C stops", watch)})

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(watch):
		}
	}
}

func (a *app) importFile(ctx context.Context, path, tool, account string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	src := &file.Source{Path: path, Tool: tool, AccountID: account}

	e, err := a.engineFor(ctx, src, true)
	if err != nil {
		return err
	}
	defer e.Store.Close()

	return e.Run(ctx, engine.RunOptions{})
}

func (a *app) verify(ctx context.Context, sample int) error {
	src, err := a.source(nil)
	if err != nil {
		return err
	}

	e, err := a.engineFor(ctx, src, true)
	if err != nil {
		return err
	}
	defer e.Store.Close()

	v, err := e.Verify(ctx, sample)
	if err != nil {
		return err
	}

	if !a.json {
		w := bufio.NewWriter(a.out)
		fmt.Fprintf(w, "\n%-24s %8s %8s %8s %8s\n", "", "Intercom", "dimmi", "sampled", "missing")

		for _, k := range v.Kinds {
			mark := "✓"
			if !k.OK {
				mark = "✗"
			}

			fmt.Fprintf(w, "%s %-22s %8d %8d %8d %8d\n", mark, strings.ReplaceAll(string(k.Kind), "_", " ")+"s", k.Source, k.Dimmi, k.Sampled, len(k.Missing))
		}

		_ = w.Flush()
	}

	a.report.Report(engine.Event{Event: "verification", Message: verdict(v.OK), Data: v})

	if !v.OK {
		return errors.New("verify found differences")
	}

	return nil
}

func verdict(ok bool) string {
	if ok {
		return "Everything checked is in dimmi."
	}

	return "Some records are missing in dimmi: run `dimmi-migrate sync` and verify again."
}

func (a *app) status(ctx context.Context) error {
	src, err := a.source(nil)
	if err != nil {
		return err
	}

	e, err := a.engineFor(ctx, src, true)
	if err != nil {
		return err
	}
	defer e.Store.Close()

	st, err := e.Status(ctx)
	if err != nil {
		return err
	}

	if st.State == nil {
		a.say("No migration has run from this machine yet. Start with `dimmi-migrate plan`.", nil)

		return nil
	}

	if a.json {
		a.report.Report(engine.Event{Event: "status", Data: st})

		return nil
	}

	w := bufio.NewWriter(a.out)
	defer w.Flush()

	fmt.Fprintf(w, "Migration %s from %s %s\n\n", st.State.MigrationID, st.State.Source, st.State.Account.Name)

	for _, k := range src.Kinds() {
		ks := st.State.Kinds[k]
		state := "waiting"

		switch {
		case st.State.Skips(k):
			state = "skipped"
		case ks == nil:
		case ks.Done:
			state = "done"
		case !ks.StartedAt.IsZero():
			state = "in progress"
		}

		line := fmt.Sprintf("  %-24s %-12s", strings.ReplaceAll(string(k), "_", " ")+"s", state)
		if st.Migration != nil {
			c := st.Migration.Counts[k]
			line += fmt.Sprintf(" %d in dimmi", c.Total())

			if c.Failed > 0 {
				line += fmt.Sprintf(", %d failed", c.Failed)
			}
		}

		fmt.Fprintln(w, line)
	}

	if !st.State.SyncedAt.IsZero() {
		fmt.Fprintf(w, "\nLast sync %s\n", st.State.SyncedAt.Local().Format(time.RFC1123))
	}

	if n := len(st.State.Failures); n > 0 {
		fmt.Fprintf(w, "\n%d records were refused. Ids and reasons:\n", n)

		for i, f := range st.State.Failures {
			if i == 20 {
				fmt.Fprintf(w, "  … and %d more (use --json for all)\n", n-20)

				break
			}

			fmt.Fprintf(w, "  %s %s: %s %s\n", f.Kind, f.SourceID, f.Code, f.Message)
		}
	}

	return nil
}

func (a *app) rollback(ctx context.Context, yes bool) error {
	src, err := a.source(nil)
	if err != nil {
		return err
	}

	e, err := a.engineFor(ctx, src, true)
	if err != nil {
		return err
	}
	defer e.Store.Close()

	if !yes {
		if a.json {
			return errors.New("rollback deletes everything this migration created in dimmi: pass --yes once the person has agreed")
		}

		fmt.Fprint(a.out, "This deletes everything this migration created in dimmi. Type yes to go on: ")

		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(line) != "yes" {
			return errors.New("rollback cancelled")
		}
	}

	mig, err := e.Rollback(ctx)
	if err != nil {
		return err
	}

	a.report.Report(engine.Event{Event: "rolled_back", Message: "Rollback " + mig.Status + ". The next run starts over.",
		Data: map[string]string{"migration_id": mig.ID, "status": mig.Status}})

	return nil
}
