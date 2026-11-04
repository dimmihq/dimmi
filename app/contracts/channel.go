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
| Two directions meet the inbox here. Outbound, the inbox hands a driver
| one message to deliver and learns what became of it. Inbound, a
| channel hands the inbox one message it received, and the inbox decides
| which contact and conversation it belongs to; channels never write
| conversations, messages or contacts themselves.
|
| A driver says what it can do, and the inbox decides from that alone:
| whether a reply goes out through a job, shows at once, or is only read
| through the API, and whether the team may start a conversation on it.
|
*/

package contracts

import (
	"context"
	"io"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
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
		idempotency key to providers that accept one, never delivers it
		twice where the provider allows avoiding that, and respects the
		context's deadline.
	*/
	Send(ctx context.Context, conn models.ChannelConnection, msg OutboundMessage) (SendResult, error)
}

/*
ChannelCapabilities is what a driver can do on a connection.

Transport says how replies travel and StartConversation whether the team
may open a thread. MaxBodyChars bounds a formatted body, zero meaning no
limit. The attachment limits use the global ones when zero. RetryWindow
is how long a transient failure keeps being retried, the inbox default
when zero.
*/
type ChannelCapabilities struct {
	Transport               Transport
	StartConversation       StartMode
	Formatting              Formatting
	MaxBodyChars            int
	Subject                 bool
	Attachments             bool
	MaxAttachments          int
	MaxAttachmentBytes      int64
	MaxTotalAttachmentBytes int64
	AllowedContentTypes     []string
	Typing                  bool
	DeliveryReceipts        bool
	ReadReceipts            bool
	ParticipantsCC          bool
	MessagingWindow         time.Duration
	RetryWindow             time.Duration
}

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
Formatting is the markup a channel renders.
*/
type Formatting string

/*
The formattings.
*/
const (
	FormatMarkdown Formatting = "markdown"
	FormatHTML     Formatting = "html"
	FormatPlain    Formatting = "plain"
)

/*
OutboundMessage is one reply on its way to a customer.

IdempotencyKey stays the same across attempts. ExternalThreadKey is
empty until the conversation has a thread on the channel. The body
comes as Markdown, plain text and safe HTML. Recipient is who the reply
goes to and CC who is copied. ReplyTo lists the channel ids of earlier
messages of the thread, newest last. Options is the raw channel options
of the request, nil when absent.
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
	BodyText           string
	BodyHTML           string
	Recipient          Recipient
	CC                 []Recipient
	Author             OutboundAuthor
	Attachments        []OutboundAttachment
	ReplyTo            []string
	Options            []byte
	IsTest             bool
}

/*
Recipient is the customer a reply goes to, by the identity the channel
reaches them with, such as their email address.
*/
type Recipient struct {
	ContactID     ids.UUID
	Name          string
	IdentityType  enums.IdentityType
	IdentityValue string
	Locale        string
}

/*
OutboundAuthor is who wrote a reply, as the customer sees them.
*/
type OutboundAuthor struct {
	Type      enums.AuthorType
	ID        ids.UUID
	Name      string
	AvatarURL string
}

/*
OutboundAttachment is a file sent with a message. Open streams it and
the caller closes what it returns.
*/
type OutboundAttachment struct {
	ID          ids.UUID
	Filename    string
	ContentType string
	SizeBytes   int64
	SHA256      string
	Inline      bool
	ContentID   string
	Open        func(ctx context.Context) (io.ReadCloser, error)
	SignedURL   string
}
