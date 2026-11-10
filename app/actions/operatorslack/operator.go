/*
|--------------------------------------------------------------------------
| Operator In Slack
|--------------------------------------------------------------------------
|
| A teammate asks Operator from Slack by mentioning the app in one of the
| team's internal rooms or by writing to it directly, and Operator
| answers in the thread as it would in the app: as that teammate, with
| their permissions and only the conversations they may see. Rooms where
| customers read, shared, community and helpdesk ones, never get an
| answer. Only a Slack user linked to a teammate is answered; in a room
| anyone else is told how to connect, while their direct messages stay
| with the Slack channel, where a customer's are support requests.
|
| Each Slack thread is one Operator thread, and a link to a dimmi
| conversation in the first message makes the thread about it. The
| answer replaces a short placeholder, and changes Operator proposes come
| as cards a teammate applies or declines from Slack.
|
*/

package operatorslack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/actions/slackteam"
	"github.com/dimmihq/dimmi/app/ai/operator"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
placeholder is what the thread shows while Operator works.
*/
const placeholder = "_Operator is looking into it…_"

/*
conversationLink finds a dimmi conversation's public id in a message.
*/
var conversationLink = regexp.MustCompile(`\bconv_[0-9A-Za-z]+`)

/*
Operator answers teammates in Slack. Bot opens Slack with the
connection's bot token, and AppURL is the install's address.
*/
type Operator struct {
	DB         *database.DB
	Clock      clock.Clock
	IDs        *ids.Registry
	Threads    *operatorchat.Threads
	Changesets *operatorchat.Changesets
	Bot        func(ctx context.Context, conn models.ChannelConnection) (*slackdriver.API, error)
	AppURL     string
}

/*
Asked is one message to Operator: the connection it arrived on and the
Slack event.
*/
type Asked struct {
	Connection models.ChannelConnection
	Event      slackdriver.Event
}

/*
OnMention answers a mention of the app in an internal room, reporting
false for a room customers read or that is not watched.
*/
func (o *Operator) OnMention(ctx context.Context, in Asked) (bool, error) {
	if !fromPerson(in) {
		return false, nil
	}

	ctx = tenancy.With(ctx, in.Connection.WorkspaceID.String())

	internal, err := o.internalRoom(ctx, in)
	if err != nil || !internal {
		return false, err
	}

	who, linked, err := o.asker(ctx, in)
	if err != nil {
		return true, err
	}

	if !linked {
		o.connectFirst(ctx, in)

		return true, nil
	}

	return true, o.answer(ctx, in, who)
}

/*
OnDirect answers a linked teammate writing to the app directly while
Operator is on, and leaves every other direct message to the channel.
*/
func (o *Operator) OnDirect(ctx context.Context, in Asked) (bool, error) {
	if !fromPerson(in) {
		return false, nil
	}

	ctx = tenancy.With(ctx, in.Connection.WorkspaceID.String())

	who, linked, err := o.asker(ctx, in)
	if err != nil || !linked {
		return false, err
	}

	on, err := o.enabled(ctx, in.Connection.WorkspaceID)
	if err != nil || !on {
		return false, err
	}

	return true, o.answer(ctx, in, who)
}

/*
fromPerson reports whether a person wrote the message, not a bot, an
edit or the app itself.
*/
func fromPerson(in Asked) bool {
	e := in.Event
	if e.BotID != "" || e.User == "" || e.User == botUser(in.Connection) {
		return false
	}

	switch e.Subtype {
	case "", "file_share", "thread_broadcast", "me_message":
		return true
	}

	return false
}

/*
internalRoom reports whether the message was written in a watched room
of the team's own.
*/
func (o *Operator) internalRoom(ctx context.Context, in Asked) (bool, error) {
	var room models.ChannelRoom

	err := o.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		room, err = models.New(tx).FindChannelRoomByExternalID(ctx, models.FindChannelRoomByExternalIDParams{
			WorkspaceID: in.Connection.WorkspaceID, ChannelConnectionID: in.Connection.ID,
			ExternalID: in.Event.ChannelID(),
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("find room: %w", err)
	}

	return room.Status == string(enums.RoomStatusActive) && room.Mode == string(enums.RoomModeInternal), nil
}

/*
asker returns the teammate who wrote, reporting false when they are not
linked.
*/
func (o *Operator) asker(ctx context.Context, in Asked) (slackteam.Teammate, bool, error) {
	team := slackteam.TeamOf(in.Connection, in.Event.UserTeam, in.Event.Team)

	return slackteam.Linked(ctx, o.DB, in.Connection, slackteam.User{TeamID: team, UserID: in.Event.User})
}

/*
enabled reports whether Operator is on for the workspace.
*/
func (o *Operator) enabled(ctx context.Context, ws ids.UUID) (bool, error) {
	var on bool

	err := o.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		s, err := operator.LoadSettings(ctx, models.New(tx), ws)
		on = s.Enabled

		return err
	})

	return on, err
}

/*
connectFirst asks a Slack user who is not linked to connect their
account, in a message only they see.
*/
func (o *Operator) connectFirst(ctx context.Context, in Asked) {
	api, err := o.Bot(ctx, in.Connection)
	if err != nil {
		return
	}

	_ = api.Ephemeral(ctx, slackdriver.Notice{
		Channel: in.Event.ChannelID(), User: in.Event.User, ThreadTS: in.Event.ThreadTS,
		Text: slackteam.ConnectText(o.AppURL, "talk with Operator"),
	})
}

/*
question is the message without the mention of the app.
*/
func question(in Asked) string {
	text := strings.ReplaceAll(in.Event.Text, "<@"+botUser(in.Connection)+">", "")

	return strings.TrimSpace(slackdriver.MrkdwnToMarkdown(text, nil))
}

/*
rootOf is the thread the message belongs to: its own when it starts
one.
*/
func rootOf(e slackdriver.Event) string {
	if e.ThreadTS != "" {
		return e.ThreadTS
	}

	return e.TS
}

/*
mentioned returns the dimmi conversation a message links to, if any.
*/
func (o *Operator) mentioned(text string) *ids.UUID {
	match := conversationLink.FindString(text)
	if match == "" {
		return nil
	}

	id, err := o.IDs.Decode(string(models.PrefixConversation), match)
	if err != nil {
		return nil
	}

	return &id
}

/*
botUser is the connection's bot user, whose own messages and mentions
are not questions.
*/
func botUser(conn models.ChannelConnection) string {
	var facts struct {
		BotUserID string `json:"bot_user_id"`
	}

	_ = json.Unmarshal(conn.Config, &facts)

	return facts.BotUserID
}
