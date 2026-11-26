/*
|--------------------------------------------------------------------------
| Migration Batch Resources
|--------------------------------------------------------------------------
|
| What a batch of records came to: a result per record in the order
| sent, and the waiting records the batch let through, each with the id
| it became, warnings, conflicts and why it failed.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/migrations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MigrationError is why a record could not be applied.
*/
type MigrationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
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
		Object: "migration_batch", Kind: kind, Results: newRecordResults(b.Results),
		Resolved: newRecordResults(b.Resolved),
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
