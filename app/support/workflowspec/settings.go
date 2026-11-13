/*
|--------------------------------------------------------------------------
| Step Settings
|--------------------------------------------------------------------------
|
| Each step kind has typed settings. Ids of teams, members, tags and
| other workflows are public ids, so a definition reads the same in an
| export as in the builder. Optional switches are pointers: absent means
| the documented default, which can then change without rewriting every
| stored definition.
|
*/

package workflowspec

/*
MessageSettings sends a message: Text with variables, after an optional
Delay of up to 30 seconds shown as typing.
*/
type MessageSettings struct {
	Text  string `json:"text"`
	Delay string `json:"delay,omitempty"`
}

/*
ArticleSettings answers with a help article: Article is the article's
id, and Text an optional line before it.
*/
type ArticleSettings struct {
	Article string `json:"article"`
	Text    string `json:"text,omitempty"`
}

/*
ButtonsSettings asks a question answered with buttons, one per user
path. AllowTyping (default true) lets the customer write instead;
Timeout ends the wait; RetryText is sent once on text channels when a
reply matches no choice.
*/
type ButtonsSettings struct {
	Text        string `json:"text"`
	AllowTyping *bool  `json:"allow_typing,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
	RetryText   string `json:"retry_text,omitempty"`
}

/*
CollectSettings asks for one value and saves it to Field: a contact
field, a custom attribute or a run variable. Type validates the answer;
Options lists the choices of a choice question. SkipIfKnown (default
true) skips the question when the field has a value.
*/
type CollectSettings struct {
	Text        string   `json:"text"`
	Field       string   `json:"field"`
	Type        string   `json:"type"`
	Options     []string `json:"options,omitempty"`
	SkipIfKnown *bool    `json:"skip_if_known,omitempty"`
	InvalidText string   `json:"invalid_text,omitempty"`
	MaxAttempts int      `json:"max_attempts,omitempty"`
	Timeout     string   `json:"timeout,omitempty"`
	AllowSkip   bool     `json:"allow_skip,omitempty"`
}

/*
ReplyTimeSettings tells the customer when the team will reply. Text
holds the {{reply_time}} variable, worked out from the office hours of
ScheduleID (the inbox's when empty) and the load of TeamID (the whole
workspace when empty).
*/
type ReplyTimeSettings struct {
	Text       string `json:"text,omitempty"`
	ScheduleID string `json:"schedule_id,omitempty"`
	TeamID     string `json:"team_id,omitempty"`
	Style      string `json:"style,omitempty"`
}

/*
DimmiSettings hands the conversation to dimmi until it resolves,
hands over or the customer goes quiet.
*/
type DimmiSettings struct {
	Instructions           string `json:"instructions,omitempty"`
	MaxReplies             int    `json:"max_replies,omitempty"`
	QuietAfter             string `json:"quiet_after,omitempty"`
	ConfirmResolution      *bool  `json:"confirm_resolution,omitempty"`
	CloseOnResolved        *bool  `json:"close_on_resolved,omitempty"`
	AskEmailBeforeHandover *bool  `json:"ask_email_before_handover,omitempty"`
	HandoverNotice         string `json:"handover_notice,omitempty"`
}

/*
On returns a switch's value, or its default when unset.
*/
func On(b *bool, fallback bool) bool {
	if b == nil {
		return fallback
	}

	return *b
}
