/*
|--------------------------------------------------------------------------
| Slack Channel
|--------------------------------------------------------------------------
|
| Replies reach Slack as messages in the conversation's own thread, or
| in the direct message it lives in. A teammate's reply shows their name
| and picture; posted with their own Slack token when they connected
| their account and are in the channel, it shows as them. Slack never
| lets the team start a conversation, so a reply always continues one.
|
| Slack has no idempotency key, so a retried send first looks for the
| parts it already posted and reports them instead of posting again.
| Every part's id is kept, so an edit, a deletion or a click on any part
| finds the message. Posting is paced per channel, about one message a
| second as Slack asks, across every replica.
|
*/

package slack

import (
	"context"
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Pacing of posts into one channel: a burst of three, then one a second.
*/
const (
	postBurst  = 3
	postWindow = 3 * time.Second
	maxFiles   = 10
	maxFileMB  = 25
	nameLimit  = 80
)

/*
Credentials hands the driver the tokens of a connection.
*/
type Credentials interface {
	/*
		BotToken returns the connection's bot token.
	*/
	BotToken(ctx context.Context, conn models.ChannelConnection) (string, error)

	/*
		UserToken returns the teammate's own Slack token on the
		connection, reporting false when they never connected their
		account.
	*/
	UserToken(ctx context.Context, conn models.ChannelConnection, memberID ids.UUID) (string, bool, error)
}

/*
Part is one provider message a reply became: its id, its place among
the reply's parts and whether it is text or a file.
*/
type Part struct {
	ExternalID string
	Index      int
	Kind       string
}

/*
Places tells the driver which Slack channel a room is and remembers the
parts each reply became.
*/
type Places interface {
	/*
		RoomChannel returns the Slack channel id of the room.
	*/
	RoomChannel(ctx context.Context, conn models.ChannelConnection, roomID ids.UUID) (string, error)

	/*
		SentParts returns the parts already posted for the message.
	*/
	SentParts(ctx context.Context, conn models.ChannelConnection, messageID ids.UUID) ([]Part, error)

	/*
		RecordParts remembers the parts posted for the message.
	*/
	RecordParts(ctx context.Context, conn models.ChannelConnection, messageID ids.UUID, parts []Part) error
}

/*
Markers names the reactions the driver shows state with: the closed
mark on a thread and the working sign on a message.
*/
type Markers struct {
	Closed  string
	Working string
}

/*
Driver is the Slack channel driver. HTTP carries every request to the
Web API at APIURL; Limiter paces posts across replicas.
*/
type Driver struct {
	HTTP        *httpclient.Client
	APIURL      string
	Credentials Credentials
	Places      Places
	Limiter     *ratelimit.Limiter
	Markers     Markers
}

/*
Type names the slack connection type.
*/
func (d *Driver) Type() string {
	return Type
}

/*
Capabilities says replies are pushed into the conversation's thread,
in Markdown, with files, and that the team never starts a thread.
*/
func (d *Driver) Capabilities(models.ChannelConnection) contracts.ChannelCapabilities {
	return contracts.ChannelCapabilities{
		Transport: contracts.TransportPush, StartConversation: contracts.StartNever,
		Formatting: contracts.FormatMarkdown, MaxBodyChars: MaxBodyChars, Attachments: true,
		MaxAttachments: maxFiles, MaxAttachmentBytes: maxFileMB << 20, ThreadAddressed: true,
	}
}

/*
API returns the Web API for the token.
*/
func (d *Driver) API(token string) *API {
	return NewAPI(d.HTTP, d.APIURL, token)
}

/*
target is where a reply goes: the channel and the thread in it, empty
for a direct message.
*/
type target struct {
	channel  string
	threadTS string
}

/*
Send posts the reply into its thread, then its files, and remembers
every part.
*/
func (d *Driver) Send(ctx context.Context, conn models.ChannelConnection, msg contracts.OutboundMessage) (
	contracts.SendResult, error,
) {
	sent, err := d.Places.SentParts(ctx, conn, msg.MessageID)
	if err != nil || len(sent) > 0 {
		return resultOf(sent), err
	}

	to, err := d.target(ctx, conn, msg.ExternalThreadKey)
	if err != nil {
		return contracts.SendResult{}, err
	}

	if err := d.pace(ctx, conn, to.channel); err != nil {
		return contracts.SendResult{}, err
	}

	token, err := d.Credentials.BotToken(ctx, conn)
	if err != nil {
		return contracts.SendResult{}, err
	}

	parts, err := d.post(ctx, sending{conn: conn, token: token, to: to, msg: msg})
	if err != nil {
		return contracts.SendResult{}, err
	}

	if err := d.Places.RecordParts(ctx, conn, msg.MessageID, parts); err != nil {
		return contracts.SendResult{}, err
	}

	return resultOf(parts), nil
}

/*
sending is one reply on its way: the connection, the bot token, where
it goes and the reply.
*/
type sending struct {
	conn  models.ChannelConnection
	token string
	to    target
	msg   contracts.OutboundMessage
}

/*
post posts the text, then shares the files into the same thread.
*/
func (d *Driver) post(ctx context.Context, s sending) ([]Part, error) {
	var parts []Part

	if s.msg.BodyMarkdown != "" || len(s.msg.Attachments) == 0 {
		ts, err := d.postText(ctx, s)
		if err != nil {
			return nil, err
		}

		parts = append(parts, Part{ExternalID: MessageID(s.to.channel, ts), Kind: "text"})
	}

	files, err := d.API(s.token).upload(ctx, s.to, s.msg.Attachments)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		parts = append(parts, Part{ExternalID: FileID(file), Index: len(parts), Kind: "file"})
	}

	return parts, nil
}

