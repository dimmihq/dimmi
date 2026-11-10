/*
|--------------------------------------------------------------------------
| Answer Metadata
|--------------------------------------------------------------------------
|
| An agent's answer says about itself what the channels show with it:
| how it ended, what the customer may do next and the articles it rests
| on. Every channel reads the same keys, so an answer looks the same
| wherever it is shown and whenever it is shown again.
|
*/

package models

import "encoding/json"

/*
AnswerSource is one article an agent's answer cites.
*/
type AnswerSource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

/*
AnswerMetadata is what an agent's answer says about itself: its
outcome, the actions it offers and the articles it cites.
*/
type AnswerMetadata struct {
	Outcome string         `json:"outcome"`
	Actions []string       `json:"actions"`
	Sources []AnswerSource `json:"sources"`
}

/*
ReadAnswerMetadata reads an agent's answer about itself from a
message's metadata; a message without it reads as empty.
*/
func ReadAnswerMetadata(raw []byte) AnswerMetadata {
	var meta struct {
		AI AnswerMetadata `json:"ai"`
	}

	if json.Unmarshal(raw, &meta) != nil {
		return AnswerMetadata{}
	}

	return meta.AI
}
