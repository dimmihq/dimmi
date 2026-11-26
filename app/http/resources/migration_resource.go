/*
|--------------------------------------------------------------------------
| Migration Resources
|--------------------------------------------------------------------------
|
| A migration as its client follows it: which account of which tool it
| belongs to, where it stands, how many records of each kind were
| applied, wait for a reference, failed or are not supported yet, and
| the latest problems with why. A batch answers with a result per record
| in the order sent, and the waiting records it let through.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/migrations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Migration is one migration with its progress.
*/
type Migration struct {
	ID            string                   `json:"id"`
	Object        string                   `json:"object"`
	Source        string                   `json:"source"`
	SourceAccount string                   `json:"source_account"`
	Name          *string                  `json:"name"`
	FormatVersion int32                    `json:"format_version"`
	Status        string                   `json:"status"           enum:"active,rolling_back,rolled_back,rollback_failed"`
	Counts        []MigrationKindCount     `json:"counts"`
	Problems      []MigrationRecordProblem `json:"problems"`
	Rollback      *MigrationRollback       `json:"rollback"`
	LastRecordAt  *time.Time               `json:"last_record_at"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

/*
MigrationSummary is a migration in a list, without its progress.
*/
type MigrationSummary struct {
	ID            string     `json:"id"`
	Object        string     `json:"object"`
	Source        string     `json:"source"`
	SourceAccount string     `json:"source_account"`
	Name          *string    `json:"name"`
	Status        string     `json:"status"         enum:"active,rolling_back,rolled_back,rollback_failed"`
	LastRecordAt  *time.Time `json:"last_record_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

/*
MigrationKindCount is how many records of one kind reached each status.
*/
type MigrationKindCount struct {
	Kind        string `json:"kind"`
	Applied     int    `json:"applied"`
	Created     int    `json:"created"`
	Deferred    int    `json:"deferred"`
	Failed      int    `json:"failed"`
	Unsupported int    `json:"unsupported"`
	Conflicted  int    `json:"conflicted"`
}

/*
MigrationRecordProblem is a record that failed, is not supported yet,
or carries conflicts.
*/
type MigrationRecordProblem struct {
	Kind      string                `json:"kind"`
	SourceID  string                `json:"source_id"`
	Status    string                `json:"status"    enum:"applied,deferred,failed,unsupported"`
	Error     *MigrationError       `json:"error"`
	Conflicts []migrations.Conflict `json:"conflicts"`
	UpdatedAt time.Time             `json:"updated_at"`
}

/*
MigrationError is why a record could not be applied.
*/
type MigrationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

/*
MigrationRollback is where undoing a migration stands.
*/
type MigrationRollback struct {
	Removed     int32      `json:"removed"`
	Error       *string    `json:"error"`
	RequestedAt *time.Time `json:"requested_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

/*
NewMigration shapes the migration with its progress.
*/
func NewMigration(m migrations.Migration) Migration {
	row := m.Row
	out := Migration{
		ID: models.PublicID(models.PrefixMigration, row.ID), Object: "migration", Source: row.Source,
		SourceAccount: row.SourceAccount, Name: row.Name, FormatVersion: row.FormatVersion, Status: row.Status,
		Counts: []MigrationKindCount{}, Problems: []MigrationRecordProblem{}, LastRecordAt: row.LastRecordAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}

	for _, c := range m.Counts {
		out.Counts = append(out.Counts, MigrationKindCount{
			Kind: c.Kind, Applied: c.Applied, Created: c.Created, Deferred: c.Deferred, Failed: c.Failed,
			Unsupported: c.Unsupported, Conflicted: c.Conflicted,
		})
	}

	for _, p := range m.Problems {
		out.Problems = append(out.Problems, newMigrationProblem(p))
	}

	if row.RollbackRequestedAt != nil {
		out.Rollback = &MigrationRollback{
			Removed: row.RollbackRemoved, Error: row.RollbackError, RequestedAt: row.RollbackRequestedAt,
			FinishedAt: row.RollbackFinishedAt,
		}
	}

	return out
}

/*
newMigrationProblem shapes a record's problem.
*/
func newMigrationProblem(r models.MigrationRecord) MigrationRecordProblem {
	out := MigrationRecordProblem{
		Kind: r.Kind, SourceID: r.SourceID, Status: r.Status, Conflicts: []migrations.Conflict{}, UpdatedAt: r.UpdatedAt,
	}

	if len(r.Error) > 0 && string(r.Error) != "null" {
		var e MigrationError
		if json.Unmarshal(r.Error, &e) == nil {
			out.Error = &e
		}
	}

	_ = json.Unmarshal(r.Conflicts, &out.Conflicts)
	if out.Conflicts == nil {
		out.Conflicts = []migrations.Conflict{}
	}

	return out
}

/*
NewMigrationSummaries shapes the migrations of a list.
*/
func NewMigrationSummaries(rows []models.Migration) []MigrationSummary {
	out := make([]MigrationSummary, 0, len(rows))

	for _, row := range rows {
		out = append(out, MigrationSummary{
			ID: models.PublicID(models.PrefixMigration, row.ID), Object: "migration", Source: row.Source,
			SourceAccount: row.SourceAccount, Name: row.Name, Status: row.Status, LastRecordAt: row.LastRecordAt,
			CreatedAt: row.CreatedAt,
		})
	}

	return out
}

/*
MigrationBatch is what a batch of records came to.
*/
type MigrationBatch struct {
	Object   string                  `json:"object"`
	Kind     string                  `json:"kind"`
	Results  []MigrationRecordResult `json:"results"`
	Resolved []MigrationRecordResult `json:"resolved"`
}

/*
MigrationRecordResult is what one record came to: created, updated,
unchanged, deferred until the records it names arrive, failed, or not
supported yet; the dimmi id it became, with warnings about fields left
out, conflicts with changes made here, and why it failed.
*/
type MigrationRecordResult struct {
	Kind       string                `json:"kind"`
	SourceID   string                `json:"source_id"`
	Status     string                `json:"status"      enum:"created,updated,unchanged,deferred,failed,unsupported"`
	ID         *string               `json:"id"`
	Warnings   []migrations.Notice   `json:"warnings"`
	Conflicts  []migrations.Conflict `json:"conflicts"`
	Error      *MigrationError       `json:"error"`
	WaitingFor []migrations.Ref      `json:"waiting_for"`
}

/*
NewMigrationBatch shapes a batch's results.
*/
func NewMigrationBatch(kind string, b migrations.Batch) MigrationBatch {
	return MigrationBatch{
		Object: "migration_batch", Kind: kind, Results: newRecordResults(b.Results), Resolved: newRecordResults(b.Resolved),
	}
}

/*
newRecordResults shapes records' results.
*/
func newRecordResults(list []migrations.Result) []MigrationRecordResult {
	out := make([]MigrationRecordResult, 0, len(list))

	for _, r := range list {
		res := MigrationRecordResult{
			Kind: r.Kind, SourceID: r.SourceID, Status: r.Outcome, Warnings: emptyIfNil(r.Warnings),
			Conflicts: emptyIfNil(r.Conflicts), WaitingFor: emptyIfNil(r.WaitingFor),
		}

		if r.RecordID != nil {
			res.ID = new(models.PublicID(migrations.PrefixOf(r.Kind), *r.RecordID))
		}

		if r.Problem != nil {
			res.Error = &MigrationError{Code: r.Problem.Code, Message: r.Problem.Message, Field: r.Problem.Field}
		}

		out = append(out, res)
	}

	return out
}

/*
emptyIfNil returns the list, empty instead of nil.
*/
func emptyIfNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}

	return list
}
