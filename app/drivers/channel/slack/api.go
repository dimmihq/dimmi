/*
|--------------------------------------------------------------------------
| Slack Web API
|--------------------------------------------------------------------------
|
| The calls the product makes to Slack, through the Slack SDK on the
| install's HTTP client, so every request passes the same address checks
| and tests answer them from recorded fixtures. Results come back in
| plain shapes and errors as the channel contract's, so nothing outside
| this package names an SDK type. A reaction already there, or already
| gone, is no error: the state the caller wanted holds.
|
*/

package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/slack-go/slack"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
fileHosts are the only hosts a Slack file is downloaded from, so a
crafted file address in a delivery can never make the server fetch
anything else.
*/
func fileHosts() []string {
	return []string{"files.slack.com", "slack-files.com"}
}

/*
API calls Slack's Web API with one token.
*/
type API struct {
	client *slack.Client
	http   *httpclient.Client
	base   string
	token  string
}

/*
NewAPI returns the Web API for the token, over the HTTP client, at the
base URL.
*/
func NewAPI(http *httpclient.Client, base, token string) *API {
	return &API{
		client: slack.New(token, slack.OptionHTTPClient(http.StdClient()), slack.OptionAPIURL(base)),
		http:   http, base: base, token: token,
	}
}

/*
Identity is who a token belongs to: the team, the bot's user, the bot
and its app, and the scopes it was granted.
*/
type Identity struct {
	TeamID   string
	TeamName string
	UserID   string
	BotID    string
	AppID    string
	Scopes   []string
}

/*
AuthTest checks the token and returns who it belongs to, with the
scopes Slack lists for it.
*/
func (a *API) AuthTest(ctx context.Context) (Identity, error) {
	resp, err := a.http.New().WithToken(a.token).Post(ctx, a.base+"auth.test", nil)
	if err != nil {
		return Identity{}, fmt.Errorf("slack auth.test: %w", err)
	}

	var body struct {
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
		TeamID string `json:"team_id"`
		Team   string `json:"team"`
		UserID string `json:"user_id"`
		BotID  string `json:"bot_id"`
	}

	if err := resp.JSON(&body); err != nil {
		return Identity{}, err
	}

	if !body.OK {
		return Identity{}, MapError("auth.test", slack.SlackErrorResponse{Err: body.Error})
	}

	return Identity{
		TeamID: body.TeamID, TeamName: body.Team, UserID: body.UserID, BotID: body.BotID,
		Scopes: splitScopes(resp.Header.Get("X-OAuth-Scopes")),
	}, nil
}

/*
splitScopes reads Slack's comma separated scope list.
*/
func splitScopes(header string) []string {
	var out []string

	for scope := range strings.SplitSeq(header, ",") {
		if scope = strings.TrimSpace(scope); scope != "" {
			out = append(out, scope)
		}
	}

	return out
}

/*
BotAppID returns the app a bot belongs to.
*/
func (a *API) BotAppID(ctx context.Context, botID string) (string, error) {
	bot, err := a.client.GetBotInfoContext(ctx, slack.GetBotInfoParameters{Bot: botID})
	if err != nil {
		return "", MapError("bots.info", err)
	}

	return bot.AppID, nil
}

/*
User is a Slack user as the product needs them.
*/
type User struct {
	ID                string
	TeamID            string
	Name              string
	Email             string
	AvatarURL         string
	Timezone          string
	IsBot             bool
	IsRestricted      bool
	IsUltraRestricted bool
	Deleted           bool
}

/*
Guest reports whether the user is a guest of their workspace.
*/
func (u User) Guest() bool {
	return u.IsRestricted || u.IsUltraRestricted
}

/*
User returns the user with the id.
*/
func (a *API) User(ctx context.Context, userID string) (User, error) {
	u, err := a.client.GetUserInfoContext(ctx, userID)
	if err != nil {
		return User{}, MapError("users.info", err)
	}

	name := u.Profile.DisplayName
	if name == "" {
		name = u.RealName
	}

	if name == "" {
		name = u.Name
	}

	return User{
		ID: u.ID, TeamID: u.TeamID, Name: name, Email: strings.ToLower(u.Profile.Email),
		AvatarURL: u.Profile.Image192, Timezone: u.TZ, IsBot: u.IsBot || u.IsAppUser,
		IsRestricted: u.IsRestricted, IsUltraRestricted: u.IsUltraRestricted, Deleted: u.Deleted,
	}, nil
}

/*
Channel is a Slack channel as the product needs it.
*/
type Channel struct {
	ID          string
	Name        string
	IsPrivate   bool
	IsArchived  bool
	IsExtShared bool
	IsIM        bool
}

/*
channelOf reads the SDK's channel.
*/
func channelOf(c slack.Channel) Channel {
	return Channel{
		ID: c.ID, Name: c.Name, IsPrivate: c.IsPrivate, IsArchived: c.IsArchived, IsExtShared: c.IsExtShared,
		IsIM: c.IsIM,
	}
}

/*
Channel returns the channel with the id.
*/
func (a *API) Channel(ctx context.Context, channelID string) (Channel, error) {
	c, err := a.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{ChannelID: channelID})
	if err != nil {
		return Channel{}, MapError("conversations.info", err)
	}

	return channelOf(*c), nil
}

/*
maxChannelPages bounds how many pages of channels one listing reads.
*/
const maxChannelPages = 50

