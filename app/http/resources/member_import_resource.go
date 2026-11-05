/*
|--------------------------------------------------------------------------
| Member Import Resources
|--------------------------------------------------------------------------
|
| The report of a member import: how many rows created, changed, left
| alone or failed, and each row's own outcome, so an admin can fix the
| failed rows in the spreadsheet and run the file again.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/memberimports"
)

/*
MemberImport is the report of one import, dry run or not.
*/
type MemberImport struct {
	Object    string            `json:"object"`
	DryRun    bool              `json:"dry_run"`
	Created   int               `json:"created"`
	Updated   int               `json:"updated"`
	Unchanged int               `json:"unchanged"`
	Failed    int               `json:"failed"`
	Rows      []MemberImportRow `json:"rows"`
}

/*
MemberImportRow is the outcome of one row: its line in the file, its
address, the result, the fields it changed and why it failed.
*/
type MemberImportRow struct {
	Row     int              `json:"row"`
	Email   string           `json:"email"`
	Result  string           `json:"result"`
	Changes []string         `json:"changes"`
	Error   *InvitationError `json:"error,omitempty"`
}

/*
NewMemberImport shapes the report. Message turns a row's refusal into
its code and message for the reader.
*/
func NewMemberImport(r memberimports.Report, message func(error) InvitationError) MemberImport {
	rows := make([]MemberImportRow, 0, len(r.Rows))
	for _, row := range r.Rows {
		out := MemberImportRow{Row: row.Row, Email: row.Email, Result: row.Result, Changes: row.Changes}
		if out.Changes == nil {
			out.Changes = []string{}
		}

		if row.Err != nil {
			refusal := message(row.Err)
			out.Error = &refusal
		}

		rows = append(rows, out)
	}

	return MemberImport{
		Object: "member_import", DryRun: r.DryRun, Created: r.Created, Updated: r.Updated,
		Unchanged: r.Unchanged, Failed: r.Failed, Rows: rows,
	}
}
