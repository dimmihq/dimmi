/*
|--------------------------------------------------------------------------
| Applying Records
|--------------------------------------------------------------------------
|
| A batch is records of one kind. Each one is applied in a transaction
| of its own, so one bad record is reported and the rest still land, and
| what it became is kept in the ledger in the same transaction, so a
| record is never applied without being remembered. A record sent again
| unchanged is skipped. A record naming another the migration has not
| received yet waits, kept whole in the ledger, and is applied as soon
| as a later batch brings what it named. A record naming one that came
| but could not be applied still lands where it can: an author becomes
| a labeled system author, a missing assignee is left out.
|
*/

package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The statuses a record reaches in the ledger.
*/
const (
	statusApplied     = "applied"
	statusDeferred    = "deferred"
	statusFailed      = "failed"
	statusUnsupported = "unsupported"
)

/*
The outcomes a batch reports for a record.
*/
const (
	ResultCreated     = "created"
	ResultUpdated     = "updated"
	ResultUnchanged   = "unchanged"
	ResultDeferred    = "deferred"
	ResultFailed      = "failed"
	ResultUnsupported = "unsupported"
)

/*
deferredPass is how many waiting records one batch tries again.
*/
const deferredPass = 200

/*
ErrNotSupportedYet refuses a record of a kind this server cannot apply
yet; a field it cannot apply yet is reported as a warning instead.
*/
var ErrNotSupportedYet = fault.New(fault.Invalid, "migration_not_supported_yet",
	"dimmi cannot apply this record yet. Send it again once it can; nothing was written.")

/*
ErrReferenceUnavailable refuses a record whose required reference came
but could not be applied.
*/
var ErrReferenceUnavailable = fault.New(fault.Invalid, "migration_reference_unavailable",
	"A record this one needs was sent but could not be applied.")

/*
Result is what one record of a batch came to.
*/
type Result struct {
	Kind       string
	SourceID   string
	Outcome    string
	RecordID   *ids.UUID
	Warnings   []Notice
	Conflicts  []Conflict
	Problem    *Problem
	WaitingFor []Ref
}

/*
BatchInput is who sends which records of which kind to which migration.
*/
type BatchInput struct {
	Membership  authz.Membership
	MigrationID ids.UUID
	Kind        string
	Records     []json.RawMessage
}

/*
Batch is what a batch came to: a result per record in the order sent,
and the results of waiting records it let through.
*/
type Batch struct {
	Results  []Result
	Resolved []Result
}

/*
Apply applies a batch of records.

Rules:
  - The caller needs imports:manage; the records are written with the
    permissions of the caller's role.
  - A batch holds 1 to 100 records of one kind.
  - The migration is not being undone.

Events: none.

Errors: [exceptions.ErrForbidden], [ErrNotFound], [ErrClosed],
[ErrUnknownKind], [ErrTooMany], [ErrEmpty]. Each record's own refusal
is in its result.
*/
func (s *Migrations) Apply(ctx context.Context, in BatchInput) (Batch, error) {
	if err := CanMigrate(in.Membership); err != nil {
		return Batch{}, err
	}

	spec, ok := specs()[in.Kind]
	if !ok {
		return Batch{}, ErrUnknownKind
	}

	switch {
	case len(in.Records) == 0:
		return Batch{}, ErrEmpty
	case len(in.Records) > MaxBatch:
		return Batch{}, ErrTooMany
	}

	ctx = tenancy.With(ctx, in.Membership.Workspace.ID.String())

	row, err := s.find(ctx, in.Membership.Workspace.ID, in.MigrationID)
	if err != nil {
		return Batch{}, err
	}

	if row.Status == "rolling_back" || row.Status == "rolled_back" {
		return Batch{}, ErrClosed
	}

	r := &runner{svc: s, row: row, m: migrator(in.Membership)}

	var out Batch

	applied := false

	for _, raw := range in.Records {
		res := r.one(ctx, arrival{kind: in.Kind, spec: spec, raw: raw})
		applied = applied || res.Outcome == ResultCreated || res.Outcome == ResultUpdated
		out.Results = append(out.Results, res)
	}

	if applied {
		out.Resolved = r.resolveDeferred(ctx)
	}

	err = s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).TouchMigration(ctx, models.TouchMigrationParams{
			Now: s.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
	})

	return out, wrap("touch migration", err)
}

/*
runner applies records of one migration as one member.
*/
type runner struct {
	svc *Migrations
	row models.Migration
	m   authz.Membership
}

/*
arrival is a record as it arrives: its kind and how it is read, its
JSON, and its ledger row when it was waiting there.
*/
type arrival struct {
	kind string
	spec kindSpec
	raw  json.RawMessage
	prev *models.MigrationRecord
}

