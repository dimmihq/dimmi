/*
|--------------------------------------------------------------------------
| Slack State In Place
|--------------------------------------------------------------------------
|
| What a teammate does in the inbox shows where the customer is. A
| closed conversation gets a check mark on its thread's first message,
| taken off again when it reopens. While the agent writes an answer the
| customer's message carries an eyes reaction, removed once the answer
| is posted or dropped. A reply a teammate deletes in the inbox is
| deleted in Slack too. A mark already in place,
| or already gone, is no error.
|
*/

package slack

import (
	"context"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MarkState puts the closed mark on the thread's first message, or takes
it off. A direct message has no thread to mark.
*/
func (d *Driver) MarkState(ctx context.Context, conn models.ChannelConnection, threadKey string, closed bool) error {
	thread, ok := ParseThreadKey(threadKey)
	if !ok || thread.Direct() || d.Markers.Closed == "" {
		return nil
	}

	channel, err := d.Places.RoomChannel(ctx, conn, thread.RoomID)
	if err != nil {
		return err
	}

	return d.react(ctx, conn, reaction{channel: channel, ts: thread.RootTS, name: d.Markers.Closed, on: closed})
}

/*
StartWorking puts the working sign on the customer's message.
*/
func (d *Driver) StartWorking(ctx context.Context, conn models.ChannelConnection, ref contracts.ChatMessageRef) error {
	return d.working(ctx, conn, ref, true)
}

/*
StopWorking takes the working sign off the customer's message.
*/
func (d *Driver) StopWorking(ctx context.Context, conn models.ChannelConnection, ref contracts.ChatMessageRef) error {
	return d.working(ctx, conn, ref, false)
}

/*
working puts the working sign on the message, or takes it off.
*/
func (d *Driver) working(
	ctx context.Context, conn models.ChannelConnection, ref contracts.ChatMessageRef, on bool,
) error {
	channel, ts, ok := SplitMessageID(ref.ChannelMessageID)
	if !ok || d.Markers.Working == "" {
		return nil
	}

	return d.react(ctx, conn, reaction{channel: channel, ts: ts, name: d.Markers.Working, on: on})
}

/*
reaction is a reaction to put on a message, or take off it.
*/
type reaction struct {
	channel string
	ts      string
	name    string
	on      bool
}

/*
react adds the reaction as the bot, or removes it.
*/
func (d *Driver) react(ctx context.Context, conn models.ChannelConnection, r reaction) error {
	token, err := d.Credentials.BotToken(ctx, conn)
	if err != nil {
		return err
	}

	if r.on {
		return d.API(token).React(ctx, r.channel, r.ts, r.name)
	}

	return d.API(token).Unreact(ctx, r.channel, r.ts, r.name)
}

/*
DeleteSent deletes a delivered reply in Slack: with the teammate's own
account when they have one there, since a reply posted with it can only
be removed with it, and as the bot otherwise.
*/
func (d *Driver) DeleteSent(ctx context.Context, conn models.ChannelConnection, ref contracts.ChatMessageRef) error {
	channel, ts, ok := SplitMessageID(ref.ChannelMessageID)
	if !ok {
		return nil
	}

	if ref.AuthorMemberID != nil {
		user, ok, err := d.Credentials.UserToken(ctx, conn, *ref.AuthorMemberID)
		if err == nil && ok && d.API(user).Delete(ctx, channel, ts) == nil {
			return nil
		}
	}

	token, err := d.Credentials.BotToken(ctx, conn)
	if err != nil {
		return err
	}

	return d.API(token).Delete(ctx, channel, ts)
}
