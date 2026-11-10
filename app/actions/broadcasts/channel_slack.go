/*
|--------------------------------------------------------------------------
| Slack Broadcasts
|--------------------------------------------------------------------------
|
| A Slack broadcast posts into each customer channel, as the dimmi app or
| as the teammate's own Slack user when they connected it, which reads as
| a personal note from their account manager. A channel the app left,
| or one that was archived, fails with Slack's reason and the rest go on.
| Sent posts can be edited or taken back later, and replies in their
| threads open conversations.
|
*/

package broadcasts

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Why a Slack broadcast cannot go out.
*/
var (
	ErrNoSlack = fault.New(fault.Invalid, "broadcast_slack_missing",
		"Connect Slack and add dimmi to your customers' channels before sending Slack broadcasts.")
	ErrNoSlackUser = fault.New(fault.Invalid, "broadcast_slack_user_missing",
		"Connect your own Slack account to send as yourself, or send as the dimmi app.")
)

/*
SlackSender posts broadcasts into customer Slack channels.
*/
type SlackSender struct {
	DB     *database.DB
	Driver *slackdriver.Driver
}

/*
Ready refuses a Slack broadcast when no channel can be posted to, or
when it is sent as a teammate who never connected Slack.
*/
func (s *SlackSender) Ready(ctx context.Context, q *models.Queries, row models.Broadcast) error {
	rooms, err := q.ListBroadcastRooms(ctx, models.ListBroadcastRoomsParams{
		WorkspaceID: row.WorkspaceID, RoomIds: nonNil(row.RoomIds),
	})
	if err != nil {
		return wrap("list broadcast rooms", err)
	}

	if len(rooms) == 0 {
		return ErrNoSlack
	}

	if row.SlackAs != "member" || row.SenderMemberID == nil {
		return nil
	}

	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: row.WorkspaceID, ID: rooms[0].ChannelConnectionID,
	})
	if err != nil {
		return wrap("find slack connection", err)
	}

	if _, ok, err := s.Driver.Credentials.UserToken(ctx, conn, *row.SenderMemberID); err != nil || !ok {
		return ErrNoSlackUser
	}

	return nil
}

/*
Deliver posts the broadcast into the recipient's channel.
*/
func (s *SlackSender) Deliver(ctx context.Context, d Delivery) (Delivered, error) {
	if d.Room == nil {
		return Delivered{}, &UndeliverableError{Status: "failed", Reason: "channel_removed"}
	}

	api, err := s.api(ctx, d.Broadcast, *d.Room)
	if err != nil {
		return Delivered{}, err
	}

	text := slackdriver.TeamText{Channel: d.Room.ExternalID, Markdown: slackBody(d)}
	if d.Broadcast.SlackAs != "member" {
		text.Name = d.Message.SenderName
	}

	ts, err := api.PostText(ctx, text)
	if err != nil {
		return Delivered{}, slackFailure(err)
	}

	return Delivered{ExternalRef: &ts}, nil
}

/*
api opens Slack as the teammate when the broadcast is sent as them,
else as the app.
*/
func (s *SlackSender) api(
	ctx context.Context, row models.Broadcast, room models.ChannelRoom) (*slackdriver.API, error,
) {
	conn, err := s.connection(ctx, room)
	if err != nil {
		return nil, err
	}

	if row.SlackAs == "member" && row.SenderMemberID != nil {
		token, ok, err := s.Driver.Credentials.UserToken(ctx, conn, *row.SenderMemberID)
		if err != nil {
			return nil, err
		}

		if !ok {
			return nil, &UndeliverableError{Status: "failed", Reason: "sender_slack_disconnected"}
		}

		return s.Driver.API(token), nil
	}

	token, err := s.Driver.Credentials.BotToken(ctx, conn)
	if err != nil {
		return nil, slackFailure(err)
	}

	return s.Driver.API(token), nil
}

/*
connection loads the Slack connection a channel belongs to.
*/
func (s *SlackSender) connection(ctx context.Context, room models.ChannelRoom) (models.ChannelConnection, error) {
	var conn models.ChannelConnection

	err := s.DB.Transaction(withTenant(ctx, room.WorkspaceID), func(ctx context.Context, tx *database.Tx) error {
		var err error

		conn, err = models.New(tx).FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: room.WorkspaceID, ID: room.ChannelConnectionID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && conn.DeletedAt != nil) {
		return conn, &UndeliverableError{Status: "failed", Reason: "slack_disconnected"}
	}

	return conn, wrap("find slack connection", err)
}

/*
slackBody is the post as Slack shows it: the title in bold above the
message.
*/
func slackBody(d Delivery) string {
	if d.Message.Subject == "" {
		return d.Message.Markdown
	}

	return "**" + d.Message.Subject + "**\n\n" + d.Message.Markdown
}

/*
slackFailure sorts a Slack refusal: one that will not change fails the
channel, anything else is tried again.
*/
func slackFailure(err error) error {
	if errors.Is(err, contracts.ErrPermanent) || errors.Is(err, contracts.ErrConnectionBroken) {
		reason := slackdriver.Code(err)
		if reason == "" {
			reason = "slack_refused"
		}

		return &UndeliverableError{Status: "failed", Reason: reason}
	}

	return err
}
