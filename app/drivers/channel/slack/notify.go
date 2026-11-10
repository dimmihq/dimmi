/*
|--------------------------------------------------------------------------
| Slack Cards
|--------------------------------------------------------------------------
|
| Notifications, digests and Operator's answers reach Slack as cards: a
| few lines of Slack markup, a quiet line of context and, when there is
| something to do, a row of buttons. A card always carries plain fallback
| text for Slack's own notifications and for screen readers, and never
| unfurls links. Customer words put on a card are escaped first, so they
| can never mention a channel or a person.
|
| A teammate is found by the email they sign in with, and a direct
| message is opened once with them; Slack says when someone cannot be
| reached that way, and the caller stops trying.
|
*/

package slack

import (
	"context"
	"strings"

	"github.com/slack-go/slack"
)

/*
maxButtons is the most buttons Slack shows in one row.
*/
const maxButtons = 5

/*
Button is one button of a card: the action id its press is claimed by,
its label, the value the press carries, and, for a link, where it goes.
Primary marks the one button that stands out.
*/
type Button struct {
	ActionID string
	Label    string
	Value    string
	URL      string
	Primary  bool
}

/*
Card is a message in Slack's own markup: Text is the plain fallback,
Markdown a body written in standard Markdown, which Slack renders as is,
Lines its sections in Slack's own markup, in order, Context the small
print under them and Buttons what can be done from it.
*/
type Card struct {
	Text     string
	Markdown string
	Lines    []string
	Context  string
	Buttons  []Button
}

/*
blocks lays the card out as Slack blocks.
*/
func (c Card) blocks() []slack.Block {
	var out []slack.Block

	if strings.TrimSpace(c.Markdown) != "" {
		out = append(out, slack.NewMarkdownBlock("", Truncate(c.Markdown, MaxBodyChars)))
	}

	for _, line := range c.Lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		out = append(out, slack.NewSectionBlock(markup(Truncate(line, 3000)), nil, nil))
	}

	if c.Context != "" {
		out = append(out, slack.NewContextBlock("", markup(Truncate(c.Context, 3000))))
	}

	if len(c.Buttons) == 0 {
		return out
	}

	elements := make([]slack.BlockElement, 0, len(c.Buttons))
	for _, b := range c.Buttons[:min(len(c.Buttons), maxButtons)] {
		elements = append(elements, buttonOf(b))
	}

	return append(out, slack.NewActionBlock("", elements...))
}

/*
markup is a text object in Slack's markup.
*/
func markup(text string) *slack.TextBlockObject {
	return slack.NewTextBlockObject(slack.MarkdownType, text, false, false)
}

/*
buttonOf builds one button.
*/
func buttonOf(b Button) *slack.ButtonBlockElement {
	button := slack.NewButtonBlockElement(b.ActionID, b.Value,
		slack.NewTextBlockObject(slack.PlainTextType, Truncate(b.Label, 75), false, false))
	button.URL = b.URL

	if b.Primary {
		button.Style = slack.StylePrimary
	}

	return button
}

/*
options are the message options of the card.
*/
func (c Card) options() []slack.MsgOption {
	return []slack.MsgOption{
		slack.MsgOptionText(Truncate(c.Text, MaxBodyChars), false), slack.MsgOptionBlocks(c.blocks()...),
		slack.MsgOptionDisableLinkUnfurl(), slack.MsgOptionDisableMediaUnfurl(),
	}
}

/*
PostCard posts the card in the channel, in the thread when one is named,
and returns its timestamp.
*/
func (a *API) PostCard(ctx context.Context, channel, threadTS string, c Card) (string, error) {
	opts := c.options()
	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	return a.postMessage(ctx, channel, opts...)
}

/*
UpdateCard replaces a message the token posted with the card.
*/
func (a *API) UpdateCard(ctx context.Context, channel, ts string, c Card) error {
	_, _, _, err := a.client.UpdateMessageContext(ctx, channel, ts, c.options()...)

	return MapError("chat.update", err)
}

/*
PostReply posts Markdown in the thread and returns its timestamp.
*/
func (a *API) PostReply(ctx context.Context, channel, threadTS, markdown string) (string, error) {
	opts := []slack.MsgOption{
		slack.MsgOptionMarkdownText(Truncate(ToSlack(markdown), MaxBodyChars)), slack.MsgOptionDisableLinkUnfurl(),
	}
	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	return a.postMessage(ctx, channel, opts...)
}

/*
OpenDirect opens the direct message between the app and the user and
returns its channel.
*/
func (a *API) OpenDirect(ctx context.Context, userID string) (string, error) {
	channel, _, _, err := a.client.OpenConversationContext(ctx, &slack.OpenConversationParameters{
		Users: []string{userID}, ReturnIM: true,
	})
	if err != nil {
		return "", MapError("conversations.open", err)
	}

	return channel.ID, nil
}

/*
UserByEmail returns the user of the team whose email it is.
*/
func (a *API) UserByEmail(ctx context.Context, email string) (User, error) {
	u, err := a.client.GetUserByEmailContext(ctx, email)
	if err != nil {
		return User{}, MapError("users.lookupByEmail", err)
	}

	return User{
		ID: u.ID, TeamID: u.TeamID, Name: cmpOr(u.Profile.DisplayName, u.RealName, u.Name),
		Email: strings.ToLower(u.Profile.Email), IsBot: u.IsBot || u.IsAppUser, Deleted: u.Deleted,
	}, nil
}

/*
cmpOr returns the first of the names that is not empty.
*/
func cmpOr(names ...string) string {
	for _, n := range names {
		if n != "" {
			return n
		}
	}

	return ""
}

/*
Unreachable reports whether Slack said the person cannot be written to
directly: unknown, disabled, or a bot.
*/
func Unreachable(err error) bool {
	switch Code(err) {
	case "slack_users_not_found", "slack_user_not_found", "slack_user_disabled", "slack_cannot_dm_bot":
		return true
	}

	return false
}

/*
Escape makes words safe to put in Slack's markup: they show as written
and can never mention anyone or link anywhere.
*/
func Escape(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}

/*
LinkText writes a link with its label in Slack's markup.
*/
func LinkText(url, label string) string {
	return "<" + url + "|" + strings.NewReplacer("|", " ", ">", "", "<", "").Replace(label) + ">"
}
