/*
|--------------------------------------------------------------------------
| Email Address Settings
|--------------------------------------------------------------------------
|
| How an email address replies and what it does with mail nobody wrote
| by hand. Replies carry a Reply-To address with the thread's token, so
| answers find their conversation, unless the workspace turns that off.
| The sender reads as the teammate from the team, or just the team.
| Newsletters and notifications either open a closed conversation, an
| open one, or nothing at all.
|
*/

package enums

/*
ReplyToMode is whether replies carry a reply token address.
*/
type ReplyToMode string

/*
The reply-to modes, in the order of the database check.
*/
const (
	ReplyToModeToken ReplyToMode = "token"
	ReplyToModeNone  ReplyToMode = "none"
)

/*
ReplyToModes returns every mode, in the order of the database check.
*/
func ReplyToModes() []ReplyToMode {
	return []ReplyToMode{ReplyToModeToken, ReplyToModeNone}
}

/*
FromNameStyle is how the sender of a reply is named.
*/
type FromNameStyle string

/*
The name styles, in the order of the database check.
*/
const (
	FromNameStyleMemberAndName FromNameStyle = "member_and_name"
	FromNameStyleNameOnly      FromNameStyle = "name_only"
)

/*
FromNameStyles returns every style, in the order of the database check.
*/
func FromNameStyles() []FromNameStyle {
	return []FromNameStyle{FromNameStyleMemberAndName, FromNameStyleNameOnly}
}

/*
BulkMailHandling is what becomes of bulk mail that starts no thread.
*/
type BulkMailHandling string

/*
The bulk mail handlings.
*/
const (
	BulkMailHandlingClose BulkMailHandling = "close"
	BulkMailHandlingOpen  BulkMailHandling = "open"
	BulkMailHandlingDrop  BulkMailHandling = "drop"
)

/*
EmailRefKind is how a Message-ID came to be filed under a
conversation.
*/
type EmailRefKind string

/*
The ref kinds: mail received, mail sent, the id a provider gave mail
sent, and an id only mentioned by mail received.
*/
const (
	EmailRefKindInbound          EmailRefKind = "inbound"
	EmailRefKindOutbound         EmailRefKind = "outbound"
	EmailRefKindOutboundProvider EmailRefKind = "outbound_provider"
	EmailRefKindReferenced       EmailRefKind = "referenced"
)