/*
postText posts the reply's text: as the teammate when they connected
their own Slack and may post there, else as the bot showing their name
and picture.
*/
func (d *Driver) postText(ctx context.Context, s sending) (string, error) {
	opts := messageOptions(s.to, s.msg)

	if user, ok := d.userToken(ctx, s); ok {
		ts, err := d.API(user).postMessage(ctx, s.to.channel, opts...)
		if err == nil {
			return ts, nil
		}
	}

	opts = append(opts, authorOptions(s.msg.Author)...)

	return d.API(s.token).postMessage(ctx, s.to.channel, opts...)
}

/*
userToken returns the teammate's own token for a reply in a room. A
direct message with the bot always comes from the bot.
*/
func (d *Driver) userToken(ctx context.Context, s sending) (string, bool) {
	if s.to.threadTS == "" || s.msg.Author.Type != enums.AuthorTypeMember {
		return "", false
	}

	token, ok, err := d.Credentials.UserToken(ctx, s.conn, s.msg.Author.ID)

	return token, ok && err == nil
}

/*
postMessage posts one message and returns its timestamp.
*/
func (a *API) postMessage(ctx context.Context, channel string, opts ...slack.MsgOption) (string, error) {
	_, ts, err := a.client.PostMessageContext(ctx, channel, opts...)
	if err != nil {
		return "", MapError("chat.postMessage", err)
	}

	return ts, nil
}

/*
messageOptions shapes the reply: its Markdown, as blocks with buttons
when it offers answer controls or asks a direct message's reader for
feedback on the agent's answer, in its thread, without link previews.
*/
func messageOptions(to target, msg contracts.OutboundMessage) []slack.MsgOption {
	body := Truncate(ToSlack(msg.BodyMarkdown), MaxBodyChars)
	opts := []slack.MsgOption{slack.MsgOptionDisableLinkUnfurl()}

	if to.threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(to.threadTS))
	}

	feedback := to.threadTS == "" && msg.Author.Type == enums.AuthorTypeAgent && msg.IdempotencyKey != ""
	if len(msg.Controls) == 0 && !feedback {
		return append(opts, slack.MsgOptionMarkdownText(body))
	}

	blocks := controlBlocks(body, msg.IdempotencyKey, msg.Controls)
	if feedback {
		blocks = append(blocks, feedbackBlock(msg.IdempotencyKey))
	}

	return append(opts, slack.MsgOptionText(Truncate(msg.BodyText, MaxBodyChars), false),
		slack.MsgOptionBlocks(blocks...))
}

/*
authorOptions shows the reply's writer by name and picture.
*/
func authorOptions(author contracts.OutboundAuthor) []slack.MsgOption {
	var opts []slack.MsgOption

	if author.Name != "" {
		opts = append(opts, slack.MsgOptionUsername(Truncate(author.Name, nameLimit)))
	}

	if author.AvatarURL != "" {
		opts = append(opts, slack.MsgOptionIconURL(author.AvatarURL))
	}

	return opts
}

/*
target finds where the conversation's thread is on Slack.
*/
func (d *Driver) target(ctx context.Context, conn models.ChannelConnection, key string) (target, error) {
	thread, ok := ParseThreadKey(key)
	if !ok {
		return target{}, &contracts.SendError{
			Code: CodeThreadUnknown, Message: "The conversation has no Slack thread.", Err: contracts.ErrPermanent,
		}
	}

	if thread.Direct() {
		return target{channel: thread.Channel}, nil
	}

	channel, err := d.Places.RoomChannel(ctx, conn, thread.RoomID)
	if err != nil {
		return target{}, err
	}

	return target{channel: channel, threadTS: thread.RootTS}, nil
}

/*
pace waits its turn to post into the channel, postponing the send when
the channel's posts are used up.
*/
func (d *Driver) pace(ctx context.Context, conn models.ChannelConnection, channel string) error {
	if d.Limiter == nil {
		return nil
	}

	result, err := d.Limiter.Attempt(ctx, "slack:post:"+conn.ID.String()+":"+channel, postBurst, postWindow)
	if err != nil {
		return fmt.Errorf("pace slack post: %w", err)
	}

	if !result.Allowed {
		return &contracts.RateLimitedError{RetryAfter: max(result.RetryAfter, time.Second)}
	}

	return nil
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
	for _, part := range parts {
		external = append(external, part.ExternalID)
	}

	return contracts.SendResult{
		ChannelMessageID: parts[0].ExternalID, Status: enums.DeliveryStatusSent,
		Metadata: map[string]any{"parts": external},
	}
}
