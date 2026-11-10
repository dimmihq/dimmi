/*
|--------------------------------------------------------------------------
| Alerts In Slack
|--------------------------------------------------------------------------
|
| An alert in Slack reads like the bell: its title, the customer's words
| when previews are on, and a link back. An alert about a conversation
| also offers what a teammate most often does next without leaving
| Slack: take it, snooze it until tomorrow, mark it urgent, or open it in
| dimmi. Every customer word is escaped, so it can never ping a channel.
|
*/

package notifications

import (
	"context"
	"strconv"
	"strings"

	fnotify "github.com/dimmihq/dimmi/framework/notifications"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackActionPrefix starts the action id of every alert button.
*/
const SlackActionPrefix = "dimmi_notify:"

/*
The buttons of an alert about a conversation.
*/
const (
	SlackActionTake   = SlackActionPrefix + "take"
	SlackActionSnooze = SlackActionPrefix + "snooze"
	SlackActionUrgent = SlackActionPrefix + "urgent"
	SlackActionOpen   = SlackActionPrefix + "open"
)

/*
ToSlack is the alert as a Slack card. A post in a team's channel names
who a conversation went to, since nobody there is "you".
*/
func (a *Alert) ToSlack(ctx context.Context, _ fnotify.Delivery, to SlackRoute) (slackdriver.Card, error) {
	title := a.title(ctx)
	if to.UserID == "" && a.Trigger == TriggerAssigned {
		title = channelAssignedTitle(a.TitleArgs)
	}

	head := "*" + slackdriver.Escape(title) + "*"
	if a.Number > 0 {
		head += "  " + slackdriver.LinkText(a.Link, "#"+strconv.FormatInt(a.Number, 10))
	}

	card := slackdriver.Card{
		Text: title, Lines: []string{head},
		Context: slackdriver.LinkText(a.Link, "Open in dimmi") + " · " + slackdriver.Escape(a.Workspace),
	}

	if a.Body != "" {
		card.Lines = append(card.Lines, quote(a.Body))
	}

	if a.ConversationID != nil {
		card.Buttons = ConversationButtons(models.PublicID(models.PrefixConversation, *a.ConversationID), a.Link)
	}

	return card, nil
}

/*
channelAssignedTitle names who took a conversation, for a channel.
*/
func channelAssignedTitle(args map[string]string) string {
	if args["actor"] == "" {
		return args["name"] + "'s conversation was assigned"
	}

	return args["actor"] + " assigned " + args["name"] + "'s conversation"
}

/*
ConversationButtons are what a teammate can do about a conversation
from Slack.
*/
func ConversationButtons(conversation, link string) []slackdriver.Button {
	return []slackdriver.Button{
		{ActionID: SlackActionTake, Label: "Take it", Value: conversation, Primary: true},
		{ActionID: SlackActionSnooze, Label: "Snooze until tomorrow", Value: conversation},
		{ActionID: SlackActionUrgent, Label: "Mark urgent", Value: conversation},
		{ActionID: SlackActionOpen, Label: "Open in dimmi", Value: conversation, URL: link},
	}
}

/*
quote shows the customer's words as a Slack quote, escaped.
*/
func quote(text string) string {
	lines := strings.Split(slackdriver.Escape(text), "\n")
	for i, line := range lines {
		lines[i] = ">" + line
	}

	return strings.Join(lines, "\n")
}
