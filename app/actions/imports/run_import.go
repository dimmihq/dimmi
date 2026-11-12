/*
|--------------------------------------------------------------------------
| Running Imports
|--------------------------------------------------------------------------
|
| A started import runs in the background as the teammate who started
| it, with their permissions as they stand when it runs. Rows go in
| batches; after each, the counts and the failed rows are saved, so the
| screen shows progress and a retried run picks up after the last saved
| batch. The failed rows make a CSV report: each row as it was in the
| file, with its line and why it failed.
|
*/

package imports

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
batch is how many rows run between saves of the progress, and
keptErrors how many failed rows an import keeps for its report.
*/
const (
	batch      = 100
	keptErrors = 10_000
)

/*
RunArgs names the import a job runs.
*/
type RunArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	ImportID    ids.UUID `json:"import_id"`
}

/*
Kind names the job.
*/
func (RunArgs) Kind() string { return "run_import" }

/*
Options run one import once at a time, retried a few times.
*/
func (RunArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: 3,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRetryable, queue.StateRunning, queue.StateScheduled,
		}},
	}
}

/*
Run runs a queued import to the end. An import gone or already over is
left alone; one whose starter may no longer import fails.
*/
func (a *Imports) Run(ctx context.Context, args RunArgs) error {
	ctx = tenancy.With(ctx, args.WorkspaceID.String())
	q := models.New(a.DB)

	row, err := q.StartImport(ctx, models.StartImportParams{
		Now: a.Clock.Now(), WorkspaceID: args.WorkspaceID, ID: args.ImportID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("start import: %w", err)
	}

	if err := a.applyRows(ctx, q, row); err != nil {
		return errors.Join(err, a.fail(ctx, q, row, err))
	}

	_, err = q.FinishImport(ctx, models.FinishImportParams{
		Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
	if err != nil {
		return fmt.Errorf("finish import: %w", err)
	}

	return nil
}

/*
applyRows applies the rows not yet saved, a batch at a time.
*/
func (a *Imports) applyRows(ctx context.Context, q *models.Queries, row models.Import) error {
	m, err := starter(ctx, q, row)
	if err != nil {
		return err
	}

	plan := Plan{Mode: row.Mode}
	if err := json.Unmarshal(row.Mapping, &plan.Mapping); err != nil {
		return fmt.Errorf("read mapping: %w", err)
	}

	s, b, err := prepare(ctx, q, row, plan)
	if err != nil {
		return err
	}

	ap := applier{
		db: a.DB, upsert: a.Upsert, m: m, kind: row.Kind, mode: row.Mode, now: a.Clock.Now, importID: row.ID,
	}
	failed := int(row.FailedCount)

	for start := int(row.ProcessedRows); start < len(s.rows); start += batch {
		p := progress{failedBefore: failed, processed: min(start+batch, len(s.rows))}

		for _, l := range s.rows[start:min(start+batch, len(s.rows))] {
			p.add(ctx, ap, b, l)
		}

		failed += p.failed
		if err := a.save(ctx, q, row, p); err != nil {
			return err
		}
	}

	return nil
}

/*
progress is what one batch came to: its counts and failed rows, how
many rows of the file have run once it is saved, and how many rows
failed in the batches before it.
*/
type progress struct {
	created, updated, skipped, failed int
	errors                            []RowError
	processed, failedBefore           int
}

/*
add applies one row and counts it.
*/
func (p *progress) add(ctx context.Context, ap applier, b builder, l line) {
	r, problem := b.build(l)

	outcome := OutcomeFailed
	if problem == "" {
		var err error
		if outcome, err = ap.apply(ctx, r); err != nil {
			outcome, problem = OutcomeFailed, reason(err)
		}
	}

	switch outcome {
	case OutcomeCreated:
		p.created++
	case OutcomeUpdated:
		p.updated++
	case OutcomeSkipped:
		p.skipped++
	default:
		p.failed++
		if p.failedBefore+len(p.errors) < keptErrors {
			p.errors = append(p.errors, RowError{Row: l.number, Message: problem})
		}
	}
}

/*
save adds a batch's counts and failed rows to the import.
*/
func (a *Imports) save(ctx context.Context, q *models.Queries, row models.Import, p progress) error {
	failures, err := json.Marshal(append([]RowError{}, p.errors...))
	if err != nil {
		return fmt.Errorf("encode row errors: %w", err)
	}

	err = q.RecordImportProgress(ctx, models.RecordImportProgressParams{
		ProcessedRows: int32Of(p.processed), Created: int32Of(p.created), Updated: int32Of(p.updated),
		Skipped: int32Of(p.skipped), Failed: int32Of(p.failed), RowErrors: failures, Now: a.Clock.Now(),
		WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
	if err != nil {
		return fmt.Errorf("record import progress: %w", err)
	}

	return nil
}

/*
int32Of bounds a count to what a query takes.
*/
func int32Of(n int) int32 {
	if n < 0 {
		return 0
	}

	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n)
}

/*
fail marks the import failed with a reason a person can read.
*/
func (a *Imports) fail(ctx context.Context, q *models.Queries, row models.Import, cause error) error {
	message := reason(cause)
	if message == "could not be saved" {
		message = "The import stopped unexpectedly."
	}

	return q.FailImport(ctx, models.FailImportParams{
		Error: &message, Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
}

/*
starter returns the membership of the teammate who started the import,
as it stands now, refusing one who left or may no longer import.
*/
func starter(ctx context.Context, q *models.Queries, row models.Import) (authz.Membership, error) {
	if row.CreatedByMemberID == nil {
		return authz.Membership{}, exceptions.ErrForbidden
	}

	m, err := memberOf(ctx, q, row.WorkspaceID, *row.CreatedByMemberID)
	if err != nil {
		return m, err
	}

	return m, allowed(m)
}

/*
memberOf returns the membership of the teammate as it stands now,
refusing one who left.
*/
func memberOf(ctx context.Context, q *models.Queries, ws, id ids.UUID) (authz.Membership, error) {
	workspace, err := q.FindWorkspace(ctx, ws)
	if err != nil {
		return authz.Membership{}, fmt.Errorf("find workspace: %w", err)
	}

	member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return authz.Membership{}, fmt.Errorf("find member: %w", err)
	}

	custom, err := q.CustomPermissions(ctx, member)
	if err != nil {
		return authz.Membership{}, fmt.Errorf("load custom role: %w", err)
	}

	m := authz.NewMembership(workspace, member, authz.HeldPermissions(enums.Role(member.Role), custom))
	if member.Status != string(enums.MemberStatusActive) {
		return m, exceptions.ErrForbidden
	}

	return m, nil
}

/*
Report returns the import's failed rows as a CSV: the line, why, and
the row's cells under the file's own header.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[ErrFileExpired], or a wrapped error.
*/
func (a *Imports) Report(ctx context.Context, in Input) ([]byte, string, error) {
	if err := allowed(in.Membership); err != nil {
		return nil, "", err
	}

	row, err := find(ctx, models.New(a.DB), in)
	if err != nil {
		return nil, "", err
	}

	if row.FileExpiredAt != nil {
		return nil, "", ErrFileExpired
	}

	s, err := read(row.File)
	if err != nil {
		return nil, "", err
	}

	var failures []RowError
	if err := json.Unmarshal(row.RowErrors, &failures); err != nil {
		return nil, "", fmt.Errorf("read row errors: %w", err)
	}

	byLine := make(map[int]line, len(s.rows))
	for _, l := range s.rows {
		byLine[l.number] = l
	}

	var buf bytes.Buffer

	w := csv.NewWriter(&buf)
	_ = w.Write(append([]string{"row", "error"}, s.columns...))

	for _, f := range failures {
		cells := make([]string, len(s.columns))
		for i := range cells {
			cells[i] = byLine[f.Row].cell(i)
		}

		_ = w.Write(append([]string{strconv.Itoa(f.Row), f.Message}, cells...))
	}

	w.Flush()

	return buf.Bytes(), "import-errors-" + models.PublicID(models.PrefixImport, row.ID) + ".csv", w.Error()
}
