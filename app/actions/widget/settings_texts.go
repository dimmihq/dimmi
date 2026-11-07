/*
|--------------------------------------------------------------------------
| Widget Texts
|--------------------------------------------------------------------------
|
| The words a customer writes for their messenger, each with a limit that
| keeps it fitting the place it shows. Plain text only: the widget never
| renders a customer's text as markup. An empty key means the built-in
| words.
|
*/

package widget

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
TextLimits returns every text key and its longest length.
*/
func TextLimits() map[string]int {
	limits := map[string]int{
		"greeting_intro": 60, "greeting": 280, "welcome_message": 280, "team_intro": 140, "delay_notice": 280,
		"offline_message": 280, "office_hours_text": 140, "email_prompt": 140, "privacy_notice": 280,
		"composer_placeholder": 60, "chat_title": 60, "chat_body": 140, "messages_empty_title": 60,
		"messages_empty_body": 140, "launcher_label": 24,
	}

	for _, key := range []string{"recent_messages", "links", "show_all", "send_message"} {
		limits[key] = 40
	}

	for _, key := range []string{
		"search_placeholder", "popular", "helpful", "update_feedback", "still_need_help", "action_waiting",
		"action_done", "action_declined", "action_canceled", "action_expired", "action_failed",
	} {
		limits[key] = 80
	}

	for _, key := range []string{
		"home", "messages", "changelog", "help", "feedback", "action_confirm", "action_cancel",
	} {
		limits[key] = 24
	}

	for _, key := range []string{
		"status_with_team", "status_waiting_on_you", "status_closed", "seen", "reopen_hint", "deleted_message",
		"new_conversation", "talk_to_human", "ai_answering",
	} {
		limits[key] = 40
	}

	for i := range maxHomeLinks {
		limits["home_link_"+strconv.Itoa(i)] = 40
	}

	return limits
}

/*
maxHomeLinks is the most links a home screen lists.
*/
const maxHomeLinks = 6

/*
mergeTexts returns the stored texts with the change applied: null or empty
removes a key, and every text is cleaned of control characters.
*/
func mergeTexts(stored json.RawMessage, change map[string]any) (json.RawMessage, error) {
	current := map[string]any{}
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &current); err != nil {
			return nil, fmt.Errorf("read widget texts: %w", err)
		}
	}

	cleaned, err := CheckTexts(change)
	if err != nil {
		return nil, err
	}

	merged := mergeObject(current, cleaned)

	out, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("encode widget texts: %w", err)
	}

	return out, nil
}

/*
CheckTexts refuses an unknown key or a text over its limit, and returns the
change cleaned, with empty texts as null so they fall back to the built-in
words.
*/
func CheckTexts(change map[string]any) (map[string]any, error) {
	limits, out := TextLimits(), make(map[string]any, len(change))

	for key, value := range change {
		limit, known := limits[key]
		if !known {
			return nil, failField("texts."+key, "This is not a text the messenger shows.")
		}

		if value == nil {
			out[key] = nil

			continue
		}

		text, ok := value.(string)
		if !ok {
			return nil, failField("texts."+key, "Use plain text.")
		}

		text = str.Clean(text)
		if len([]rune(text)) > limit {
			return nil, failField("texts."+key, fmt.Sprintf("Use at most %d characters.", limit))
		}

		out[key] = nil
		if text != "" {
			out[key] = text
		}
	}

	return out, nil
}
