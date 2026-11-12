/*
|--------------------------------------------------------------------------
| Merge Errors
|--------------------------------------------------------------------------
|
| What refuses a merge of conversations or of contacts, and a change to
| many conversations at once. Each carries a stable code programs can
| branch on.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Merge failures.

ErrMergeIntoItself refuses a merge that names the target among its
sources. ErrMergeOtherCustomer refuses to merge conversations of people
who are neither the same contact nor at the same company.
ErrMergeTwoUsers refuses to merge two contacts the host app identified
under different ids, since both would keep signing in as themselves.
ErrMergeTooMany refuses more sources than one merge takes.
ErrBulkTooMany refuses more conversations than one bulk change takes.
ErrMergedInto answers a request for a conversation merged into another;
its details name the target's id and number, so a client can follow.
*/
var (
	ErrMergeIntoItself = fault.New(fault.Invalid, "merge_into_itself",
		"A conversation or contact cannot be merged into itself.").OnField("source_ids")
	ErrMergeOtherCustomer = fault.New(fault.Invalid, "merge_other_customer",
		"Only conversations of the same customer, or of people at the same company, can be merged.").
		OnField("source_ids")
	ErrMergeTwoUsers = fault.New(fault.Conflict, "merge_two_users",
		"Both contacts are identified users with different ids, so they cannot be merged.").OnField("source_id")
	ErrMergeTooMany = fault.New(fault.Invalid, "merge_too_many",
		"Merge at most 20 conversations at once.").OnField("source_ids")
	ErrMergedInto = fault.New(fault.NotFound, "merged_into",
		"This conversation was merged into another.")
	ErrBulkTooMany = fault.New(fault.Invalid, "bulk_too_many",
		"Change at most 200 conversations at once.").OnField("conversation_ids")
)
