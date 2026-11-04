/*
|--------------------------------------------------------------------------
| Inbox Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a team works its conversations and contacts.
| Each refusal names the field it concerns where there is one, so a form
| shows it beside the right input, and carries a stable code programs
| can branch on.
|
*/

package exceptions

import (
	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
Conversation state failures.

ErrSnoozeInPast and ErrSnoozeTooFar refuse a wake-up time less than a
minute away or more than a year away. ErrMemberInactive refuses to hand
a conversation to someone who is not an active member, and
ErrTeamNotFound to a team the workspace does not have. ErrInboxNotFound
refuses an inbox the workspace does not have.
*/
var (
	ErrSnoozeInPast = fault.New(fault.Invalid, "snooze_in_past",
		"Snooze until at least a minute from now.").OnField("snoozed_until")
	ErrSnoozeTooFar = fault.New(fault.Invalid, "snooze_too_far",
		"A conversation can be snoozed for a year at most.").OnField("snoozed_until")
	ErrMemberInactive = fault.New(fault.Invalid, "member_inactive",
		"Conversations can only go to active members of this workspace.").OnField("assignee_id")
	ErrTeamNotFound = fault.New(fault.Invalid, "team_not_found",
		"This workspace has no such team.").OnField("team_id")
	ErrInboxNotFound = fault.New(fault.Invalid, "inbox_not_found",
		"This workspace has no such inbox.").OnField("inbox_id")
)

/*
Message failures.

ErrNewMessagesSince refuses a reply written without seeing the messages
that arrived meanwhile; callers add them under data. ErrConversationSpam
refuses a reply in a conversation marked as spam. ErrChannelNotAPI
refuses a customer message through the API on a conversation of another
channel. ErrMessageNotInConversation refuses a read marker on a message
of another conversation. ErrBodyRequired refuses an empty message.
*/
var (
	ErrNewMessagesSince = fault.New(fault.Conflict, "new_messages_since",
		"New messages arrived since you started typing.")
	ErrConversationSpam = fault.New(fault.Conflict, "conversation_spam",
		"This conversation is marked as spam. Take it out of spam to reply.")
	ErrChannelNotAPI = fault.New(fault.Conflict, "channel_not_api",
		"Customer messages can only be added through the API to API conversations.")
	ErrMessageNotInConversation = fault.New(fault.Invalid, "message_not_in_conversation",
		"This message is not part of the conversation.").OnField("last_read_message_id")
	ErrBodyRequired = fault.New(fault.Invalid, "body_required", "Write a message first.").OnField("body")
)

/*
Channel failures.

ErrChannelCannotInitiate refuses to start a conversation on a channel
where only the customer can. ErrChannelUnsupported refuses to reply on
a channel this server has no driver for. ErrContactUnreachable refuses
a reply to a customer the channel has no address for. ErrNoAPIChannel
refuses to start a conversation through the API in an inbox without an
API channel.
*/
var (
	ErrChannelCannotInitiate = fault.New(fault.Invalid, "channel_cannot_initiate",
		"Conversations on this channel can only be started by the customer.").OnField("channel_connection_id")
	ErrChannelUnsupported = fault.New(fault.Invalid, "channel_unsupported",
		"This server cannot send on this channel.")
	ErrContactUnreachable = fault.New(fault.Invalid, "contact_unreachable_on_channel",
		"This customer has no address on the conversation's channel.")
	ErrNoAPIChannel = fault.New(fault.Invalid, "inbox_has_no_api_channel",
		"This inbox has no API channel to start conversations on.").OnField("inbox_id")
)

/*
Contact failures.

ErrContactExists refuses a new contact with an external id another
contact holds. ErrContactKeyRequired refuses to start a conversation
without saying who the customer is. ErrInvalidAttributes refuses custom
attributes with a key that is not a short snake_case name, a value that
is not text, a number or a boolean, text over 1,024 characters, or more
than a hundred of them.
*/
var (
	ErrInvalidAttributes = fault.New(fault.Invalid, "invalid_custom_attributes",
		"Custom attributes need snake_case keys and short plain values, at most 100 of them.").
		OnField("custom_attributes")
	ErrContactExists = fault.New(fault.Conflict, "contact_exists",
		"Another contact already has this external id.").OnField("external_id")
	ErrContactKeyRequired = fault.New(fault.Invalid, "contact_required",
		"Name the customer by id, external id or email.").OnField("contact")
)

/*
Search failures.

ErrSearchQueryTooLong refuses a search longer than the server reads.
*/
var (
	ErrSearchQueryTooLong = fault.New(fault.Invalid, "search_query_too_long",
		"Search for at most 256 characters.").OnField("q")
)

/*
ErrCursorMismatch refuses a cursor made for another list or order, so a
cursor taken elsewhere never silently skips rows. It is a malformed
request rather than invalid input.
*/
var ErrCursorMismatch = &fhttp.ResponseError{
	Status: fhttp.StatusBadRequest, Code: "cursor_mismatch",
	Message: "This cursor belongs to a different list.",
}

/*
Inbox preference failures.

ErrInvalidPreferences refuses a view no member has, or a sort, status or
layout no list accepts. ErrPreferencesTooLarge refuses preferences over
64 KB.
*/
var (
	ErrInvalidPreferences = fault.New(fault.Invalid, "invalid_preferences",
		"These preferences name a view or setting the inbox does not have.").OnField("views")
	ErrPreferencesTooLarge = fault.New(fault.Invalid, "preferences_too_large",
		"Inbox preferences can be 64 KB at most.")
)

/*
Saved view failures.

ErrInvalidDefinition refuses conditions a view cannot hold, naming the
path to the culprit. ErrViewLimitReached refuses a view past the
workspace's 200 shared or a member's 50 personal ones. ErrViewNameTaken
refuses a name another view of the same scope and owner already has.
*/
var (
	ErrInvalidDefinition = fault.New(fault.Invalid, "invalid_definition",
		"This view's conditions are not valid.").OnField("definition")
	ErrViewLimitReached = fault.New(fault.Invalid, "view_limit_reached",
		"There are as many views as allowed. Delete one to add another.")
	ErrViewNameTaken = fault.New(fault.Invalid, "view_name_taken",
		"Another view already has this name.").OnField("name")
)
