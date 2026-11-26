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
| a labelled system author, a missing assignee is left out.
|
*/

package migrations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

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
Ref names another record of the migration by its kind and source id.
*/
type Ref struct {
	Kind     string `json:"kind"`
	SourceID string `json:"source_id"`
}

/*
Notice is a warning about a field: what was left out and why.
*/
type Notice struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

/*
Conflict is a field a teammate changed here after the migration wrote
it, which a later sync leaves as the teammate set it: the value here,
and the one the other tool sent.
*/
type Conflict struct {
	Field  string `json:"field"`
	Dimmi  any    `json:"dimmi"`
	Source any    `json:"source"`
}

/*
Problem is why a record could not be applied.
*/
type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

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

	spec, ok := kinds()[in.Kind]
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
		res := r.one(ctx, in.Kind, spec, raw, nil)
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
one applies one record in its own transaction and returns its result.
A record read from the ledger, waiting, passes its row as prev.
*/
func (r *runner) one(ctx context.Context, kind string, spec kindSpec, raw json.RawMessage, prev *models.MigrationRecord,
) Result {
	res := Result{Kind: kind}

	it, base, err := spec.decode(raw)
	res.SourceID = base.SourceID

	if err != nil {
		res.Outcome, res.Problem = ResultFailed, problemOf(err)

		if base.SourceID != "" {
			r.saveFailure(ctx, kind, base, raw, res.Problem)
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
		r.saveFailure(ctx, kind, base, raw, res.Problem)
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

	if _, err := q.LockMigration(ctx, models.LockMigrationParams{
		WorkspaceID: r.row.WorkspaceID, ID: r.row.ID,
	}); err != nil {
		return fmt.Errorf("hold migration: %w", err)
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

	return r.settle(ctx, q, rec, prev, out, applyErr, res)
}

/*
settle keeps what the record came to and fills in its result.
*/
func (r *runner) settle(ctx context.Context, q *models.Queries, rec record, prev *models.MigrationRecord,
	out outcome, applyErr error, res *Result,
) error {
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

/*
ledgerRow returns the record's row in the ledger, nil when there is none.
*/
func (r *runner) ledgerRow(ctx context.Context, q *models.Queries, kind, sourceID string,
) (*models.MigrationRecord, error) {
	row, err := q.FindMigrationRecord(ctx, models.FindMigrationRecordParams{
		MigrationID: r.row.ID, Kind: kind, SourceID: sourceID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find migration record: %w", err)
	}

	return &row, nil
}

/*
ledgerEntry is what a record came to, for the ledger. Keep holds the
record's JSON, for a record that waits or failed.
*/
type ledgerEntry struct {
	status    string
	id        *ids.UUID
	created   bool
	label     string
	state     json.RawMessage
	waiting   []Ref
	problem   *Problem
	warnings  []Notice
	conflicts []Conflict
	keep      bool
}

/*
save writes the record's row in the ledger.
*/
func (r *runner) save(ctx context.Context, q *models.Queries, rec record, e ledgerEntry) error {
	params := models.SaveMigrationRecordParams{
		WorkspaceID: r.row.WorkspaceID, MigrationID: r.row.ID, Kind: rec.kind, SourceID: rec.base.SourceID,
		Status: e.status, RecordID: e.id, Created: e.created, Label: orNil(cut(e.label, 200)),
		ContentHash: rec.hash, SourceUpdatedAt: rec.base.SourceUpdatedAt, State: e.state,
		WaitingFor: mustJSON(nonNilRefs(e.waiting)), Warnings: mustJSON(nonNil(e.warnings)),
		Conflicts: mustJSON(nonNil(e.conflicts)), Now: r.svc.Clock.Now(),
	}

	if e.state == nil {
		params.State = json.RawMessage(`{}`)
		if rec.prev != nil {
			params.State = rec.prev.State
		}
	}

	if e.keep {
		params.Payload = rec.raw
	}

	if e.problem != nil {
		params.Error = mustJSON(e.problem)
	}

	if _, err := q.SaveMigrationRecord(ctx, params); err != nil {
		return fmt.Errorf("save migration record: %w", err)
	}

	return nil
}

/*
saveFailure keeps a record that could not be read or applied, in a
transaction of its own.
*/
func (r *runner) saveFailure(ctx context.Context, kind string, base Base, raw json.RawMessage, p *Problem) {
	err := r.svc.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		prev, err := r.ledgerRow(ctx, q, kind, base.SourceID)
		if err != nil {
			return err
		}

		if prev != nil && prev.Status == statusApplied {
			return nil
		}

		rec := record{kind: kind, base: base, raw: compactJSON(raw), hash: contentHash(raw), prev: prev}

		return r.save(ctx, q, rec, ledgerEntry{status: statusFailed, problem: p, keep: true})
	})
	if err != nil {
		log.From(ctx).ErrorContext(ctx, "keep failed migration record", log.Err(err))
	}
}

/*
resolveDeferred applies the waiting records whose references have all
arrived, until a pass lets none through.
*/
func (r *runner) resolveDeferred(ctx context.Context) []Result {
	var out []Result

	tried := map[ids.UUID]bool{}

	for range 5 {
		rows, err := r.deferred(ctx)
		if err != nil {
			log.From(ctx).ErrorContext(ctx, "list waiting migration records", log.Err(err))

			return out
		}

		progressed := false

		for _, row := range rows {
			if tried[row.ID] || row.Payload == nil {
				continue
			}

			spec, ok := kinds()[row.Kind]
			if !ok || !r.ready(ctx, row) {
				continue
			}

			tried[row.ID] = true
			res := r.one(ctx, row.Kind, spec, row.Payload, &row)
			out = append(out, res)
			progressed = progressed || res.Outcome != ResultDeferred
		}

		if !progressed {
			return out
		}
	}

	return out
}

/*
deferred lists the migration's waiting records.
*/
func (r *runner) deferred(ctx context.Context) ([]models.MigrationRecord, error) {
	var rows []models.MigrationRecord

	err := r.svc.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		rows, err = models.New(tx).ListDeferredMigrationRecords(ctx, models.ListDeferredMigrationRecordsParams{
			MigrationID: r.row.ID, RowLimit: deferredPass,
		})

		return err
	})

	return rows, err
}

/*
ready reports whether every reference the waiting record named has
arrived.
*/
func (r *runner) ready(ctx context.Context, row models.MigrationRecord) bool {
	var waiting []Ref
	if err := json.Unmarshal(row.WaitingFor, &waiting); err != nil {
		return true
	}

	ok := true

	err := r.svc.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		found, err := r.resolve(ctx, models.New(tx), waiting)
		ok = err == nil && len(found.missing()) == 0

		return err
	})

	return err == nil && ok
}

