/*
|--------------------------------------------------------------------------
| Social Driver
|--------------------------------------------------------------------------
|
| One driver serves Messenger and Instagram, once for each platform. A
| reply reaches the customer only after they wrote: Meta lets anyone on
| the team, and the AI agent, answer for 24 hours after the customer's
| last message, then only a person for the rest of a week, and nobody
| after that until the customer writes again. A reply goes out as its
| text, cut where the platform's limit asks, then each file on its own;
| a try that failed halfway sends only what is still missing.
|
*/

package social

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Credentials hands the driver the token a connection calls Meta with: a
Page's own token on Messenger, the business's on Instagram.
*/
type Credentials interface {
	/*
		Token returns the token the connection calls Meta with.
	*/
	Token(ctx context.Context, conn models.ChannelConnection) (string, error)
}

/*
Part is one message on the platform a reply became: Meta's id, its
place among the reply's parts and its kind, text or file.
*/
type Part struct {
	ExternalID string
	Index      int
	Kind       string
}

/*
Window is when the customer last wrote on a connection, zero when they
never did, and their newest message.
*/
type Window struct {
	LastInboundAt time.Time
	LastInboundID string
}

/*
Places tells the driver what it needs from the inbox: the parts a reply
already became and when the customer last wrote.
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
		Window returns when the customer last wrote on the connection.
	*/
	Window(ctx context.Context, conn models.ChannelConnection, contactID ids.UUID) (Window, error)
}

