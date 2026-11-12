/*
|--------------------------------------------------------------------------
| Import File
|--------------------------------------------------------------------------
|
| An import is a CSV people export from another tool or make in a
| spreadsheet, so it is read forgivingly: a byte order mark, short rows,
| stray quotes, spaces around cells and blank lines are all fine. What
| cannot be read is refused at upload, before anything is kept: no
| header, a file too big, or more rows than one import takes.
|
*/

package imports

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
)

/*
Limits of one import file.
*/
const (
	MaxBytes = 10 << 20
	MaxRows  = 50_000
)

/*
The failures of an uploaded file.
*/
var (
	ErrUnreadable = fault.New(fault.Invalid, "import_unreadable",
		"Upload a CSV file with a header row, 50,000 rows and 10 MB at most.").OnField("file")
	ErrEmpty = fault.New(fault.Invalid, "import_empty", "The file has a header but no rows.").OnField("file")
)

/*
sheet is a read file: its header and its non-blank rows, trimmed. A row
keeps its line in the file, counting the header as line one.
*/
type sheet struct {
	columns []string
	rows    []line
}

/*
line is one row of the file.
*/
type line struct {
	number int
	cells  []string
}

/*
cell returns the row's trimmed value in the column, empty where the row
is short.
*/
func (l line) cell(column int) string {
	if column < 0 || column >= len(l.cells) {
		return ""
	}

	return l.cells[column]
}

/*
read parses the file.

Errors: [ErrUnreadable] or [ErrEmpty].
*/
func read(data []byte) (sheet, error) {
	if len(data) == 0 || len(data) > MaxBytes {
		return sheet{}, ErrUnreadable
	}

	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	r.FieldsPerRecord, r.LazyQuotes, r.TrimLeadingSpace = -1, true, true

	header, err := r.Read()
	if err != nil {
		return sheet{}, ErrUnreadable
	}

	s := sheet{columns: trimmed(header)}
	if strings.Join(s.columns, "") == "" {
		return sheet{}, ErrUnreadable
	}

	for number := 2; ; number++ {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return sheet{}, ErrUnreadable
		}

		cells := trimmed(record)
		if strings.Join(cells, "") == "" {
			continue
		}

		if len(s.rows) == MaxRows {
			return sheet{}, ErrUnreadable
		}

		s.rows = append(s.rows, line{number: number, cells: cells})
	}

	if len(s.rows) == 0 {
		return sheet{}, ErrEmpty
	}

	return s, nil
}

/*
trimmed returns the cells without surrounding space.
*/
func trimmed(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = strings.TrimSpace(c)
	}

	return out
}
