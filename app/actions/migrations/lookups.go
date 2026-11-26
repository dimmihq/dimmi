/*
|--------------------------------------------------------------------------
| Looking Records Up
|--------------------------------------------------------------------------
|
| A client checking its work names records by their source ids and reads
| back what each became: applied with its dimmi id, waiting, failed,
| unsupported, or never received. Nothing about the records' content
| comes back, only where each stands.
|
*/

package migrations

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
LookupInput is who looks up which records of one kind of a migration.
*/
type LookupInput struct {
	Membership  authz.Membership
	MigrationID ids.UUID
	Kind        string
	SourceIDs   []string
}

/*
Found is where one record looked up stands: its status, missing when the
migration never received it, and the dimmi id it became.
*/
type Found struct {
	SourceID string
	Status   string
	RecordID *ids.UUID
}

/*
StatusMissing is the status of a record the migration never received.
*/
const StatusMissing = "missing"

/*
Lookup returns where each record named stands, in the order named.

Rules:
  - The caller needs imports:manage.
  - At most 100 source ids of one kind.

Events: none.
*/
func (s *Migrations) Lookup(ctx context.Context, in LookupInput) ([]Found, error) {
	if err := CanMigrate(in.Membership); err != nil {
		return nil, err
	}

	if _, ok := specs()[in.Kind]; !ok {
		return nil, ErrUnknownKind
	}

	if len(in.SourceIDs) > MaxBatch {
		return nil, ErrTooMany.OnField("source_ids")
	}

	ctx = tenancy.With(ctx, in.Membership.Workspace.ID.String())

	row, err := s.find(ctx, in.Membership.Workspace.ID, in.MigrationID)
	if err != nil {
		return nil, err
	}

	var rows []models.MigrationRecord

	err = s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err = models.New(tx).FindMigrationRecords(ctx, models.FindMigrationRecordsParams{
			MigrationID: row.ID, Kind: in.Kind, SourceIds: in.SourceIDs,
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("look up migration records: %w", err)
	}

	out := make([]Found, 0, len(in.SourceIDs))

	for _, sid := range in.SourceIDs {
		f := Found{SourceID: sid, Status: StatusMissing}

		i := slices.IndexFunc(rows, func(r models.MigrationRecord) bool { return r.SourceID == strings.TrimSpace(sid) })
		if i >= 0 {
			f.Status, f.RecordID = rows[i].Status, rows[i].RecordID
		}

		out = append(out, f)
	}

	return out, nil
}
