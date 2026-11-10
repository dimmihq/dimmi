/*
|--------------------------------------------------------------------------
| Team Cards
|--------------------------------------------------------------------------
|
| What the team sees in Slack, as blocks. A triage card says who is
| waiting, on what, and where it stands, with the buttons to take it,
| move it on or open it in dimmi; once the team took the conversation
| from the agent it offers to hand it back, and once solved by the team
| to draft a help article from it. A draft shows the agent's answer for
| a teammate to send, edit or discard. The App Home lists a requester's
| conversations. Every text a customer wrote is shown as plain text, so
| nothing in it mentions anyone or formats the card.
|
*/

package slack

import (
	"cmp"
	"strings"

	"github.com/slack-go/slack"
)

/*
TriageCard is the triage post of one conversation: its public id, who
and what it is about, where it stands, and links to the customer's
thread and to dimmi. HandedOff offers handing it back to the agent,
Solved drafting a help article, Gone says the customer deleted it.
*/
type TriageCard struct {
	ConversationID string
	Customer       string
	Company        string
	Excerpt        string
	Status         string
	Assignee       string
	Priority       string
	SLADue         string
	ThreadLink     string
	AppLink        string
	HandedOff      bool
	Solved         bool
	Gone           bool
}

/*
TriageCardOf returns the card as a message to the channel.
*/
func TriageCardOf(channel string, c TriageCard) TeamCard {
	headline := "*" + shown(c.Customer) + "*"
	if c.Company != "" {
		headline += " · " + shown(c.Company)
	}

	if c.Gone {
		headline += " · _Deleted in Slack_"
	}

	blocks := []slack.Block{
		slack.NewSectionBlock(teamMarkup(headline+"\n>"+shown(Truncate(c.Excerpt, 280))), nil, nil),
		slack.NewContextBlock("facts", teamMarkup(facts(c))),
		slack.NewActionBlock("triage", triageButtons(c)...),
	}

	return TeamCard{Channel: channel, Text: c.Customer + ": " + Truncate(c.Excerpt, 120), blocks: blocks}
}

/*
facts is the card's line of state.
*/
func facts(c TriageCard) string {
	line := "Status: " + c.Status + " · Assignee: " + orNobody(c.Assignee) + " · Priority: " + c.Priority
	if c.SLADue != "" {
		line += " · SLA due " + c.SLADue
	}

	if c.ThreadLink != "" {
		line += " · <" + c.ThreadLink + "|Customer thread>"
	}

	return line
}

/*
orNobody names an empty assignee.
*/
func orNobody(name string) string {
	if name == "" {
		return "nobody"
	}

	return name
}

/*
triageButtons are the card's buttons and status menu.
*/
func triageButtons(c TriageCard) []slack.BlockElement {
	id := c.ConversationID
	out := []slack.BlockElement{
		teamButton(TeamActionID(triagePrefix, id, TriageTake), "Take it", slack.StylePrimary),
		slack.NewOptionsSelectBlockElement(slack.OptTypeStatic, label("Status"),
			TeamActionID(triagePrefix, id, TriageStatus),
			teamOption("open", "Open"), teamOption("closed", "Closed")),
		teamButton(TeamActionID(triagePrefix, id, TriageSnooze), "Snooze until tomorrow", ""),
		teamButton(TeamActionID(triagePrefix, id, TriageUrgent), "Urgent", slack.StyleDanger),
	}

	if c.HandedOff {
		out = append(out, teamButton(TeamActionID(triagePrefix, id, TriageAIBack), "Hand back to AI", ""))
	}

	if c.Solved {
		out = append(out, teamButton(TeamActionID(triagePrefix, id, TriageArticle), "Draft help article", ""))
	}

	open := teamButton(TeamActionID(triagePrefix, id, TriageOpen), "Open in dimmi", "")
	open.URL = c.AppLink

	return append(out, open)
}

/*
DraftCard is a draft the agent wrote for the team: the run's public id,
the conversation it answers and the answer.
*/
type DraftCard struct {
	RunID    string
	Customer string
	Body     string
}

/*
DraftCardOf returns the draft as a message to the channel's thread.
*/
func DraftCardOf(channel, threadTS string, d DraftCard) TeamCard {
	blocks := []slack.Block{
		slack.NewContextBlock("draft_note",
			teamMarkup("Draft answer for "+shown(d.Customer)+". Only the team sees it.")),
		slack.NewMarkdownBlock("draft_body", Truncate(ToSlack(d.Body), MaxBodyChars)),
		slack.NewActionBlock("draft_actions",
			teamButton(TeamActionID(draftPrefix, d.RunID, DraftSend), "Send", slack.StylePrimary),
			teamButton(TeamActionID(draftPrefix, d.RunID, DraftEdit), "Edit", ""),
			teamButton(TeamActionID(draftPrefix, d.RunID, DraftDiscard), "Discard", slack.StyleDanger)),
	}

	text := "Draft answer: " + Truncate(d.Body, 120)

	return TeamCard{Channel: channel, ThreadTS: threadTS, Text: text, blocks: blocks}
}

/*
SettledDraftOf returns the draft's message once it was sent or
discarded, its buttons gone.
*/
func SettledDraftOf(channel, outcome, body string) TeamCard {
	return TeamCard{Channel: channel, Text: outcome, blocks: []slack.Block{
		slack.NewContextBlock("draft_note", teamMarkup(outcome)),
		slack.NewMarkdownBlock("draft_body", Truncate(ToSlack(body), MaxBodyChars)),
	}}
}

