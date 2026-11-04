/*
|--------------------------------------------------------------------------
| Member Import File
|--------------------------------------------------------------------------
|
| A member import is a CSV file people make in a spreadsheet, so it is
| read forgivingly: a byte order mark, columns in any order, spaces
| around cells and blank lines are all fine. What cannot be guessed is
| refused before any row runs: a missing header, a column the import does
| not know, no email column, or more rows than one import takes.
|
*/

package memberimports

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
The columns of an import file, and the most rows one file holds.
*/
const (
	columnEmail      = "email"
	columnRole       = "role"
	columnTeams      = "teams"
	columnCustomRole = "custom_role"
	columnStatus     = "status"
	MaxRows          = 1000
)

/*
row is one line of the file: its number counting the header as the
first line, and its cells by column, trimmed. Columns the file leaves
out read as empty.
*/
type row struct {
	number int
	cells  map[string]string
}

/*
cell returns the trimmed value of the column.
*/
func (r row) cell(column string) string {
	return r.cells[column]
}

/*
readFile reads the header and every non-blank row.

Errors: [exceptions.ErrImportUnreadable], [exceptions.ErrUnknownColumn],
[exceptions.ErrMissingEmailColumn] or [exceptions.ErrTooManyRows].
*/
func readFile(file io.Reader) ([]row, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, exceptions.ErrImportUnreadable
	}

	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, byteOrderMark())))
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, exceptions.ErrImportUnreadable
	}

	columns, err := readHeader(header)
	if err != nil {
		return nil, err
	}

	return readRows(reader, columns)
}

/*
readHeader names the column of each position, refusing names it does
not know and a header without an email column.
*/
func readHeader(header []string) ([]string, error) {
	known := map[string]bool{
		columnEmail: true, columnRole: true, columnTeams: true, columnCustomRole: true, columnStatus: true,
	}
	columns := make([]string, len(header))

	for i, name := range header {
		name = strings.ToLower(strings.TrimSpace(name))
		if !known[name] {
			return nil, exceptions.ErrUnknownColumn.With("column", name)
		}

		columns[i] = name
	}

	if slices.Contains(columns, columnEmail) {
		return columns, nil
	}

	return nil, exceptions.ErrMissingEmailColumn
}

/*
readRows reads the rows after the header, skipping blank ones.
*/
func readRows(reader *csv.Reader, columns []string) ([]row, error) {
	var rows []row

	for number := 2; ; number++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}

		if err != nil {
			return nil, exceptions.ErrImportUnreadable
		}

		r := row{number: number, cells: map[string]string{}}
		for i, value := range record {
			if i < len(columns) {
				r.cells[columns[i]] = strings.TrimSpace(value)
			}
		}

		if strings.Join(record, "") == "" {
			continue
		}

		if len(rows) == MaxRows {
			return nil, exceptions.ErrTooManyRows.With("max_rows", MaxRows)
		}

		rows = append(rows, r)
	}
}

/*
byteOrderMark is the mark spreadsheets put before UTF-8 text.
*/
func byteOrderMark() []byte {
	return []byte{0xEF, 0xBB, 0xBF}
}
