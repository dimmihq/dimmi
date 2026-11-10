/*
|--------------------------------------------------------------------------
| Team Action Keys
|--------------------------------------------------------------------------
|
| Every button the team presses names what it acts on and what it does,
| so a press finds both without any state of its own: a triage card's
| buttons the conversation, a draft's the run that wrote it, a feedback
| button the answer. A button that names nothing the product knows does
| nothing.
|
*/

package slack

import "strings"

/*
Prefixes of the team's action ids, and the callback id of the form that
edits a draft.
*/
const (
	triagePrefix     = "tr:"
	draftPrefix      = "dr:"
	feedbackPrefix   = "fb:"
	discussionPrefix = "ds:"
	DraftEditorID    = "dimmi_draft_edit"
	DraftEditorBlock = "draft"
	DraftEditorInput = "text"
)

/*
The verbs of a triage card and of a draft.
*/
const (
	TriageTake    = "take"
	TriageStatus  = "status"
	TriageSnooze  = "snooze"
	TriageUrgent  = "urgent"
	TriageOpen    = "open"
	TriageAIBack  = "aiback"
	TriageArticle = "article"
	DraftSend     = "send"
	DraftEdit     = "edit"
	DraftDiscard  = "discard"
)

/*
TriagePrefix is what every triage card action id starts with.
*/
func TriagePrefix() string { return triagePrefix }

/*
DraftPrefix is what every draft action id starts with.
*/
func DraftPrefix() string { return draftPrefix }

/*
FeedbackPrefix is what every feedback action id starts with.
*/
func FeedbackPrefix() string { return feedbackPrefix }

/*
DiscussionPrefix is what the start discussion button's id starts with.
*/
func DiscussionPrefix() string { return discussionPrefix }

/*
TeamActionID names the verb on the thing with the public id.
*/
func TeamActionID(prefix, publicID, verb string) string {
	return prefix + publicID + ":" + verb
}

/*
ParseTeamActionID reads an action id of the prefix into the public id
and the verb.
*/
func ParseTeamActionID(prefix, actionID string) (publicID, verb string, ok bool) {
	rest, ok := strings.CutPrefix(actionID, prefix)
	if !ok {
		return "", "", false
	}

	publicID, verb, ok = strings.Cut(rest, ":")

	return publicID, verb, ok && publicID != "" && verb != ""
}
