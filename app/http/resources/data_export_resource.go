/*
|--------------------------------------------------------------------------
| Data Export Resource
|--------------------------------------------------------------------------
|
| An export as its member sees it: what it covers, where it stands and,
| once ready, where its file downloads from until it expires. The
| selection itself stays inside, since it holds internal ids.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/dataexports"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DataExport is one export. Warnings name what the file could not hold as
asked; URL is where a ready export downloads from.
*/
type DataExport struct {
	ID           string          `json:"id"`
	Object       string          `json:"object"`
	Kind         string          `json:"kind"`
	Format       string          `json:"format"`
	Status       string          `json:"status"`
	IncludeNotes bool            `json:"include_notes"`
	RowCount     *int64          `json:"row_count"`
	Warnings     json.RawMessage `json:"warnings"`
	FileName     string          `json:"file_name"`
	SizeBytes    *int64          `json:"size_bytes"`
	Error        *string         `json:"error"`
	ExpiresAt    *time.Time      `json:"expires_at"`
	URL          *string         `json:"url"`
	CreatedAt    time.Time       `json:"created_at"`
}

/*
NewDataExport shapes the export.
*/
func NewDataExport(e models.DataExport) DataExport {
	out := DataExport{
		ID: models.PublicID(models.PrefixDataExport, e.ID), Object: "data_export", Kind: e.Kind, Format: e.Format,
		Status: e.Status, IncludeNotes: e.IncludeNotes, RowCount: e.RowCount, Warnings: e.Warnings,
		FileName: dataexports.FileName(e), SizeBytes: e.SizeBytes, ExpiresAt: e.ExpiresAt, CreatedAt: e.CreatedAt,
	}

	if e.Status == "failed" {
		out.Error = new("The export could not be built.")
	}

	if e.Status == "ready" {
		out.URL = new("/api/v1/data-exports/" + out.ID + "/download")
	}

	return out
}

/*
NewDataExports shapes a list of exports.
*/
func NewDataExports(rows []models.DataExport) []DataExport {
	out := make([]DataExport, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewDataExport(row))
	}

	return out
}
