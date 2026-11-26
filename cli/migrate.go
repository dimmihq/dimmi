package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
	"github.com/dimmihq/dimmi/cli/secrets"
	"github.com/dimmihq/dimmi/cli/sources/file"
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
		dimmiURL := fs.String("dimmi-url", "", "the dimmi to move into, https://trydimmi.com unless self-hosted")
		source := fs.String("source", "intercom", "the tool to move from")
		noBrowser := fs.Bool("no-browser", false, "print the address instead of opening the browser")

		return command{fs, func() error { return a.connect(ctx, *dimmiURL, *source, *noBrowser) }}, true
	case "plan":
		workflows := multi(fs, "workflow", "an Intercom workflow URL to bring, repeatable")

		return command{fs, func() error { return a.plan(ctx, *workflows) }}, true
	case "run":
		skip := fs.String("skip", "", "kinds to leave out, comma-separated, such as contact_note,ticket")
		only := fs.String("only", "", "import only these kinds, comma-separated")
		workflows := multi(fs, "workflow", "an Intercom workflow URL to bring, repeatable")

		return command{fs, func() error { return a.run(ctx, *skip, *only, *workflows, false, 0) }}, true
	case "sync":
		watch := fs.Duration("watch", 0, "repeat the sync this often, such as 15m, until stopped")

		return command{fs, func() error { return a.run(ctx, "", "", nil, true, *watch) }}, true
	case "verify":
		sample := fs.Int("sample", 50, "records to look up in dimmi per kind")

		return command{fs, func() error { return a.verify(ctx, *sample) }}, true
	case "status":
		return command{fs, func() error { return a.status(ctx) }}, true
	case "rollback":
		yes := fs.Bool("yes", false, "don't ask for confirmation")

		return command{fs, func() error { return a.rollback(ctx, *yes) }}, true
	case "import":
		tool := fs.String("source-name", "", "the tool the file came from, such as zendesk")
		account := fs.String("account", "", "the account in that tool, so reruns resume")

		return command{fs, func() error {
			if fs.NArg() != 1 {
				return errors.New("name the file: dimmi migrate import records.jsonl")
			}

			return a.importFile(ctx, fs.Arg(0), *tool, *account)
		}}, true
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

	return a.page(ctx, &setup, noBrowser)
}

/*
disconnect forgets the source token. `dimmi logout` forgets the dimmi
sign-in.
*/
func (a *app) disconnect() error {
	if err := a.keys.Delete(a.sourceKey()); err != nil {
		return err
	}

	a.say("Forgot the saved Intercom token.", nil)

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
		return nil, errors.New("Intercom isn't connected: run `dimmi migrate connect` first")
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

		fmt.Fprintf(w, "  %-24s %10s", engine.Label(k.Kind), count)

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
	fmt.Fprintln(w, "Leave out a kind with `dimmi migrate run --skip contact_note,ticket`.")

	return nil
}

func kinds(list string) ([]format.Kind, error) {
	var out []format.Kind

	for k := range strings.SplitSeq(list, ",") {
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

			fmt.Fprintf(w, "%s %-22s %8d %8d %8d %8d\n", mark, engine.Label(k.Kind), k.Source, k.Dimmi, k.Sampled, len(k.Missing))
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

	return "Some records are missing in dimmi: run `dimmi migrate sync` and verify again."
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
		a.say("No migration has run from this machine yet. Start with `dimmi migrate plan`.", nil)

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

		line := fmt.Sprintf("  %-24s %-12s", engine.Label(k), state)
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
			return errors.New("rollback canceled")
		}
	}

	mig, err := e.Rollback(ctx)
	if err != nil {
		return err
	}

	a.report.Report(engine.Event{
		Event: "rolled_back", Message: "Rollback " + mig.Status + ". The next run starts over.",
		Data: map[string]string{"migration_id": mig.ID, "status": mig.Status},
	})

	return nil
}

/*
inventory reports how the workspace's data is shaped: part types,
article blocks, how attachments are served. It prints kinds, counts and
field names only.
*/
func (a *app) inventory(ctx context.Context, out, visitor string) error {
	src, err := a.source(nil)
	if err != nil {
		return err
	}

	rep, err := src.Inventory(ctx, out, visitor)
	if err != nil {
		return err
	}

	raw, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(a.out, string(raw))

	return nil
}
