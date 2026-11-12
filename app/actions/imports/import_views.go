/*
|--------------------------------------------------------------------------
| Import Views
|--------------------------------------------------------------------------
|
| What a teammate sees of an import: its header, its first rows, the
| mapping its headers suggest and the one chosen, and the first rows
| that failed. A dry run comes back as a prediction that counts what
| the rows would come to.
|
*/

package imports

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/app/models"
)

/*
How many rows a draft shows and how many failures a view or preview
names.
*/
const (
	previewRows = 5
	shownErrors = 20
)

/*
RowError is a row that could not be imported: its line in the file and
why.
*/
type RowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

/*
View is an import with what its file shows: the header, the first rows,
the suggested and chosen mappings, and the first failed rows.
*/
type View struct {
	Import    models.Import
	Columns   []string
	Preview   [][]string
	Suggested []Mapping
	Mapping   []Mapping
	Errors    []RowError
}

/*
Prediction is what a dry run found the rows would come to.
*/
type Prediction struct {
	Rows    int
	Created int
	Updated int
	Skipped int
	Failed  int
	Errors  []RowError
}

/*
count adds one row's outcome to the prediction.
*/
func (p *Prediction) count(outcome string, row int, problem string) {
	switch outcome {
	case OutcomeCreated:
		p.Created++
	case OutcomeUpdated:
		p.Updated++
	case OutcomeSkipped:
		p.Skipped++
	default:
		p.Failed++
		if len(p.Errors) < shownErrors {
			p.Errors = append(p.Errors, RowError{Row: row, Message: problem})
		}
	}
}

/*
describeImport returns the import with what its file shows.
*/
func describeImport(ctx context.Context, q *models.Queries, row models.Import) (View, error) {
	v := View{Import: row, Mapping: []Mapping{}, Errors: []RowError{}, Preview: [][]string{}}

	s, err := read(row.File)
	if err != nil {
		return v, err
	}

	v.Columns = s.columns
	for _, l := range s.rows[:min(previewRows, len(s.rows))] {
		cells := make([]string, len(s.columns))
		for i := range cells {
			cells[i] = l.cell(i)
		}

		v.Preview = append(v.Preview, cells)
	}

	defs, err := definitions(ctx, q, row)
	if err != nil {
		return v, err
	}

	v.Suggested = suggest(row.Kind, s.columns, defs)

	if err := json.Unmarshal(row.Mapping, &v.Mapping); err != nil {
		return v, fmt.Errorf("read mapping: %w", err)
	}

	var all []RowError
	if err := json.Unmarshal(row.RowErrors, &all); err != nil {
		return v, fmt.Errorf("read row errors: %w", err)
	}

	v.Errors = append(v.Errors, all[:min(shownErrors, len(all))]...)

	return v, nil
}
