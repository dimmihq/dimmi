/*
|--------------------------------------------------------------------------
| Waiting Records
|--------------------------------------------------------------------------
|
| A record naming one that has not arrived waits in the ledger. After a
| batch applies anything, the waiting records whose references all
| arrived are applied, a few passes at most, so a chain of them settles.
|
*/

package migrations

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/models"
)

/*
resolveDeferred applies the waiting records whose references have all
arrived, until a pass lets none through.
*/
func (r *runner) resolveDeferred(ctx context.Context) []Result {
	var out []Result

	tried := map[ids.UUID]bool{}

	for range 5 {
		results, progressed := r.pass(ctx, tried)
		out = append(out, results...)

		if !progressed {
			break
		}
	}

	return out
}

/*
pass applies once each waiting record not tried yet whose references
have all arrived, and reports whether any of them went through.
*/
func (r *runner) pass(ctx context.Context, tried map[ids.UUID]bool) ([]Result, bool) {
	rows, err := r.deferred(ctx)
	if err != nil {
		log.From(ctx).ErrorContext(ctx, "list waiting migration records", log.Err(err))

		return nil, false
	}

	var out []Result

	progressed := false

	for _, row := range rows {
		spec, ok := specs()[row.Kind]
		if tried[row.ID] || row.Payload == nil || !ok || !r.ready(ctx, row) {
			continue
		}

		tried[row.ID] = true
		res := r.one(ctx, arrival{kind: row.Kind, spec: spec, raw: row.Payload, prev: &row})
		out = append(out, res)
		progressed = progressed || res.Outcome != ResultDeferred
	}

	return out, progressed
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
