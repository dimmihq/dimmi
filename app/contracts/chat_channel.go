/*
|--------------------------------------------------------------------------
| Chat Channel Extras
|--------------------------------------------------------------------------
|
| Chat apps are shared places where the conversation stays visible to
| everyone in it, so their drivers do more than deliver replies. They
| show that the agent is writing, mark a conversation closed where the
| customer sees it, take back the buttons of an answer once they are
| used, and follow a teammate who deletes a reply already sent. Each is
| an optional interface a driver may implement; callers type-assert,
| and a driver without one simply skips that behavior.
|
| A provider is the source of truth for what its users wrote: when a
| customer edits or deletes a message there, the inbox follows.
|
*/

package contracts

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The answer controls an agent's answer may offer.
*/
const (
	ControlSolved  = "solved"
	ControlHandoff = "handoff"
)

/*
ChatMessageRef names one message on a chat app: the conversation's
thread key and the provider's id of the message. AuthorMemberID is the
teammate who sent it, when one did: a reply posted with their own
account can only be removed with it.
*/
type ChatMessageRef struct {
	ThreadKey        string
	ChannelMessageID string
	AuthorMemberID   *ids.UUID
}

/*
ChannelWorkingIndicator shows on a chat app that the agent is writing
an answer to a message, and stops showing it.
*/
type ChannelWorkingIndicator interface {
	/*
		StartWorking shows the agent is writing an answer to the message.
	*/
	StartWorking(ctx context.Context, conn models.ChannelConnection, ref ChatMessageRef) error

	/*
		StopWorking takes the sign away again; a sign already gone is no
		error.
	*/
	StopWorking(ctx context.Context, conn models.ChannelConnection, ref ChatMessageRef) error
}

/*
ChannelStateMarker shows on a chat app whether a conversation is
closed, on the thread its customer reads.
*/
type ChannelStateMarker interface {
	/*
		MarkState marks the thread closed when closed is true and clears
		the mark otherwise. A mark already in place is no error.
	*/
	MarkState(ctx context.Context, conn models.ChannelConnection, threadKey string, closed bool) error
}

/*
ChannelControlsUpdater takes the buttons off an answer once a customer
used them or a teammate replied.
*/
type ChannelControlsUpdater interface {
	/*
		ClearControls removes the answer controls from the sent message,
		keeping its text.
	*/
	ClearControls(ctx context.Context, conn models.ChannelConnection, ref ChatMessageRef, body string) error
}

/*
ChannelMessageEditor follows a teammate who deletes a reply the channel
already delivered.
*/
type ChannelMessageEditor interface {
	/*
		DeleteSent deletes the delivered reply on the channel.
	*/
	DeleteSent(ctx context.Context, conn models.ChannelConnection, ref ChatMessageRef) error
}

/*
AgentWorking hears when the agent starts and ends writing an answer to
a customer's message, so a chat app can show it.
*/
type AgentWorking interface {
	/*
		Started is called when the agent begins answering the message.
	*/
	Started(ctx context.Context, workspaceID, conversationID, messageID ids.UUID)

	/*
		Stopped is called when the agent's answer is written or dropped.
	*/
	Stopped(ctx context.Context, workspaceID, conversationID, messageID ids.UUID)
}

/*
EchoAuthor is who answered natively on a chat app. MemberID is the
linked teammate, whose answer is stored as their reply. Helper marks a
community helper without a link, whose answer stays theirs as a
customer but still counts as an answer.
*/
type EchoAuthor struct {
	MemberID *ids.UUID
	Helper   bool
}

/*
InboundEdit is a customer's edit at the provider of a message the inbox
holds, found by the connection and the provider's id of the message.
*/
type InboundEdit struct {
	ConnectionID     ids.UUID
	ChannelMessageID string
	Body             string
	EditedAt         time.Time
}

/*
InboundDeletion is a deletion at the provider of a message the inbox
holds. Reason says why, such as source_deleted.
*/
type InboundDeletion struct {
	ConnectionID     ids.UUID
	ChannelMessageID string
	Reason           string
}
