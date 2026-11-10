/*
|--------------------------------------------------------------------------
| Slack Team Calls
|--------------------------------------------------------------------------
|
| The calls the team's own work in Slack makes: posting and updating the
| cards the team acts on, a notice with a button, the agent's session
| status in a thread, the App Home a requester sees, joining a channel
| and listing the public ones a rule may join. Like every other call,
| results come back in plain shapes and errors as the channel
| contract's.
|
*/

package slack

import (
	"context"

	"github.com/slack-go/slack"
)

/*
The agent session states the channel sets on a thread.
*/
const (
	SessionProcessing = "processing"
	SessionActive     = "active"
)

/*
TeamCard is a message of blocks the team acts on: where it goes, its text
for notifications and its blocks.
*/
type TeamCard struct {
	Channel  string
	ThreadTS string
	Text     string
	blocks   []slack.Block
}

/*
PostTeamCard posts the card as the bot and returns its timestamp.
*/
func (a *API) PostTeamCard(ctx context.Context, c TeamCard) (string, error) {
	opts := []slack.MsgOption{
		slack.MsgOptionText(Truncate(c.Text, MaxBodyChars), false), slack.MsgOptionBlocks(c.blocks...),
		slack.MsgOptionDisableLinkUnfurl(),
	}
	if c.ThreadTS != "" {
		opts = append(opts, slack.MsgOptionTS(c.ThreadTS))
	}

	return a.postMessage(ctx, c.Channel, opts...)
}

/*
UpdateTeamCard replaces the message at the timestamp with the card.
*/
func (a *API) UpdateTeamCard(ctx context.Context, ts string, c TeamCard) error {
	_, _, _, err := a.client.UpdateMessageContext(ctx, c.Channel, ts,
		slack.MsgOptionText(Truncate(c.Text, MaxBodyChars), false), slack.MsgOptionBlocks(c.blocks...))

	return MapError("chat.update", err)
}

/*
TeamText is Markdown the bot posts for the team: where it goes, in the
thread when one is named, and the name it shows.
*/
type TeamText struct {
	Channel  string
	ThreadTS string
	Markdown string
	Name     string
}

/*
PostText posts the text as the bot and returns its timestamp.
*/
func (a *API) PostText(ctx context.Context, t TeamText) (string, error) {
	opts := []slack.MsgOption{
		slack.MsgOptionMarkdownText(Truncate(ToSlack(t.Markdown), MaxBodyChars)), slack.MsgOptionDisableLinkUnfurl(),
	}
	if t.ThreadTS != "" {
		opts = append(opts, slack.MsgOptionTS(t.ThreadTS))
	}

	if t.Name != "" {
		opts = append(opts, slack.MsgOptionUsername(Truncate(t.Name, nameLimit)))
	}

	return a.postMessage(ctx, t.Channel, opts...)
}

/*
EphemeralTeamCard shows the card to one user alone.
*/
func (a *API) EphemeralTeamCard(ctx context.Context, user string, c TeamCard) error {
	opts := []slack.MsgOption{
		slack.MsgOptionText(Truncate(c.Text, MaxBodyChars), false), slack.MsgOptionBlocks(c.blocks...),
	}
	if c.ThreadTS != "" {
		opts = append(opts, slack.MsgOptionTS(c.ThreadTS))
	}

	_, err := a.client.PostEphemeralContext(ctx, c.Channel, user, opts...)

	return MapError("chat.postEphemeral", err)
}

/*
SetSession sets the agent's session status in the thread.
*/
func (a *API) SetSession(ctx context.Context, channel, threadTS, status string) error {
	_, err := a.client.SetAgentSessionStatusContext(ctx, slack.AgentSessionSetStatusParameters{
		Status: status, ChannelID: channel, ThreadTS: threadTS,
	})

	return MapError("agents.sessions.setStatus", err)
}

/*
PublishHome shows the user their App Home.
*/
func (a *API) PublishHome(ctx context.Context, user string, items []HomeItem) error {
	_, err := a.client.PublishViewContext(ctx, slack.PublishViewContextRequest{
		UserID: user, View: slack.HomeTabViewRequest{Type: slack.VTHomeTab, Blocks: homeBlocks(items)},
	})

	return MapError("views.publish", err)
}

/*
OpenDraftEditor opens a form to edit the draft before sending it.
*/
func (a *API) OpenDraftEditor(ctx context.Context, triggerID, runPublicID, text string) error {
	_, err := a.client.OpenViewContext(ctx, triggerID, draftEditor(runPublicID, text))

	return MapError("views.open", err)
}

/*
Join joins the public channel.
*/
func (a *API) Join(ctx context.Context, channel string) (Channel, error) {
	c, _, _, err := a.client.JoinConversationContext(ctx, channel)
	if err != nil {
		return Channel{}, MapError("conversations.join", err)
	}

	return channelOf(*c), nil
}

/*
ChannelPage returns a page of at most limit of the team's public and
private channels the token can see, archived ones left out, and the
cursor of the next.
*/
func (a *API) ChannelPage(ctx context.Context, cursor string, limit int) ([]Channel, string, error) {
	page, next, err := a.client.GetConversationsContext(ctx, &slack.GetConversationsParameters{
		Cursor: cursor, ExcludeArchived: true, Limit: limit, Types: []string{"public_channel", "private_channel"},
	})
	if err != nil {
		return nil, "", MapError("conversations.list", err)
	}

	out := make([]Channel, 0, len(page))
	for _, c := range page {
		out = append(out, channelOf(c))
	}

	return out, next, nil
}
