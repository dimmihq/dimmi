/*
|--------------------------------------------------------------------------
| Import Resource
|--------------------------------------------------------------------------
|
| An import as the teammate running it sees it: the file's header and
| first rows, the mapping its headers suggest and the one chosen, where
| the run stands and what it came to, the first failed rows and, once
| any failed, where their report downloads from. Until when it can be
| undone, and where an undo stands. The file itself stays inside.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/imports"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Import is one import.
*/
type Import struct {
	ID               string             `json:"id"`
	Object           string             `json:"object"`
	Kind             string             `json:"kind"`
	Status           string             `json:"status"`
	FileName         string             `json:"file_name"`
	RowCount         int32              `json:"row_count"`
	Columns          []string           `json:"columns"`
	Preview          [][]string         `json:"preview"`
	SuggestedMapping []imports.Mapping  `json:"suggested_mapping"`
	Mapping          []imports.Mapping  `json:"mapping"`
	Mode             string             `json:"mode"`
	ProcessedRows    int32              `json:"processed_rows"`
	CreatedCount     int32              `json:"created_count"`
	UpdatedCount     int32              `json:"updated_count"`
	SkippedCount     int32              `json:"skipped_count"`
	FailedCount      int32              `json:"failed_count"`
	Errors           []imports.RowError `json:"errors"`
	ErrorReportURL   *string            `json:"error_report_url"`
	Error            *string            `json:"error"`
	StartedAt        *time.Time         `json:"started_at"`
	FinishedAt       *time.Time         `json:"finished_at"`
	CreatedAt        time.Time          `json:"created_at"`
	RollbackUntil    *time.Time         `json:"rollback_until"`
	Rollback         *ImportRollback    `json:"rollback"`
	FileExpiredAt    *time.Time         `json:"file_expired_at"`
}

/*
ImportRollback is where undoing an import stands and what it came to:
records removed, records put back, and records kept, the first of them
named with why.
*/
type ImportRollback struct {
	Status      string               `json:"status"`
	Total       int32                `json:"total"`
	Processed   int32                `json:"processed"`
	Removed     int32                `json:"removed"`
	Reverted    int32                `json:"reverted"`
	Kept        int32                `json:"kept"`
	KeptRecords []imports.KeptRecord `json:"kept_records"`
	Error       *string              `json:"error"`
	RequestedAt *time.Time           `json:"requested_at"`
	FinishedAt  *time.Time           `json:"finished_at"`
}

/*
rollbackState is an import's undo as its row keeps it.
*/
type rollbackState struct {
	status                                    *string
	total, processed, removed, reverted, kept int32
	keptRecords                               json.RawMessage
	err                                       *string
	requestedAt, finishedAt                   *time.Time
}

/*
newImportRollback shapes an import's undo, nil when none was asked for.
*/
func newImportRollback(r rollbackState) *ImportRollback {
	if r.status == nil {
		return nil
	}

	kept := []imports.KeptRecord{}
	_ = json.Unmarshal(r.keptRecords, &kept)

	return &ImportRollback{
		Status: *r.status, Total: r.total, Processed: r.processed, Removed: r.removed, Reverted: r.reverted,
		Kept: r.kept, KeptRecords: kept, Error: r.err, RequestedAt: r.requestedAt, FinishedAt: r.finishedAt,
	}
}

