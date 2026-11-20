/*
|--------------------------------------------------------------------------
| WhatsApp Channel
|--------------------------------------------------------------------------
|
| Replies reach WhatsApp through Meta's Cloud API, from the connection's
| business number to the customer's thread: by their phone's WhatsApp id
| when known, else by their business-scoped user id. Free text goes out
| only within 24 hours of the customer's last message; after that only a
| template Meta approved does, and a reply that is not one fails with
| the reason, never silently turned into something else.
|
| Text is WhatsApp's own markup, cut into parts when longer than one
| message holds; each file is a message of its own after the text. Meta
| takes no idempotency key, so a retried send first looks for the parts
| it already sent and sends only what is missing.
|
*/

package whatsapp

import (
	"context"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of WhatsApp's messages and files.
*/
const (
	maxFiles      = 10
	maxImageBytes = 5 << 20
	maxVideoBytes = 16 << 20
	maxDocBytes   = 100 << 20
)

/*
Credentials hands the driver a connection's token and its facts.
*/
type Credentials interface {
	/*
		Token returns the token the connection calls Meta with.
	*/
	Token(ctx context.Context, conn models.ChannelConnection) (string, error)
}

/*
Part is one WhatsApp message a reply became: Meta's id, its place
among the reply's parts and its kind.
*/
type Part struct {
	ExternalID string
	Index      int
	Kind       string
}

/*
Window is a customer's state on a connection: until when free text may
go out, zero when they never wrote, and their newest message.
*/
type Window struct {
	ExpiresAt     time.Time
	LastInboundID string
}

/*
Open reports whether free text may go out at the time.
*/
func (w Window) Open(now time.Time) bool {
	return !w.ExpiresAt.IsZero() && now.Before(w.ExpiresAt)
}

/*
Places tells the driver what it needs from the inbox: the parts a reply
already became, the customer's window, and the connection's templates.
*/
type Places interface {
	/*
		SentParts returns the parts already sent for the message.
	*/
	SentParts(ctx context.Context, conn models.ChannelConnection, messageID ids.UUID) ([]Part, error)

	/*
		RecordParts remembers the parts sent for the message.
	*/
	RecordParts(ctx context.Context, conn models.ChannelConnection, messageID ids.UUID, parts []Part) error

	/*
		Window returns the customer's window on the connection.
	*/
	Window(ctx context.Context, conn models.ChannelConnection, contactID ids.UUID) (Window, error)

	/*
		Phone returns the customer's WhatsApp phone id, its digits, empty
		when WhatsApp never shared it.
	*/
	Phone(ctx context.Context, conn models.ChannelConnection, contactID ids.UUID) (string, error)

	Templates
}

/*
Templates are a connection's templates as dimmi copied them from Meta.
*/
type Templates interface {
	/*
		Template returns the connection's template with the name and
		language, as Meta last described it.
	*/
	Template(ctx context.Context, conn models.ChannelConnection, name, language string) (Template, bool, error)

	/*
		RefreshTemplates copies the connection's templates from Meta again,
		after Meta refused one dimmi thought it could send.
	*/
	RefreshTemplates(ctx context.Context, conn models.ChannelConnection) error
}

/*
Facts are what a WhatsApp connection knows about its number.
*/
type Facts struct {
	Mode               string `json:"mode,omitempty"`
	AccountID          string `json:"waba_id,omitempty"`
	PhoneNumberID      string `json:"phone_number_id,omitempty"`
	BusinessID         string `json:"business_id,omitempty"`
	DisplayPhoneNumber string `json:"display_phone_number,omitempty"`
	VerifiedName       string `json:"verified_name,omitempty"`
	NameStatus         string `json:"name_status,omitempty"`
	QualityRating      string `json:"quality_rating,omitempty"`
	MessagingLimit     string `json:"messaging_limit,omitempty"`
	AppID              string `json:"app_id,omitempty"`
	Coexistence        bool   `json:"coexistence,omitempty"`
}

/*
Driver is the WhatsApp channel driver. HTTP carries every call to the
Graph API at GraphURL and Version; Clock tells it the time.
*/
type Driver struct {
	HTTP        *httpclient.Client
	GraphURL    string
	Version     string
	Credentials Credentials
	Places      Places
	Clock       clock.Clock
	FactsOf     func(models.ChannelConnection) Facts
}

/*
Type names the whatsapp connection type.
*/
func (d *Driver) Type() string {
	return Type
}

/*
Capabilities says replies are pushed to the customer's thread as plain
WhatsApp markup with files, within 24 hours of their last message, and
that the team opens a thread only with a template.
*/
func (d *Driver) Capabilities(models.ChannelConnection) contracts.ChannelCapabilities {
	return contracts.ChannelCapabilities{
		Transport: contracts.TransportPush, StartConversation: contracts.StartWithTemplate,
		Formatting: contracts.FormatPlain, MaxBodyChars: MaxBodyChars, Attachments: true,
		MaxAttachments: maxFiles, MaxAttachmentBytes: maxDocBytes, Typing: true, DeliveryReceipts: true,
		ReadReceipts: true, MessagingWindow: 24 * time.Hour, ThreadAddressed: true,
	}
}

/*
Graph returns the Graph API for the token.
*/
func (d *Driver) Graph(token string) *Graph {
	return NewGraph(d.HTTP, d.GraphURL, d.Version, token)
}

/*
sending is one reply on its way: the connection, its number, the token,
who it goes to and the reply.
*/
type sending struct {
	conn  models.ChannelConnection
	phone string
	graph *Graph
	to    Recipient
	msg   contracts.OutboundMessage
}

/*
Send delivers the reply: its template, or its text and then its files,
sending only the parts a try before did not.
*/
func (d *Driver) Send(ctx context.Context, conn models.ChannelConnection, msg contracts.OutboundMessage) (
	contracts.SendResult, error,
) {
	opts, err := ParseOptions(msg.Options)
	if err != nil {
		return contracts.SendResult{}, refuse(CodeRefused, "The reply's WhatsApp options are not readable.")
	}

	sent, err := d.Places.SentParts(ctx, conn, msg.MessageID)
	if err != nil || complete(sent, msg, opts) {
		return resultOf(sent), err
	}

	s, err := d.prepare(ctx, conn, msg)
	if err != nil {
		return contracts.SendResult{}, err
	}

	if opts.Template != nil {
		return d.sendTemplate(ctx, s, *opts.Template)
	}

	window, err := d.Places.Window(ctx, conn, msg.Recipient.ContactID)
	if err != nil {
		return contracts.SendResult{}, err
	}

	if !window.Open(d.Clock.Now()) {
		return contracts.SendResult{}, &contracts.SendError{
			Code:    CodeWindowClosed,
			Message: windowClosedMessage,
			Err:     contracts.ErrWindowClosed,
		}
	}

	if len(sent) == 0 && window.LastInboundID != "" {
		d.markReadBeforeReply(ctx, s, window.LastInboundID)
	}

	parts, err := d.post(ctx, s, sent)

	return resultOf(parts), err
}

/*
prepare finds the number, the token and who the reply goes to.
*/
func (d *Driver) prepare(ctx context.Context, conn models.ChannelConnection, msg contracts.OutboundMessage) (
	sending, error,
) {
	facts := d.FactsOf(conn)
	if facts.PhoneNumberID == "" {
		return sending{}, &contracts.SendError{
			Code: CodeConnectionBroken, Message: "The WhatsApp connection has no number. Reconnect it.",
			Err: contracts.ErrConnectionBroken,
		}
	}

	to, ok := ParseThreadKey(msg.ExternalThreadKey)
	if !ok {
		to, ok = recipientOf(msg.Recipient)
	}

	if to.WaID == "" {
		phone, err := d.Places.Phone(ctx, conn, msg.Recipient.ContactID)
		if err != nil {
			return sending{}, err
		}

		to.WaID, ok = phone, ok || phone != ""
	}

	if !ok {
		return sending{}, refuse(CodeThreadUnknown, "The customer has no WhatsApp number on this conversation.")
	}

	token, err := d.Credentials.Token(ctx, conn)
	if err != nil {
		return sending{}, err
	}

	return sending{conn: conn, phone: facts.PhoneNumberID, graph: d.Graph(token), to: to, msg: msg}, nil
}

/*
recipientOf reads who a reply goes to from the customer's identity
when the conversation has no WhatsApp thread yet.
*/
func recipientOf(r contracts.Recipient) (Recipient, bool) {
	switch r.IdentityType {
	case enums.IdentityTypeWhatsAppUser:
		return Recipient{UserID: r.IdentityValue}, r.IdentityValue != ""
	case enums.IdentityTypePhone:
		digits := strings.TrimPrefix(r.IdentityValue, "+")

		return Recipient{WaID: digits}, digits != ""
	case enums.IdentityTypeEmail, enums.IdentityTypeWidgetVisitor, enums.IdentityTypeExternalID,
		enums.IdentityTypeSlackUser:
		return Recipient{}, false
	default:
		return Recipient{}, false
	}
}

/*
complete reports whether the parts already sent are the whole reply.
*/
func complete(sent []Part, msg contracts.OutboundMessage, opts Options) bool {
	if len(sent) == 0 {
		return false
	}

	if opts.Template != nil {
		return true
	}

	files := 0
	for _, p := range sent {
		if p.Kind == "file" {
			files++
		}
	}

	return files == len(msg.Attachments) && (msg.BodyMarkdown == "" || len(sent) > files)
}

/*
refuse is a permanent failure with the code and words.
*/
func refuse(code, message string) error {
	return &contracts.SendError{Code: code, Message: message, Err: contracts.ErrPermanent}
}

/*
resultOf reports the parts as the send's result, the first part being
the message's own id.
*/
func resultOf(parts []Part) contracts.SendResult {
	if len(parts) == 0 {
		return contracts.SendResult{}
	}

	external := make([]string, 0, len(parts))
	for _, p := range parts {
		external = append(external, p.ExternalID)
	}

	return contracts.SendResult{
		ChannelMessageID: parts[0].ExternalID, Status: enums.DeliveryStatusSent,
		Metadata: map[string]any{"parts": external},
	}
}
