/*
|--------------------------------------------------------------------------
| Channel
|--------------------------------------------------------------------------
|
| A channel is one way conversations reach customers: the messenger,
| email, a chat app, or the REST API. Every channel speaks to the inbox
| through this one contract, so the inbox stores, assigns and replies the
| same way whatever the channel, and adding a channel is new code, never
| a change to the inbox.
|
| A driver says what it can do, and the inbox decides from that alone:
| whether a reply goes out through a job, shows at once, or is only read
| through the API, and whether the team may start a conversation on it.
|
*/

package contracts

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Transport is how a reply reaches the customer: pushed by a job to a
provider, shown at once in a live session, or not sent at all, left for
the developer to read through the API.
*/
type Transport string

/*
The transports.
*/
const (
	TransportPush     Transport = "push"
	TransportRealtime Transport = "realtime"
	TransportNone     Transport = "none"
)

/*
StartMode says whether the team may open a thread on a channel: never,
always, or only with a template the provider approved.
*/
type StartMode string

/*
The start modes.
*/
const (
	StartNever        StartMode = "never"
	StartAlways       StartMode = "always"
	StartWithTemplate StartMode = "with_template"
)

/*
Errors a driver's Send wraps to say how a delivery failed. Anything else
is transient and tried again.

ErrPermanent fails the message at once. ErrRecipientUnreachable means
the customer cannot be reached on the channel, such as a hard bounce.
ErrConnectionBroken means the connection's credentials no longer work.
*/
var (
	ErrPermanent            = errors.New("channel: permanent failure")
	ErrRecipientUnreachable = errors.New("channel: recipient unreachable")
	ErrConnectionBroken     = errors.New("channel: connection credentials invalid or revoked")
)

/*
Channel is one channel driver, one per connection type, stateless and
safe for concurrent use. Per-connection state comes in with the
connection.
*/
type Channel interface {
	/*
		Type names the connection type the driver serves, such as email.
	*/
	Type() string

	/*
		Capabilities says what the driver can do on the connection. It must
		be cheap: no network.
	*/
	Capabilities(conn models.ChannelConnection) ChannelCapabilities

	/*
		Send delivers one outbound message. It may be called again for the
		same message after a transient failure, so it passes the message's
		idempotency key to providers that accept one, and it respects the
		context's deadline.
	*/
	Send(ctx context.Context, conn models.ChannelConnection, msg OutboundMessage) (SendResult, error)
}

/*
ChannelCapabilities is what a driver can do on a connection: how replies
travel (Transport) and whether the team may start a thread
(StartConversation).
*/
type ChannelCapabilities struct {
	Transport         Transport
	StartConversation StartMode
}

/*
OutboundMessage is one reply on its way to a customer. IdempotencyKey
stays the same across attempts; ExternalThreadKey is empty until the
channel created a thread. Recipient is who the reply goes to.
*/
type OutboundMessage struct {
	WorkspaceID        ids.UUID
	ConversationID     ids.UUID
	ConversationNumber int64
	MessageID          ids.UUID
	IdempotencyKey     string
	ExternalThreadKey  string
	Subject            string
	BodyMarkdown       string
	Recipient          Recipient
	Author             OutboundAuthor
}

/*
Recipient is the customer a reply goes to: their contact, name and
email address, empty when they have none.
*/
type Recipient struct {
	ContactID ids.UUID
	Name      string
	Email     string
}

/*
OutboundAuthor is who a reply is from, as the customer sees them.
*/
type OutboundAuthor struct {
	Type string
	ID   ids.UUID
	Name string
}

/*
SendResult is what a delivered message became on the channel: the
provider's id for it, required unless the channel sends nothing, and the
thread key when the send created the thread. SentAt is the provider's
time, zero when it gave none.
*/
type SendResult struct {
	ChannelMessageID  string
	ExternalThreadKey string
	SentAt            time.Time
}
