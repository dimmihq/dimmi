/*
|--------------------------------------------------------------------------
| Import Resource
|--------------------------------------------------------------------------
|
| An import as the teammate running it sees it: the file's header and
| first rows, the mapping its headers suggest and the one chosen, where
| the run stands and what it came to, the first failed rows and, once
| any failed, where their report downloads from. The file itself stays
| inside.
|
*/

package resources

import (
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
		FinishedAt: row.FinishedAt, CreatedAt: row.CreatedAt,
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
}

/*
NewImportSummaries shapes a list of imports.
*/
func NewImportSummaries(rows []models.ListImportsRow) []ImportSummary {
	out := make([]ImportSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, ImportSummary{
			ID: models.PublicID(models.PrefixImport, r.ID), Object: "import", Kind: r.Kind, Status: r.Status,
			FileName: r.FileName, RowCount: r.RowCount, ProcessedRows: r.ProcessedRows, CreatedCount: r.CreatedCount,
			UpdatedCount: r.UpdatedCount, SkippedCount: r.SkippedCount, FailedCount: r.FailedCount,
			FinishedAt: r.FinishedAt, CreatedAt: r.CreatedAt,
		})
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
