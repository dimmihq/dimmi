package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
	"github.com/dimmihq/dimmi/cli/sources/file"
)

/*
printer writes lines for people through a buffer. The buffer keeps the
first write error, and flush returns it.
*/
type printer struct {
	w *bufio.Writer
}

func newPrinter(w io.Writer) printer {
	return printer{w: bufio.NewWriter(w)}
}

/*
f writes formatted text.
*/
func (p printer) f(layout string, args ...any) {
	_, _ = fmt.Fprintf(p.w, layout, args...)
}

/*
ln writes a line.
*/
func (p printer) ln(args ...any) {
	_, _ = fmt.Fprintln(p.w, args...)
}

/*
flush writes out what's buffered and returns the first write error.
*/
func (p printer) flush() error {
	return p.w.Flush()
}

/*
plan prints what will come across and what won't.
*/
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

	p := newPrinter(a.out)
	acct := inv.Account
	p.f("Intercom workspace %s (%s, %s region)\n\n", acct.Name, acct.ID, strings.ToUpper(acct.Region))
	p.ln("Comes across:")

	for _, k := range inv.Kinds {
		p.f("  %-24s %10s", engine.Label(k.Kind), planCount(k))

		if len(k.Notes) > 0 {
			p.f("   %s", strings.Join(k.Notes, "; "))
		}

		p.ln()
	}

	planLeftovers(p, inv)

	return p.flush()
}

/*
planCount is how the plan shows a kind's count.
*/
func planCount(k engine.KindCount) string {
	count := fmt.Sprint(k.Count)
	if k.Estimate {
		count = "about " + count
	}

	if len(k.Notes) > 0 && k.Count == 0 && !k.Estimate {
		count = "-"
	}

	return count
}

/*
planLeftovers prints the missing permissions, what stays behind and the
warnings.
*/
func planLeftovers(p printer, inv engine.Inventory) {
	if len(inv.Missing) > 0 {
		p.ln("\nThe Intercom token lacks these permissions; add them to the app and run plan again:")

		for _, m := range inv.Missing {
			p.ln("  - " + m)
		}
	}

	p.ln("\nStays behind:")

	for _, n := range inv.NotImported {
		p.ln("  - " + n)
	}

	for _, warn := range inv.Warnings {
		p.ln("\n! " + warn)
	}

	p.ln("\nNothing imported sends an email or notification, or starts a workflow.")
	p.ln("Leave out a kind with `dimmi migrate run --skip contact_note,ticket`.")
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

/*
runOptions is what `dimmi migrate run` and `sync` ask for: the kinds to
skip or keep, comma-separated, the workflows to bring, whether this is a
sync, and how often to repeat it.
*/
type runOptions struct {
	skip      string
	only      string
	workflows []string
	sync      bool
	watch     time.Duration
}

func (a *app) run(ctx context.Context, opts runOptions) error {
	skipKinds, err := kinds(opts.skip)
	if err != nil {
		return err
	}

	onlyKinds, err := kinds(opts.only)
	if err != nil {
		return err
	}

	src, err := a.source(opts.workflows)
	if err != nil {
		return err
	}

	e, err := a.engineFor(ctx, src)
	if err != nil {
		return err
	}
	defer closeStore(e)

	for {
		err := e.Run(ctx, engine.RunOptions{Skip: skipKinds, Only: onlyKinds, Sync: opts.sync})
		if err != nil || opts.watch <= 0 {
			return err
		}

		msg := fmt.Sprintf("next sync in %s; Ctrl-C stops", opts.watch)
		a.report.Report(engine.Event{Event: "waiting", Message: msg})

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(opts.watch):
		}
	}
}

func (a *app) importFile(ctx context.Context, path, tool, account string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	src := &file.Source{Path: path, Tool: tool, AccountID: account}

	e, err := a.engineFor(ctx, src)
	if err != nil {
		return err
	}
	defer closeStore(e)

	return e.Run(ctx, engine.RunOptions{})
}

func (a *app) verify(ctx context.Context, sample int) error {
	src, err := a.source(nil)
	if err != nil {
		return err
	}

	e, err := a.engineFor(ctx, src)
	if err != nil {
		return err
	}
	defer closeStore(e)

	v, err := e.Verify(ctx, sample)
	if err != nil {
		return err
	}

	if !a.json {
		p := newPrinter(a.out)
		p.f("\n%-24s %8s %8s %8s %8s\n", "", "Intercom", "dimmi", "sampled", "missing")

		for _, k := range v.Kinds {
			mark := "✓"
			if !k.OK {
				mark = "✗"
			}

			p.f("%s %-22s %8d %8d %8d %8d\n", mark, engine.Label(k.Kind), k.Source, k.Dimmi, k.Sampled, len(k.Missing))
		}

		_ = p.flush()
	}

	a.report.Report(engine.Event{Event: "verification", Message: verdict(v), Data: v})

	if !v.OK {
		return errors.New("verify found differences")
	}

	return nil
}

/*
verdict sums up a verification in a sentence.
*/
func verdict(v engine.Verification) string {
	if v.OK {
		return "Everything checked is in dimmi."
	}

	return "Some records are missing in dimmi: run `dimmi migrate sync` and verify again."
}

func (a *app) status(ctx context.Context) error {
	src, err := a.source(nil)
	if err != nil {
		return err
	}

	e, err := a.engineFor(ctx, src)
	if err != nil {
		return err
	}
	defer closeStore(e)

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

	p := newPrinter(a.out)
	p.f("Migration %s from %s %s\n\n", st.State.MigrationID, st.State.Source, st.State.Account.Name)

	for _, k := range src.Kinds() {
		p.ln(statusLine(st, k))
	}

	if !st.State.SyncedAt.IsZero() {
		p.f("\nLast sync %s\n", st.State.SyncedAt.Local().Format(time.RFC1123))
	}

	statusFailures(p, st.State.Failures)

	return p.flush()
}

/*
statusLine is the status of one kind: where it stands on this machine
and, when dimmi answered, its counts there.
*/
func statusLine(st engine.Status, k format.Kind) string {
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

	return line
}

/*
statusFailures lists the first 20 refused records.
*/
func statusFailures(p printer, failures []engine.Failure) {
	n := len(failures)
	if n == 0 {
		return
	}

	p.f("\n%d records were refused. Ids and reasons:\n", n)

	for i, f := range failures {
		if i == 20 {
			p.f("  … and %d more (use --json for all)\n", n-20)

			break
		}

		p.f("  %s %s: %s %s\n", f.Kind, f.SourceID, f.Code, f.Message)
	}
}

/*
rollbackOptions says whether the person already agreed to the rollback.
*/
type rollbackOptions struct {
	yes bool
}

func (a *app) rollback(ctx context.Context, opts rollbackOptions) error {
	src, err := a.source(nil)
	if err != nil {
		return err
	}

	e, err := a.engineFor(ctx, src)
	if err != nil {
		return err
	}
	defer closeStore(e)

	if !opts.yes {
		if err := a.confirmRollback(); err != nil {
			return err
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
confirmRollback asks the person to type yes. With --json there's no one
to ask, so it refuses.
*/
func (a *app) confirmRollback() error {
	if a.json {
		return errors.New("rollback deletes everything this migration created in dimmi: " +
			"pass --yes once the person has agreed")
	}

	_, _ = fmt.Fprint(a.out, "This deletes everything this migration created in dimmi. Type yes to go on: ")

	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSpace(line) != "yes" {
		return errors.New("rollback canceled")
	}

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

	_, err = fmt.Fprintln(a.out, string(raw))

	return err
}
