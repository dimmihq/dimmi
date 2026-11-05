/*
|--------------------------------------------------------------------------
| Export Audit Logs
|--------------------------------------------------------------------------
|
| Owners and admins hand their audit log to auditors and to their own
| tools as a CSV file, narrowed the way they read it on screen. A file
| covers a date range of at most a year, the last 30 days unless asked
| otherwise, and is streamed a thousand entries at a time, each batch
| read in a short transaction of its own, so a long file never holds
| the database. No cell can start a spreadsheet formula.
|
| Every export is itself recorded once the file ends, with what it was
| narrowed to and how many entries it held, and whether the reader took
| all of it.
|
*/

package audit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/csvsafe"
)

/*
The bounds of an export: the longest range it may cover, the range it
covers when none is asked for, how many entries each read takes, and
how many exports a workspace may make an hour.
*/
const (
	exportMaxDays     = 366
	exportDefaultDays = 30
	exportBatch       = 1000
	exportsPerHour    = 10
	day               = 24 * time.Hour
)

/*
errReaderGone means the reader of the file went away before its end.
*/
var errReaderGone = errors.New("export reader went away")

/*
ExportColumns are the columns of an exported audit log, in order.
*/
func ExportColumns() []string {
	return []string{
		"id", "created_at", "event", "actor_type", "actor_id", "actor_label", "via_kind", "via_id",
		"subject_type", "subject_id", "subject_label", "changes", "ip", "user_agent", "request_id",
	}
}

/*
ExportAuditLogsInput is who asks, through their membership, the filter
to narrow the log with, and the filters as they were asked for, kept
in the export's own entry.
*/
type ExportAuditLogsInput struct {
	Membership authz.Membership
	Filter     Filter
	Asked      map[string][]string
}

/*
Export is an export allowed to start: the name of its file, its filter
with the start of the range settled, the range it covers from From to
To, and what it started from. A range asked without an end runs up to
the moment the log is read.
*/
type Export struct {
	Filename string
	Filter   Filter
	From     time.Time
	To       time.Time
	in       ExportAuditLogsInput
}

/*
Encoder turns an entry into the cells of its row, in the order of
ExportColumns.
*/
type Encoder func(models.AuditLog) []string

/*
ExportAuditLogs streams the audit log of the caller's workspace as CSV.

Rules:
  - The caller needs audit_logs:read.
  - The range covers at most 366 days; none asked for is the last 30.
  - A workspace exports at most 10 times an hour.

Errors: [exceptions.ErrForbidden], [exceptions.ErrRangeTooLarge],
[exceptions.ErrTooManyExports], or a wrapped database or limiter
error.
*/
type ExportAuditLogs struct {
	List    *ListAuditLogs
	Record  *RecordAuditLog
	Limiter *ratelimit.Limiter
	Clock   clock.Clock
}

/*
Start checks the export may run and settles its range and file name,
before anything is written.
*/
func (a *ExportAuditLogs) Start(ctx context.Context, in ExportAuditLogsInput) (Export, error) {
	if !in.Membership.Can(readPermission) {
		return Export{}, exceptions.ErrForbidden
	}

	filter := in.Filter
	before := a.Clock.Now()

	if filter.CreatedBefore != nil {
		before = *filter.CreatedBefore
	}

	after := before.Add(-exportDefaultDays * day)
	if filter.CreatedAfter != nil {
		after = *filter.CreatedAfter
	}

	if before.Sub(after) > exportMaxDays*day {
		return Export{}, exceptions.ErrRangeTooLarge.With("max_days", exportMaxDays)
	}

	if err := a.throttle(ctx, in.Membership); err != nil {
		return Export{}, err
	}

	filter.CreatedAfter = &after
	name := fmt.Sprintf("dimmi-audit-%s-%s-%s.csv", in.Membership.Workspace.Slug,
		after.UTC().Format(time.DateOnly), before.UTC().Format(time.DateOnly))

	return Export{Filename: name, Filter: filter, From: after, To: before, in: in}, nil
}

/*
throttle counts the export against the workspace's hourly allowance.
*/
func (a *ExportAuditLogs) throttle(ctx context.Context, m authz.Membership) error {
	res, err := a.Limiter.Attempt(ctx, "audit_export:"+m.Workspace.ID.String(), exportsPerHour, time.Hour)
	if err != nil {
		return appauth.Unavailable(ctx, err)
	}

	if !res.Allowed {
		return exceptions.ErrTooManyExports.With("retry_after", max(int(res.RetryAfter.Seconds()), 1))
	}

	return nil
}

/*
Write streams the export to out, newest entry first, then records it.
A reader that goes away ends the stream; the entry then says the export
did not complete, and that is not an error. A log that cannot be read
is, after the entry is written.
*/
func (a *ExportAuditLogs) Write(ctx context.Context, e Export, out io.Writer, encode Encoder) error {
	rows, err := a.stream(ctx, e, out, encode)

	recordErr := a.record(context.WithoutCancel(ctx), e, rows, err == nil)
	if err != nil && !errors.Is(err, errReaderGone) {
		return err
	}

	return recordErr
}

/*
stream writes the header and every entry, a batch at a time, and
returns how many entries the reader took.
*/
func (a *ExportAuditLogs) stream(ctx context.Context, e Export, out io.Writer, encode Encoder) (int, error) {
	w := csvsafe.NewWriter(out)
	if err := send(w, [][]string{ExportColumns()}); err != nil {
		return 0, err
	}

	rows := 0
	in := ListAuditLogsInput{Membership: e.in.Membership, Filter: e.Filter, Limit: exportBatch}

	for {
		page, err := a.List.Handle(ctx, in)
		if err != nil {
			return rows, readFailure(ctx, err)
		}

		batch := make([][]string, 0, len(page.Entries))
		for _, entry := range page.Entries {
			batch = append(batch, encode(entry))
		}

		if err := send(w, batch); err != nil {
			return rows, err
		}

		rows += len(batch)
		if !page.More {
			return rows, nil
		}

		last := page.Entries[len(page.Entries)-1]
		in.From = &Position{At: last.CreatedAt, ID: last.ID}
	}
}

/*
send writes the records and hands them to the reader.
*/
func send(w *csvsafe.Writer, records [][]string) error {
	for _, record := range records {
		if err := w.Write(record); err != nil {
			return fmt.Errorf("%w: %w", errReaderGone, err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("%w: %w", errReaderGone, err)
	}

	return nil
}

/*
readFailure tells a reader who went away, which cancels the read, from
a log that could not be read.
*/
func readFailure(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %w", errReaderGone, ctx.Err())
	}

	return err
}

/*
record writes the export's own entry: what it was narrowed to, how
many entries it held and whether the reader took them all.
*/
func (a *ExportAuditLogs) record(ctx context.Context, e Export, rows int, completed bool) error {
	filters := map[string]any{}
	for name, values := range e.in.Asked {
		filters[name] = values
	}

	filters["created_after"] = e.From.UTC().Format(time.RFC3339)
	filters["created_before"] = e.To.UTC().Format(time.RFC3339)

	m := e.in.Membership
	_, err := a.Record.Handle(ctx, Entry{
		WorkspaceID: m.Workspace.ID, Event: EventAuditLogsExported, ActorID: m.Member.ID,
		Subject: Subject{Type: "workspace", ID: m.Workspace.ID, Label: m.Workspace.Name},
		Changes: map[string]any{"filters": filters, "rows": rows, "completed": completed},
	})

	return err
}
