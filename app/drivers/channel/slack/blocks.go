/*
|--------------------------------------------------------------------------
| Answer Controls
|--------------------------------------------------------------------------
|
| Every answer the agent gives on Slack ends with a way out: Solved for
| the customer who got what they needed, and Talk to a human for anyone
| who did not. The answer then goes as blocks, its Markdown in a
| markdown block with the buttons under it. Each button's action id
| names the answer and the control, so a click finds both without any
| state of its own. Once used, or once a teammate replies, the buttons
| go and the text stays.
|
*/

package slack

import (
	"cmp"
	"context"
	"strings"

	"github.com/slack-go/slack"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
controlLabels are the buttons' words.
*/
func controlLabels() map[string]string {
	return map[string]string{contracts.ControlSolved: "Solved", contracts.ControlHandoff: "Talk to a human"}
}

/*
controlBlocks returns the answer's Markdown as a block, the articles it
cites as a line under it, and a button for each control it offers.
*/
func controlBlocks(body, messagePublicID string, controls []string,
	sources []contracts.OutboundSource,
) []slack.Block {
	labels := controlLabels()
	buttons := make([]slack.BlockElement, 0, len(controls))

	for _, control := range controls {
		label, ok := labels[control]
		if !ok {
			continue
		}

		button := slack.NewButtonBlockElement(ControlActionID(messagePublicID, control), control,
			slack.NewTextBlockObject(slack.PlainTextType, label, false, false))
		if control == contracts.ControlSolved {
			button.WithStyle(slack.StylePrimary)
		}

		buttons = append(buttons, button)
	}

	blocks := []slack.Block{slack.NewMarkdownBlock("", body)}
	if cited := sourcesBlock(sources); cited != nil {
		blocks = append(blocks, cited)
	}
	if len(buttons) > 0 {
		blocks = append(blocks, slack.NewActionBlock("controls", buttons...))
	}

	return blocks
}

/*
ClearControls takes the buttons off a sent answer, keeping its text.
*/
func (d *Driver) ClearControls(ctx context.Context, conn models.ChannelConnection, ref contracts.ChatMessageRef,
	body string,
) error {
	channel, ts, ok := SplitMessageID(ref.ChannelMessageID)
	if !ok {
		return nil
	}

	token, err := d.Credentials.BotToken(ctx, conn)
	if err != nil {
		return err
	}

	return d.API(token).Update(ctx, channel, ts, ToSlack(body))
}

/*
sourcesBlock lists the cited articles as links, nil when there are none.
*/
func sourcesBlock(sources []contracts.OutboundSource) slack.Block {
	if len(sources) == 0 {
		return nil
	}

	links := make([]string, 0, len(sources))
	for _, s := range sources {
		links = append(links, LinkText(s.URL, cmp.Or(s.Title, s.URL)))
	}

	text := slack.NewTextBlockObject(slack.MarkdownType, "Sources: "+strings.Join(links, " · "), false, false)

	return slack.NewContextBlock("sources", text)
}