/*
Facts are what a connection knows about its Page or Instagram account.
AccountID is the Page's or the Instagram account's id, BusinessID the
business portfolio that granted access, and AppID the Meta app the
connection was made with.
*/
type Facts struct {
	AccountID  string `json:"account_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Username   string `json:"username,omitempty"`
	PictureURL string `json:"picture_url,omitempty"`
	BusinessID string `json:"business_id,omitempty"`
	AppID      string `json:"app_id,omitempty"`
}

/*
Driver is the channel driver of one platform. HTTP carries every call to
the Graph API at GraphURL and Version. Standard is how long after the
customer's last message anyone may reply, and Human how long a teammate
still may.
*/
type Driver struct {
	Platform    Platform
	HTTP        *httpclient.Client
	GraphURL    string
	Version     string
	Credentials Credentials
	Places      Places
	Clock       clock.Clock
	FactsOf     func(models.ChannelConnection) Facts
	Standard    time.Duration
	Human       time.Duration
}

/*
Type names the connection type, the platform's own name.
*/
func (d *Driver) Type() string {
	return string(d.Platform)
}

/*
Capabilities says replies are pushed to the customer's thread as plain
text with files, that the team can never write first, and that a reply
may go for as long as a teammate may still write. The shorter time the
AI agent has is checked when a reply is sent.
*/
func (d *Driver) Capabilities(models.ChannelConnection) contracts.ChannelCapabilities {
	limit := facebookTextChars
	if d.Platform == Instagram {
		limit = instagramTextBytes
	}

	return contracts.ChannelCapabilities{
		Transport: contracts.TransportPush, StartConversation: contracts.StartNever,
		Formatting: contracts.FormatPlain, MaxBodyChars: limit, Attachments: true, MaxAttachments: maxFiles,
		MaxAttachmentBytes: maxFileBytes, Typing: true, ReadReceipts: true, MessagingWindow: d.Human,
		ThreadAddressed: true,
	}
}

/*
Graph returns the Graph API for the token.
*/
func (d *Driver) Graph(token string) *meta.Graph {
	return meta.NewGraph(d.HTTP, d.GraphURL, d.Version, token)
}

/*
Phase is what Meta allows on a thread at a moment.
*/
type Phase string

/*
The phases: anyone may reply, only a teammate may, or nobody may.
*/
const (
	PhaseOpen   Phase = "open"
	PhaseHuman  Phase = "human"
	PhaseClosed Phase = "closed"
)

/*
PhaseAt returns what Meta allows on the thread at the time.
*/
func (d *Driver) PhaseAt(w Window, now time.Time) Phase {
	switch {
	case w.LastInboundAt.IsZero() || !now.Before(w.LastInboundAt.Add(d.Human)):
		return PhaseClosed
	case now.Before(w.LastInboundAt.Add(d.Standard)):
		return PhaseOpen
	default:
		return PhaseHuman
	}
}

/*
sending is one reply on its way: the account it leaves from, the Graph
API with its token, who it goes to, how Meta is told it may go, and the
reply.
*/
type sending struct {
	conn    models.ChannelConnection
	account string
	graph   *meta.Graph
	to      Target
	tagged  bool
	msg     contracts.OutboundMessage
}

/*
Send delivers the reply: its text and then its files, sending only the
parts a try before did not.
*/
func (d *Driver) Send(ctx context.Context, conn models.ChannelConnection, msg contracts.OutboundMessage) (
	contracts.SendResult, error,
) {
	sent, err := d.Places.SentParts(ctx, conn, msg.MessageID)
	if err != nil {
		return contracts.SendResult{}, err
	}

	pieces, err := d.piecesOf(msg)
	if err != nil || len(sent) >= len(pieces) {
		return resultOf(sent), err
	}

	s, err := d.prepare(ctx, conn, msg)
	if err != nil {
		return contracts.SendResult{}, err
	}

	if len(sent) == 0 {
		_ = d.act(ctx, s, "mark_seen")
	}

	parts, err := d.post(ctx, s, pieces, sent)

	return resultOf(parts), err
}

/*
prepare finds the account, who the reply goes to, whether Meta still
lets it go, and the token.
*/
func (d *Driver) prepare(ctx context.Context, conn models.ChannelConnection, msg contracts.OutboundMessage) (
	sending, error,
) {
	facts := d.FactsOf(conn)
	if facts.AccountID == "" {
		return sending{}, &contracts.SendError{
			Code:    CodeConnectionBroken,
			Message: "The " + d.Platform.Name() + " connection has no account. Reconnect it.",
			Err:     contracts.ErrConnectionBroken,
		}
	}

	to, ok := ParseThreadKey(msg.ExternalThreadKey)
	if !ok && msg.Recipient.IdentityType == d.Platform.Identity() && msg.Recipient.IdentityValue != "" {
		to, ok = Target{Customer: msg.Recipient.IdentityValue}, true
	}

	if !ok {
		return sending{}, refuse(CodeThreadUnknown, "The customer has not written on "+d.Platform.Name()+" yet.")
	}

	tagged, err := d.allowed(ctx, conn, msg)
	if err != nil {
		return sending{}, err
	}

	token, err := d.Credentials.Token(ctx, conn)
	if err != nil {
		return sending{}, err
	}

	return sending{conn: conn, account: facts.AccountID, graph: d.Graph(token), to: to, tagged: tagged, msg: msg}, nil
}

/*
allowed reports whether the reply may go now and whether it needs
Meta's human-agent tag, which only a teammate's reply may carry.
*/
func (d *Driver) allowed(ctx context.Context, conn models.ChannelConnection, msg contracts.OutboundMessage) (
	bool, error,
) {
	window, err := d.Places.Window(ctx, conn, msg.Recipient.ContactID)
	if err != nil {
		return false, err
	}

	switch d.PhaseAt(window, d.Clock.Now()) {
	case PhaseOpen:
		return false, nil
	case PhaseHuman:
		if msg.Author.Type == enums.AuthorTypeMember {
			return true, nil
		}

		return false, &contracts.SendError{
			Code: CodeHumanOnly, Message: humanOnlyMessage, Err: contracts.ErrWindowClosed,
		}
	case PhaseClosed:
		return false, &contracts.SendError{
			Code: CodeWindowClosed, Message: windowClosedMessage, Err: contracts.ErrWindowClosed,
		}
	default:
		return false, &contracts.SendError{
			Code: CodeWindowClosed, Message: windowClosedMessage, Err: contracts.ErrWindowClosed,
		}
	}
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
