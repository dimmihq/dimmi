/*
|--------------------------------------------------------------------------
| Conversation Facts Export
|--------------------------------------------------------------------------
|
| A conversation facts export is the reports' rows: one line per
| conversation started in the report's range, with its filters, as the
| member who asked sees the workspace when the build runs.
|
*/

package dataexports

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
renderFacts writes the facts of the export's range as one sheet.
*/
func renderFacts(ctx context.Context, tx *database.Tx, m authz.Membership, row models.DataExport) ([]file, int, error) {
	var f reports.Filters
	if err := json.Unmarshal(row.Selection, &f); err != nil {
		return nil, 0, fmt.Errorf("read selection: %w", err)
	}

	lines, err := reports.FactRows(ctx, tx, reports.FactScope{
		Membership: m, Filters: f, Now: row.CreatedAt, Limit: limits()[row.Kind][row.Format],
	})
	if err != nil {
		return nil, 0, err
	}

	var buf bytes.Buffer

	w := csv.NewWriter(&buf)
	if err := w.WriteAll(lines); err != nil {
		return nil, 0, fmt.Errorf("write facts: %w", err)
	}

	return []file{{name: "conversation-facts.csv", data: buf.Bytes()}}, len(lines) - 1, nil
}