/*
NewImport shapes the import.
*/
func NewImport(v imports.View) Import {
	row := v.Import
	out := Import{
		ID: models.PublicID(models.PrefixImport, row.ID), Object: "import", Kind: row.Kind, Status: row.Status,
		FileName: row.FileName, RowCount: row.RowCount, Columns: v.Columns, Preview: v.Preview,
		SuggestedMapping: v.Suggested, Mapping: v.Mapping, Mode: row.Mode, ProcessedRows: row.ProcessedRows,
		CreatedCount: row.CreatedCount, UpdatedCount: row.UpdatedCount, SkippedCount: row.SkippedCount,
		FailedCount: row.FailedCount, Errors: v.Errors, Error: row.Error, StartedAt: row.StartedAt,
		FinishedAt: row.FinishedAt, CreatedAt: row.CreatedAt, RollbackUntil: imports.RollbackDeadline(row.FinishedAt),
		FileExpiredAt: row.FileExpiredAt,
		Rollback: newImportRollback(rollbackState{
			status: row.RollbackStatus, total: row.RollbackTotal, processed: row.RollbackProcessed,
			removed: row.RollbackRemoved, reverted: row.RollbackReverted, kept: row.RollbackKept,
			keptRecords: row.RollbackKeptRecords, err: row.RollbackError, requestedAt: row.RollbackRequestedAt,
			finishedAt: row.RollbackFinishedAt,
		}),
	}

	if row.FailedCount > 0 {
		out.ErrorReportURL = new("/api/v1/imports/" + out.ID + "/errors")
	}

	return out
}

/*
ImportSummary is an import in a list, without its file's contents.
*/
type ImportSummary struct {
	ID            string     `json:"id"`
	Object        string     `json:"object"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	FileName      string     `json:"file_name"`
	RowCount      int32      `json:"row_count"`
	ProcessedRows int32      `json:"processed_rows"`
	CreatedCount  int32      `json:"created_count"`
	UpdatedCount  int32      `json:"updated_count"`
	SkippedCount  int32      `json:"skipped_count"`
	FailedCount   int32      `json:"failed_count"`
	FinishedAt    *time.Time `json:"finished_at"`
	CreatedAt     time.Time  `json:"created_at"`

	CreatedByName  *string         `json:"created_by_name"`
	ErrorReportURL *string         `json:"error_report_url"`
	RollbackUntil  *time.Time      `json:"rollback_until"`
	Rollback       *ImportRollback `json:"rollback"`
	FileExpiredAt  *time.Time      `json:"file_expired_at"`
}

/*
NewImportSummaries shapes a list of imports.
*/
func NewImportSummaries(rows []models.ListImportsRow) []ImportSummary {
	out := make([]ImportSummary, 0, len(rows))
	for _, r := range rows {
		item := ImportSummary{
			ID: models.PublicID(models.PrefixImport, r.ID), Object: "import", Kind: r.Kind, Status: r.Status,
			FileName: r.FileName, RowCount: r.RowCount, ProcessedRows: r.ProcessedRows, CreatedCount: r.CreatedCount,
			UpdatedCount: r.UpdatedCount, SkippedCount: r.SkippedCount, FailedCount: r.FailedCount,
			FinishedAt: r.FinishedAt, CreatedAt: r.CreatedAt, CreatedByName: r.CreatedByName,
			RollbackUntil: imports.RollbackDeadline(r.FinishedAt), FileExpiredAt: r.FileExpiredAt,
			Rollback: newImportRollback(rollbackState{
				status: r.RollbackStatus, total: r.RollbackTotal, processed: r.RollbackProcessed,
				removed: r.RollbackRemoved, reverted: r.RollbackReverted, kept: r.RollbackKept,
				keptRecords: r.RollbackKeptRecords, err: r.RollbackError, requestedAt: r.RollbackRequestedAt,
				finishedAt: r.RollbackFinishedAt,
			}),
		}

		if r.FailedCount > 0 {
			item.ErrorReportURL = new("/api/v1/imports/" + item.ID + "/errors")
		}

		out = append(out, item)
	}

	return out
}

/*
ImportPrediction is what a dry run found the rows would come to.
*/
type ImportPrediction struct {
	Object  string             `json:"object"`
	Rows    int                `json:"rows"`
	Created int                `json:"created"`
	Updated int                `json:"updated"`
	Skipped int                `json:"skipped"`
	Failed  int                `json:"failed"`
	Errors  []imports.RowError `json:"errors"`
}

/*
NewImportPrediction shapes a dry run.
*/
func NewImportPrediction(p imports.Prediction) ImportPrediction {
	return ImportPrediction{
		Object: "import_preview", Rows: p.Rows, Created: p.Created, Updated: p.Updated, Skipped: p.Skipped,
		Failed: p.Failed, Errors: p.Errors,
	}
}
