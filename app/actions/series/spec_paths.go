/*
|--------------------------------------------------------------------------
| Steps That Choose A Path
|--------------------------------------------------------------------------
|
| Waiting for something, checking the traveler and splitting them
| between variants each name the paths they can send a traveler down.
|
*/

package series

import "encoding/json"

/*
WaitForSettings holds the person until they match Rules, do Event, or
act on what an earlier step sent them, for at most Amount Units.
*/
type WaitForSettings struct {
	Rules      json.RawMessage `json:"rules,omitempty"`
	Event      string          `json:"event,omitempty"`
	Engagement *Engagement     `json:"engagement,omitempty"`
	Amount     int             `json:"amount"`
	Unit       string          `json:"unit"`
}

/*
Engagement is what someone did with an earlier message step: opened,
clicked or replied.
*/
type Engagement struct {
	Node   string `json:"node"`
	Action string `json:"action"`
}

/*
ConditionSettings is a check: the first path whose rules or engagement
match is taken, and everyone else goes down else.
*/
type ConditionSettings struct {
	Paths []ConditionPath `json:"paths"`
}

/*
ConditionPath is one labeled path of a check.
*/
type ConditionPath struct {
	ID         string          `json:"id"`
	Label      string          `json:"label"`
	Rules      json.RawMessage `json:"rules,omitempty"`
	Engagement *Engagement     `json:"engagement,omitempty"`
}

/*
SplitSettings sends each traveler down one variant by weight, or down
Winner once one is picked.
*/
type SplitSettings struct {
	Variants []Variant `json:"variants"`
	Winner   string    `json:"winner,omitempty"`
}
