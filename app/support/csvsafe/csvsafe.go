/*
|--------------------------------------------------------------------------
| Formula-Safe CSV
|--------------------------------------------------------------------------
|
| Every CSV file the application hands out holds values people typed,
| such as names and labels, and is opened in spreadsheets that run any
| cell starting with a formula sign. A cell that could start a formula
| gets a leading apostrophe, which spreadsheets read as "this is text",
| so a crafted name never runs on the reader's machine.
|
*/

package csvsafe

import (
	"encoding/csv"
	"fmt"
	"io"
)

/*
formulaStarts are the first characters that make a spreadsheet read a
cell as a formula, or let it hide one behind a tab or a line break.
*/
const formulaStarts = "=+-@\t\r"

/*
Cell returns the value as it is safe to write: unchanged, or with a
leading apostrophe when it starts like a formula.
*/
func Cell(value string) string {
	if value == "" {
		return value
	}

	for _, start := range formulaStarts {
		if rune(value[0]) == start {
			return "'" + value
		}
	}

	return value
}

/*
Writer writes records to a CSV file, guarding every cell.
*/
type Writer struct {
	csv *csv.Writer
}

/*
NewWriter returns a writer that writes to w.
*/
func NewWriter(w io.Writer) *Writer {
	return &Writer{csv: csv.NewWriter(w)}
}

/*
Write writes one record, each cell guarded. Records are buffered: call
Flush to send them on.
*/
func (w *Writer) Write(record []string) error {
	safe := make([]string, len(record))
	for i, value := range record {
		safe[i] = Cell(value)
	}

	if err := w.csv.Write(safe); err != nil {
		return fmt.Errorf("write csv record: %w", err)
	}

	return nil
}

/*
Flush sends the buffered records on and reports the first write that
failed.
*/
func (w *Writer) Flush() error {
	w.csv.Flush()

	if err := w.csv.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}

	return nil
}
