/*
|--------------------------------------------------------------------------
| Steps That Act
|--------------------------------------------------------------------------
|
| Split variants, AI paths and the steps that change the traveler or
| tell another system about them.
|
*/

package series

import "encoding/json"

/*
Variant is one path of a split and its share.
*/
type Variant struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Weight int    `json:"weight"`
}

/*
AIPathSettings lets the AI pick one of Paths for the person by Prompt;
anything going wrong takes fallback.
*/
type AIPathSettings struct {
	Prompt string   `json:"prompt"`
	Paths  []AIPath `json:"paths"`
}

/*
AIPath is one path the AI may pick, described for it.
*/
type AIPath struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

/*
ActionSettings is a step that changes the person: tags to add or take
off, an attribute to write, or a topic to join or leave.
*/
type ActionSettings struct {
	TagIDs     []string        `json:"tag_ids,omitempty"`
	Key        string          `json:"key,omitempty"`
	Value      json.RawMessage `json:"value,omitempty"`
	TopicID    string          `json:"topic_id,omitempty"`
	Subscribed bool            `json:"subscribed,omitempty"`
}

/*
WebhookSettings posts the person and the step to URL.
*/
type WebhookSettings struct {
	URL string `json:"url"`
}
