/*
|--------------------------------------------------------------------------
| Customer Record Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a team keeps its records of customers and what
| they talk about: tags, companies, merges and the people on a thread.
| Each refusal carries a stable code programs can branch on.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Tag failures.

ErrTagLimitReached refuses a tag past the most a conversation, contact
or company may carry, or a new tag past the most a workspace may have.
ErrTagExists refuses a name another tag holds, in any case.
ErrTagNameRequired refuses a tag without a name, ErrTagIconUnknown an
icon the app does not have, and
ErrTagMessageNotInConversation a message of another conversation as
what prompted a tag.
*/
var (
	ErrTagLimitReached = fault.New(fault.Invalid, "tag_limit_reached",
		"A record holds at most 50 tags and a workspace at most 10,000.").OnField("tags")
	ErrTagExists = fault.New(fault.Conflict, "tag_exists",
		"Another tag already has this name.").OnField("name")
	ErrTagNameRequired = fault.New(fault.Invalid, "tag_name_required",
		"Give the tag a name.").OnField("name")
	ErrTagIconUnknown = fault.New(fault.Invalid, "tag_icon_unknown",
		"Pick one of the tag icons.").OnField("icon")
	ErrTagMessageNotInConversation = fault.New(fault.Invalid, "message_not_in_conversation",
		"This message is not part of the conversation.").OnField("message_id")
)

/*
Contact and company failures.

ErrUpsertKeyRequired refuses an upsert with no external id or email or
domain to match on. ErrExternalIDImmutable refuses an upsert that would
replace a contact's external id. ErrContactHasConversations refuses to
delete a customer with history outside the erase path. ErrCompanyNotFound
refuses a company the workspace does not have, ErrCompanyExists an
external id another company holds, and ErrCompanyNameRequired a new
company without a name. ErrInvalidUpsertFlag refuses an upsert parameter
that is neither true nor false.
*/
var (
	ErrUpsertKeyRequired = fault.New(fault.Invalid, "upsert_key_required",
		"Name the record by external id, or by email or domain.").OnField("external_id")
	ErrExternalIDImmutable = fault.New(fault.Conflict, "external_id_immutable",
		"This contact already has another external id.").OnField("external_id")
	ErrContactHasConversations = fault.New(fault.Conflict, "contact_has_conversations",
		"This contact has conversations. Erase them instead.")
	ErrCompanyNotFound = fault.New(fault.Invalid, "company_not_found",
		"This workspace has no such company.").OnField("company_id")
	ErrCompanyExists = fault.New(fault.Conflict, "company_exists",
		"Another company already has this external id.").OnField("external_id")
	ErrCompanyNameRequired = fault.New(fault.Invalid, "company_name_required",
		"Give the company a name.").OnField("name")
	ErrInvalidUpsertFlag = fault.New(fault.Invalid, "invalid_parameter",
		"upsert must be true or false.")
)