/*
DiscussionOffer is the notice offering to start a discussion of the
conversation a teammate linked.
*/
func DiscussionOffer(channel, threadTS, conversationPublicID string) TeamCard {
	start := teamButton(TeamActionID(discussionPrefix, conversationPublicID, "start"), "Start discussion",
		slack.StylePrimary)

	blocks := []slack.Block{
		slack.NewSectionBlock(teamMarkup("Discuss this conversation here? Replies in the thread become notes."),
			nil, nil),
		slack.NewActionBlock("discussion", start),
	}

	text := "Start a discussion of this conversation?"

	return TeamCard{Channel: channel, ThreadTS: threadTS, Text: text, blocks: blocks}
}

/*
DiscussionCard opens a discussion thread: what the conversation is
about and its link.
*/
func DiscussionCard(channel, summary, link string) TeamCard {
	text := "Discussion: " + shown(Truncate(summary, 500)) + "\n<" + link +
		"|Open in dimmi> · Replies here become notes."

	return TeamCard{Channel: channel, Text: Truncate(summary, 120), blocks: []slack.Block{
		slack.NewSectionBlock(teamMarkup(text), nil, nil),
	}}
}

/*
Home is what one person's App Home shows: a heading, the line shown
when there is nothing, and their conversations.
*/
type Home struct {
	Heading string
	Empty   string
	Items   []HomeItem
}

/*
HomeItem is one conversation on an App Home, with a link and the words
the link reads.
*/
type HomeItem struct {
	Title     string
	Status    string
	Link      string
	LinkLabel string
}

/*
RequesterHome is the App Home of someone who asks the team for help.
*/
func RequesterHome(items []HomeItem) Home {
	return Home{Heading: "Your requests", Empty: "You have no requests yet. Ask in a support channel.", Items: items}
}

/*
TeammateHome is the App Home of a linked teammate: what they hold.
*/
func TeammateHome(items []HomeItem) Home {
	return Home{Heading: "Assigned to you", Empty: "Nothing is waiting on you right now.", Items: items}
}

/*
homeBlocks lists the person's conversations, or says there are none.
*/
func homeBlocks(home Home) slack.Blocks {
	blocks := []slack.Block{slack.NewHeaderBlock(label(home.Heading))}
	if len(home.Items) == 0 {
		blocks = append(blocks, slack.NewSectionBlock(teamMarkup(home.Empty), nil, nil))
	}

	for _, item := range home.Items {
		line := "*" + shown(Truncate(item.Title, 150)) + "* · " + item.Status
		if item.Link != "" {
			line += " · <" + item.Link + "|" + cmp.Or(item.LinkLabel, "Open thread") + ">"
		}

		blocks = append(blocks, slack.NewSectionBlock(teamMarkup(line), nil, nil))
	}

	return slack.Blocks{BlockSet: blocks}
}

/*
DraftEditorLimit is the longest draft Slack's editor can hold; a longer
one is edited in dimmi, so it is never sent cut short.
*/
const DraftEditorLimit = 3000

/*
draftEditor is the form that edits a draft before it is sent.
*/
func draftEditor(runPublicID, text string) slack.ModalViewRequest {
	input := slack.NewPlainTextInputBlockElement(label("Answer"), DraftEditorInput)
	input.Multiline = true
	input.InitialValue = Truncate(text, DraftEditorLimit)

	return slack.ModalViewRequest{
		Type: slack.VTModal, Title: label("Edit draft"), Submit: label("Send"), Close: label("Cancel"),
		CallbackID: DraftEditorID, PrivateMetadata: runPublicID,
		Blocks: slack.Blocks{BlockSet: []slack.Block{
			slack.NewInputBlock(DraftEditorBlock, label("Answer"), nil, input),
		}},
	}
}

/*
feedbackBlock asks for a verdict on the answer with the public id.
*/
func feedbackBlock(messagePublicID string) slack.Block {
	return slack.NewContextActionsBlock("feedback", slack.NewFeedbackButtonsBlockElement(
		feedbackPrefix+messagePublicID,
		slack.NewFeedbackButton(label("Good"), "good"), slack.NewFeedbackButton(label("Bad"), "bad")))
}

/*
teamButton is a button with its id, label and style.
*/
func teamButton(actionID, text string, style slack.Style) *slack.ButtonBlockElement {
	b := slack.NewButtonBlockElement(actionID, actionID, label(text))
	if style != "" {
		b.WithStyle(style)
	}

	return b
}

/*
teamOption is a menu option.
*/
func teamOption(value, text string) *slack.OptionBlockObject {
	return slack.NewOptionBlockObject(value, label(text), nil)
}

/*
label is a plain text object.
*/
func label(text string) *slack.TextBlockObject {
	return slack.NewTextBlockObject(slack.PlainTextType, text, false, false)
}

/*
teamMarkup is a mrkdwn text object.
*/
func teamMarkup(text string) *slack.TextBlockObject {
	return slack.NewTextBlockObject(slack.MarkdownType, text, false, false)
}

/*
shown escapes text someone wrote so it shows as written: no mention,
link or formatting can come of it.
*/
func shown(text string) string {
	return strings.NewReplacer("*", "\u2217", "_", "\uff3f", "~", "\u223c", "`", "'").
		Replace(escaper().Replace(text))
}