/*
problemOf returns the refusal as a record's problem, a generic one for
an error that is not a refusal.
*/
func problemOf(err error) *Problem {
	if f, ok := errors.AsType[*fault.Error](err); ok {
		return &Problem{Code: f.Code(), Message: f.Message(), Field: f.Field()}
	}

	return &Problem{Code: "migration_record_error", Message: "dimmi could not apply this record. Send it again."}
}

/*
contentHash is the hash of the record's JSON with its keys sorted, so
the same record sent with its keys in another order is still the same.
*/
func contentHash(raw json.RawMessage) string {
	sum := sha256.Sum256(canonical(raw))

	return hex.EncodeToString(sum[:])
}

/*
canonical returns the JSON with its object keys sorted.
*/
func canonical(raw json.RawMessage) []byte {
	var v any

	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()

	if err := d.Decode(&v); err != nil {
		return raw
	}

	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}

	return out
}

/*
compactJSON returns the JSON compacted, or a JSON string of it when it
is not JSON.
*/
func compactJSON(raw json.RawMessage) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return mustJSON(string(raw))
	}

	return buf.Bytes()
}

/*
mustJSON encodes the value, which is always encodable.
*/
func mustJSON(v any) json.RawMessage {
	out, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}

	return out
}

/*
nonNil returns the list, empty instead of nil.
*/
func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}

	return list
}

