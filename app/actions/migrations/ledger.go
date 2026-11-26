/*
|--------------------------------------------------------------------------
| The Ledger
|--------------------------------------------------------------------------
|
| Every record a migration receives is kept under its kind and source id:
| what it became, the hash of what was sent, and, for one that waits or
| failed, the record itself, so it can be applied later or shown.
|
*/

package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/models"
)

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
func (r *runner) saveFailure(ctx context.Context, failed record, p *Problem) {
	kind, base, raw := failed.kind, failed.base, failed.raw

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
