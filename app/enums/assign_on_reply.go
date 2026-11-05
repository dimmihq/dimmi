/*
|--------------------------------------------------------------------------
| Assign On Reply
|--------------------------------------------------------------------------
|
| Whether a member's reply takes a conversation nobody holds. Most people
| follow their inbox's rule; some always want what they answer, and some
| never do, such as a lead who only chimes in.
|
*/

package enums

/*
AssignOnReply is a member's own rule for taking conversations by
replying.
*/
type AssignOnReply string

/*
The rules: follow the inbox, always take, never take.
*/
const (
	AssignOnReplyInherit AssignOnReply = "inherit"
	AssignOnReplyAlways  AssignOnReply = "always"
	AssignOnReplyNever   AssignOnReply = "never"
)
