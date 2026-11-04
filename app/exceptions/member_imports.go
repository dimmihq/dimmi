/*
|--------------------------------------------------------------------------
| Member Import Errors
|--------------------------------------------------------------------------
|
| What can go wrong while changing members in bulk from a spreadsheet.
| A file that cannot be read as a whole is refused before any row runs.
| A row that cannot apply fails on its own and the rest go ahead, so the
| report names each failed row with the reason.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
File failures.

ErrImportUnreadable refuses a file that is not CSV text with a header.
ErrUnknownColumn refuses a header naming a column the import does not
know, and carries column. ErrMissingEmailColumn refuses a header without
the email column. ErrTooManyRows refuses a file with more rows than one
import takes, and carries max_rows. ErrTooManyImports refuses an import
once the workspace ran as many as it may this hour, and carries
retry_after.
*/
var (
	ErrImportUnreadable = fault.New(fault.Invalid, "invalid_csv",
		"The file must be a CSV file with a header row.").OnField("file")
	ErrUnknownColumn = fault.New(fault.Invalid, "unknown_column",
		"The file has a column the import does not know.").OnField("file")
	ErrMissingEmailColumn = fault.New(fault.Invalid, "missing_email_column",
		"The file needs an email column.").OnField("file")
	ErrTooManyRows = fault.New(fault.Invalid, "too_many_rows",
		"The file has more rows than one import takes.").OnField("file")
	ErrTooManyImports = fault.New(fault.RateLimited, "too_many_imports",
		"This workspace imported members too often. Try again later.")
)

/*
Row failures.

ErrImportRoleNotAllowed refuses a row that makes someone an owner.
ErrImportInvalidRole and ErrImportInvalidStatus refuse values that are
no role or status. ErrImportUnknownTeam refuses a team name the
workspace does not have. ErrImportDuplicateEmail refuses a second row
for one address. ErrImportOwnerNotChanged refuses a role or status
change of an owner. ErrImportCustomRole refuses a custom role, which
the workspace cannot hold yet. ErrImportUnknownMember refuses to
deactivate an address that is not a member.
*/
var (
	ErrImportRoleNotAllowed = fault.New(fault.Invalid, "role_not_allowed",
		"An import cannot make someone an owner.").OnField("role")
	ErrImportInvalidRole = fault.New(fault.Invalid, "invalid_role",
		"The role must be admin or member.").OnField("role")
	ErrImportInvalidStatus = fault.New(fault.Invalid, "invalid_status",
		"The status must be active or deactivated.").OnField("status")
	ErrImportUnknownTeam = fault.New(fault.Invalid, "unknown_team",
		"A team in this row does not exist in the workspace.").OnField("teams")
	ErrImportDuplicateEmail = fault.New(fault.Invalid, "duplicate_email",
		"An earlier row already names this address.").OnField("email")
	ErrImportOwnerNotChanged = fault.New(fault.Invalid, "owner_not_changed",
		"An owner's role and status cannot change through an import.").OnField("role")
	ErrImportCustomRole = fault.New(fault.Invalid, "custom_role_unavailable",
		"Custom roles are not available yet; leave the column empty.").OnField("custom_role")
	ErrImportUnknownMember = fault.New(fault.Invalid, "unknown_member",
		"Only a member of the workspace can be deactivated.").OnField("status")
)