/*
MemberChannels returns the public and private channels the bot user is
in, archived ones left out.
*/
func (a *API) MemberChannels(ctx context.Context, botUserID string) ([]Channel, error) {
	var out []Channel

	cursor := ""

	for range maxChannelPages {
		page, next, err := a.client.GetConversationsForUserContext(ctx, &slack.GetConversationsForUserParameters{
			UserID: botUserID, Cursor: cursor, Types: []string{"public_channel", "private_channel"},
			Limit: 200, ExcludeArchived: true,
		})
		if err != nil {
			return out, MapError("users.conversations", err)
		}

		for _, c := range page {
			out = append(out, channelOf(c))
		}

		if next == "" {
			return out, nil
		}

		cursor = next
	}

	return out, nil
}

/*
Message returns the top-level message of the channel at the timestamp,
reporting false when there is none.
*/
func (a *API) Message(ctx context.Context, channelID, ts string) (Event, bool, error) {
	resp, err := a.client.GetConversationHistoryContext(ctx, &slack.GetConversationHistoryParameters{
		ChannelID: channelID, Latest: ts, Inclusive: true, Limit: 1,
	})
	if err != nil {
		return Event{}, false, MapError("conversations.history", err)
	}

	for _, msg := range resp.Messages {
		if msg.Timestamp != ts {
			continue
		}

		ev, err := eventOf(msg)

		return ev, err == nil, err
	}

	return Event{}, false, nil
}

/*
eventOf reads an SDK message as an event, through its wire shape.
*/
func eventOf(msg slack.Message) (Event, error) {
	raw, err := json.Marshal(msg.Msg)
	if err != nil {
		return Event{}, fmt.Errorf("encode slack message: %w", err)
	}

	ev, err := ParseEvent(raw)
	ev.Type = "message"

	return ev, err
}

/*
Notice is a message only one user sees: the channel, the user, the
thread when it belongs in one, and its text.
*/
type Notice struct {
	Channel  string
	User     string
	ThreadTS string
	Text     string
}

/*
Ephemeral shows the notice to its user alone.
*/
func (a *API) Ephemeral(ctx context.Context, n Notice) error {
	opts := []slack.MsgOption{slack.MsgOptionText(n.Text, false)}
	if n.ThreadTS != "" {
		opts = append(opts, slack.MsgOptionTS(n.ThreadTS))
	}

	_, err := a.client.PostEphemeralContext(ctx, n.Channel, n.User, opts...)

	return MapError("chat.postEphemeral", err)
}

/*
React adds the reaction to the message; one already there is no error.
*/
func (a *API) React(ctx context.Context, channelID, ts, name string) error {
	err := MapError("reactions.add", a.client.AddReactionContext(ctx, name, slack.NewRefToMessage(channelID, ts)))
	if Code(err) == CodeAlreadyReacted {
		return nil
	}

	return err
}

/*
Unreact takes the reaction off the message; one already gone is no
error.
*/
func (a *API) Unreact(ctx context.Context, channelID, ts, name string) error {
	err := MapError("reactions.remove", a.client.RemoveReactionContext(ctx, name, slack.NewRefToMessage(channelID, ts)))
	if Code(err) == CodeNoReaction {
		return nil
	}

	return err
}

/*
Delete deletes a message the token posted; one already gone is no
error.
*/
func (a *API) Delete(ctx context.Context, channelID, ts string) error {
	_, _, err := a.client.DeleteMessageContext(ctx, channelID, ts)

	err = MapError("chat.delete", err)
	if Code(err) == CodeMessageNotFound {
		return nil
	}

	return err
}

/*
Update replaces a message's text, dropping its buttons.
*/
func (a *API) Update(ctx context.Context, channelID, ts, markdown string) error {
	return a.updateBlocks(ctx, channelID, ts, rendered{
		text: markdown, blocks: []slack.Block{slack.NewMarkdownBlock("", Truncate(markdown, MaxBodyChars))},
	})
}

/*
rendered is a message laid out as blocks, with the text Slack shows
where blocks cannot be.
*/
type rendered struct {
	text   string
	blocks []slack.Block
}

/*
updateBlocks replaces a message with the rendered blocks.
*/
func (a *API) updateBlocks(ctx context.Context, channelID, ts string, r rendered) error {
	_, _, _, err := a.client.UpdateMessageContext(ctx, channelID, ts,
		slack.MsgOptionBlocks(r.blocks...), slack.MsgOptionText(Truncate(r.text, MaxBodyChars), false))

	return MapError("chat.update", err)
}

/*
Errors downloading a file. ErrFileHost refuses an address on a host
Slack never serves files from; ErrFileUnavailable means Slack would not
hand the file over.
*/
var (
	ErrFileHost        = errors.New("slack: file address outside Slack's file hosts")
	ErrFileUnavailable = errors.New("slack: file unavailable")
)

/*
OpenFile streams a file shared in Slack, at most limit bytes of it.
The caller closes what it returns.
*/
func (a *API) OpenFile(ctx context.Context, address string, limit int64) (io.ReadCloser, error) {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || !allowedHost(u.Hostname()) {
		return nil, ErrFileHost
	}

	resp, err := a.http.New().WithToken(a.token).MaxBody(limit).Get(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("download slack file: %w", err)
	}

	if resp.Status != fhttp.StatusOK {
		return nil, fmt.Errorf("download slack file: %w: status %d", ErrFileUnavailable, resp.Status)
	}

	return io.NopCloser(bytes.NewReader(resp.Body)), nil
}

/*
allowedHost reports whether files are downloaded from the host.
*/
func allowedHost(host string) bool {
	for _, allowed := range fileHosts() {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}

	return false
}
