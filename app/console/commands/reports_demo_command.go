//go:build dev

/*
|--------------------------------------------------------------------------
| Reports Demo
|--------------------------------------------------------------------------
|
| Fills a development workspace with a year of history at the volume
| reports must stay fast at: hundreds of thousands of conversations
| spread over business hours, answered by the AI or the team, replied to
| over several turns, closed, some reopened, some rated, some still
| waiting. It writes rows in bulk, through the same triggers live
| changes go through, so the event log is real; then it works out every
| conversation's facts with the real builder, timing it, and counts the
| backlog of each past day. It needs the workspace's teammates, inboxes
| and customers to exist already, as inbox:demo leaves them.
|
*/

package commands

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
)

/*
reportsDemoBatch is how many conversations one transaction writes.
*/
const reportsDemoBatch = 20_000

/*
ReportsDemoCommand seeds a workspace with a year of report history.
*/
type ReportsDemoCommand struct {
	App *app.App
}

/*
Name returns reports:demo.
*/
func (c ReportsDemoCommand) Name() string {
	return "reports:demo"
}

/*
Description says what the command does.
*/
func (c ReportsDemoCommand) Description() string {
	return "Seed a development workspace with a year of conversations for the reports, and build their facts"
}

/*
Flags names the workspace and how much to seed.
*/
func (c ReportsDemoCommand) Flags(flags *console.Flags) {
	flags.String("workspace", "", "the workspace's public id, ws_...")
	flags.String("member", "", "the owner's member public id, mem_...")
	flags.Int("conversations", 300_000, "how many conversations to seed")
	flags.Int("days", 365, "how many days back they spread")
}

/*
Handle seeds the workspace, builds the facts and counts past backlogs.
*/
func (c ReportsDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	owner, err := SeriesDemoCommand(c).membership(ctx, in.String("workspace"), in.String("member"))
	if err != nil {
		return err
	}

	ws := owner.Workspace.ID
	total, days := in.Int("conversations"), in.Int("days")
	scoped := tenancy.With(ctx, ws.String())

	for done := 0; done < total; done += reportsDemoBatch {
		n := min(reportsDemoBatch, total-done)
		err := c.App.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			plan, rows := reportsDemoSQL()

			return runStatements(ctx, tx, plan+rows, []any{ws, n, days, done})
		})
		if err != nil {
			return err
		}

		out.Line("seeded %d of %d conversations", done+n, total)
	}

	if err := c.build(ctx, out); err != nil {
		return err
	}

	err = c.App.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		_, err := tx.Exec(ctx, backlogHistorySQL, ws, days)

		return err
	})
	if err != nil {
		return fmt.Errorf("count past backlogs: %w", err)
	}

	out.Line("counted the backlog of the last %d days", days)

	return nil
}

/*
runStatements runs each statement of the script in turn, each with as
many of the values as it names.
*/
func runStatements(ctx context.Context, tx *database.Tx, script string, args []any) error {
	for stmt := range strings.SplitSeq(script, ";\n") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		n := 0
		for _, m := range regexp.MustCompile(`\$(\d+)`).FindAllStringSubmatch(stmt, -1) {
			if k, _ := strconv.Atoi(m[1]); k > n {
				n = k
			}
		}

		if _, err := tx.Exec(ctx, stmt, args[:n]...); err != nil {
			return fmt.Errorf("seed step: %w\n%s", err, stmt)
		}
	}

	return nil
}

/*
build works out the facts of every queued conversation with the real
builder, timing it.
*/
func (c ReportsDemoCommand) build(ctx context.Context, out *console.Output) error {
	start := c.App.Clock.Now()
	built := 0
	far := start.Add(24 * time.Hour)

	for {
		n, err := c.App.Actions.ReportMaintenance.Drain(ctx, far)
		if err != nil {
			return err
		}

		if n == 0 {
			break
		}

		built += n
		if built%20_000 < n {
			out.Line("built facts for %d conversations", built)
		}
	}

	took := c.App.Clock.Now().Sub(start)
	out.Line("built facts for %d conversations in %s (%.0f a second)", built, took.Round(time.Second),
		float64(built)/took.Seconds())

	return nil
}
