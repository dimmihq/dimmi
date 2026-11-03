/*
|--------------------------------------------------------------------------
| Channel
|--------------------------------------------------------------------------
|
| A channel adapter carries conversations to and from one kind of place
| customers write from: the messenger, email, a chat app. Two directions
| meet the inbox here. Outbound, the inbox hands a channel one message
| to deliver and learns what became of it. Inbound, a channel hands the
| inbox one message it received, and the inbox decides which contact
| and conversation it belongs to; channels never write conversations,
| messages or contacts themselves.
|
| Errors carry their meaning in sentinels: a permanent failure ends a
| message, an unwrapped error is transient and retried, and a rate
| limit postpones without spending an attempt.
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
Channel is one conversation channel adapter. One instance serves every
connection of its type and is safe for concurrent use; what differs per
connection comes in with the connection.
*/
type Channel interface {
	/*
		Type returns the channel type the adapter serves.
	*/
	Type() enums.ChannelType

	/*
		Capabilities returns what the channel can do on the connection.
		It never touches the network.
	*/
	Capabilities(conn *models.ChannelConnection) ChannelCapabilities

	/*
		Send delivers one outbound message. It may be called again for
		the same message when an attempt failed, and must not deliver it
		twice where the provider allows avoiding that.
	*/
	Send(ctx context.Context, conn *models.ChannelConnection, msg OutboundMessage) (SendResult, error)
}

/*
ChannelCapabilities is what a channel can do on a connection.

Transport says how outbound messages travel and Start whether the
business may open a thread. MaxBodyChars bounds a formatted body, zero
meaning no limit. The attachment limits use the global ones when zero.
RetryWindow is how long a transient failure keeps being retried, the
inbox default when zero.
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
Transport is how outbound messages of a channel travel.
*/
type Transport string

/*
The transports: pushed to a provider by a job, read live by the
customer, or not sent anywhere.
*/
const (
	TransportPush     Transport = "push"
	TransportRealtime Transport = "realtime"
	TransportNone     Transport = "none"
)

/*
StartMode is whether the business may open a thread on the channel.
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
OutboundMessage is one message to deliver.

IdempotencyKey is stable across retries of the same message.
ExternalThreadKey is empty until the conversation has a thread on the
channel. The body comes as Markdown, plain text and safe HTML. ReplyTo
lists the channel ids of earlier messages of the thread, newest last.
Options is the raw channel options of the request, nil when absent.
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
Recipient is who an outbound message goes to, by the identity the
channel reaches them with.
*/
type Recipient struct {
	ContactID     ids.UUID
	Name          string
	IdentityType  enums.IdentityType
	IdentityValue string
	Locale        string
}

/*
OutboundAuthor is who wrote an outbound message, as the customer sees
them.
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