/*
one applies one record in its own transaction and returns its result.
*/
func (r *runner) one(ctx context.Context, a arrival) Result {
	kind, spec, raw, prev := a.kind, a.spec, a.raw, a.prev
	res := Result{Kind: kind}

	it, base, err := spec.decode(raw)
	res.SourceID = base.SourceID

	if err != nil {
		res.Outcome, res.Problem = ResultFailed, problemOf(err)

		if base.SourceID != "" {
			r.saveFailure(ctx, record{kind: kind, base: base, raw: raw}, res.Problem)
		}

		return res
	}

	hash := contentHash(raw)

	err = r.svc.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return r.inTx(ctx, tx, record{kind: kind, base: base, item: it, raw: raw, hash: hash, prev: prev}, &res)
	})
	if err != nil {
		log.From(ctx).ErrorContext(ctx, "migration record failed",
			log.String("kind", kind), log.Err(err))

		res.Outcome, res.Problem = ResultFailed, problemOf(err)
		r.saveFailure(ctx, record{kind: kind, base: base, raw: raw}, res.Problem)
	}

	return res
}

/*
record is a decoded record on its way in: its kind, base, item, raw
JSON, content hash, and its ledger row when it was read from there.
*/
type record struct {
	kind string
	base Base
	item item
	raw  json.RawMessage
	hash string
	prev *models.MigrationRecord
}

/*
inTx applies the record inside its transaction.
*/
func (r *runner) inTx(ctx context.Context, tx *database.Tx, rec record, res *Result) error {
	q := models.New(tx)
	if err := q.MarkImportSource(ctx); err != nil {
		return fmt.Errorf("tag imported history: %w", err)
	}

	held, err := q.ShareMigration(ctx, models.ShareMigrationParams{WorkspaceID: r.row.WorkspaceID, ID: r.row.ID})
	if err != nil {
		return fmt.Errorf("hold migration: %w", err)
	}

	if held.Status == "rolling_back" || held.Status == "rolled_back" {
		return ErrClosed
	}

	prev, err := r.ledgerRow(ctx, q, rec.kind, rec.base.SourceID)
	if err != nil {
		return err
	}

	if prev != nil && prev.Status == statusApplied && prev.ContentHash == rec.hash {
		res.Outcome, res.RecordID = ResultUnchanged, prev.RecordID
		res.Warnings, res.Conflicts = notices(prev.Warnings), conflicts(prev.Conflicts)

		return nil
	}

	refs, err := r.resolve(ctx, q, rec.item.refs())
	if err != nil {
		return err
	}

	if waiting := refs.missing(); len(waiting) > 0 {
		res.Outcome, res.WaitingFor = ResultDeferred, waiting

		return r.save(ctx, q, rec, ledgerEntry{status: statusDeferred, waiting: waiting, keep: true})
	}

	st := &step{runner: r, tx: tx, q: q, refs: refs, prev: prev, rec: rec, now: r.svc.Clock.Now()}

	var out outcome

	applyErr := r.svc.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error {
		var err error
		out, err = rec.item.apply(ctx, st)

		return err
	})

	return r.settle(ctx, q, settled{rec: rec, prev: prev, out: out, err: applyErr}, res)
}

/*
settled is a record once applied: the record, its earlier ledger row,
what applying it made, and why it failed, nil when it did not.
*/
type settled struct {
	rec  record
	prev *models.MigrationRecord
	out  outcome
	err  error
}

/*
settle keeps what the record came to and fills in its result.
*/
func (r *runner) settle(ctx context.Context, q *models.Queries, done settled, res *Result) error {
	rec, prev, out, applyErr := done.rec, done.prev, done.out, done.err

	if applyErr != nil {
		if _, ok := errors.AsType[*fault.Error](applyErr); !ok {
			return applyErr
		}

		res.Problem = problemOf(applyErr)
		res.Outcome = ResultFailed
		status := statusFailed

		if errors.Is(applyErr, ErrNotSupportedYet) {
			res.Outcome, status = ResultUnsupported, statusUnsupported
		}

		return r.save(ctx, q, rec, ledgerEntry{status: status, problem: res.Problem, label: out.label, keep: true})
	}

	res.Outcome, res.RecordID = ResultUpdated, &out.id
	if prev == nil || prev.RecordID == nil {
		res.Outcome = ResultCreated
	}

	res.Warnings, res.Conflicts = out.warnings, out.conflicts

	return r.save(ctx, q, rec, ledgerEntry{
		status: statusApplied, id: &out.id, created: out.created, label: out.label, state: out.state,
		warnings: out.warnings, conflicts: out.conflicts,
	})
}