/*
nonNilRefs returns the references, empty instead of nil.
*/
func nonNilRefs(list []Ref) []Ref {
	return nonNil(list)
}

/*
notices reads the warnings a ledger row keeps.
*/
func notices(raw json.RawMessage) []Notice {
	var out []Notice
	_ = json.Unmarshal(raw, &out)

	return out
}

/*
conflicts reads the conflicts a ledger row keeps.
*/
func conflicts(raw json.RawMessage) []Conflict {
	var out []Conflict
	_ = json.Unmarshal(raw, &out)

	return out
}

/*
step is one record being applied: its transaction, the references it
named, its earlier ledger row and the server's time.
*/
type step struct {
	runner *runner
	tx     *database.Tx
	q      *models.Queries
	refs   resolved
	prev   *models.MigrationRecord
	rec    record
	now    time.Time
	ledger *ticketLedger
}

/*
ws returns the workspace the step writes in.
*/
func (s *step) ws() ids.UUID {
	return s.runner.row.WorkspaceID
}

/*
outcome is what applying a record made: the row it became, whether the
migration created it, a label for records that name it, what a later
sync needs to remember, and the warnings and conflicts.
*/
type outcome struct {
	id        ids.UUID
	created   bool
	label     string
	state     json.RawMessage
	warnings  []Notice
	conflicts []Conflict
}

/*
warn adds a warning to the outcome.
*/
func (o *outcome) warn(field, code, message string) {
	o.warnings = append(o.warnings, Notice{Field: field, Code: code, Message: message})
}

/*
unsupported warns that a field is not applied yet.
*/
func (o *outcome) unsupported(field string) {
	o.warn(field, "not_supported_yet", "dimmi does not apply this field yet; it was left out.")
}

/*
item is a decoded record of any kind: the records it names, and how it
is applied.
*/
type item interface {
	refs() []Ref
	apply(ctx context.Context, s *step) (outcome, error)
}

/*
kindSpec reads a record of one kind.
*/
type kindSpec struct {
	decode func(raw json.RawMessage) (item, Base, error)
}

/*
validator is a record that checks itself once decoded.
*/
type validator interface {
	validate() error
}

/*
decoder returns the reader of records of the type: decoded, its source
id checked, then checked itself.
*/
func decoder[T any, P interface {
	*T
	item
	validator
	base() Base
}]() kindSpec {
	return kindSpec{decode: func(raw json.RawMessage) (item, Base, error) {
		var v T

		p := P(&v)

		d := json.NewDecoder(bytes.NewReader(raw))
		if err := d.Decode(p); err != nil {
			var peek Base
			_ = json.Unmarshal(raw, &peek)

			return nil, peek, invalid("", "The record could not be read: "+jsonProblem(err))
		}

		b := p.base()
		b.SourceID = strings.TrimSpace(b.SourceID)

		if b.SourceID == "" || len(b.SourceID) > 255 {
			return nil, b, invalid("source_id", "Every record has a source_id of 1 to 255 characters.")
		}

		if err := p.validate(); err != nil {
			return nil, b, err
		}

		return p, b, nil
	}}
}

/*
jsonProblem says what was wrong with the JSON.
*/
func jsonProblem(err error) string {
	if te, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		return fmt.Sprintf("%s has the wrong type.", te.Field)
	}

	return "it is not valid JSON."
}

/*
invalid returns a record's refusal of the field.
*/
func invalid(field, message string) error {
	f := fault.New(fault.Invalid, "migration_record_invalid", message)
	if field != "" {
		f = f.OnField(field)
	}

	return f
}

/*
required refuses an empty text field.
*/
func required(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalid(field, field+" is required.")
	}

	return nil
}

/*
oneOf refuses a value outside the choices, empty allowed.
*/
func oneOf(field, value string, choices ...string) error {
	if value == "" || slices.Contains(choices, value) {
		return nil
	}

	return invalid(field, field+" is one of "+strings.Join(choices, ", ")+".")
}
