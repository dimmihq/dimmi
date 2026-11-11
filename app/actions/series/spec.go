/*
|--------------------------------------------------------------------------
| Series Definition
|--------------------------------------------------------------------------
|
| A series is stored as plain JSON a person or a script can read: who
| enters, what makes them leave, the goal, and a graph of steps joined
| by paths. Step ids are made once and never change, which is what lets
| people already on a journey follow a newer version of it. Each kind of
| step reads its own settings from the same shape.
|
*/

package series

import (
	"encoding/json"
)

/*
Kinds of step.
*/
const (
	KindEntry        = "entry"
	KindEmail        = "email"
	KindChat         = "chat"
	KindPost         = "post"
	KindSlack        = "slack"
	KindWait         = "wait"
	KindWaitFor      = "wait_for"
	KindCondition    = "condition"
	KindSplit        = "split"
	KindAIPath       = "ai_path"
	KindTag          = "tag"
	KindUntag        = "untag"
	KindSetAttribute = "set_attribute"
	KindTopic        = "topic"
	KindWebhook      = "webhook"
	KindExit         = "exit"
)

/*
Paths a step leaves by, besides the ones its settings name.
*/
const (
	PathNext     = "next"
	PathFailed   = "failed"
	PathMatched  = "matched"
	PathTimedOut = "timed_out"
	PathElse     = "else"
	PathFallback = "fallback"
	PathNoDate   = "no_date"
)

/*
Spec is a whole series: Entry says who starts it, Exit what makes them
leave, Goal what success is, Quiet whether email waits for the day in
the person's zone, and Nodes and Edges the steps and the paths between
them.
*/
type Spec struct {
	Entry Entry  `json:"entry"`
	Exit  Exit   `json:"exit"`
	Goal  *Goal  `json:"goal,omitempty"`
	Quiet bool   `json:"quiet_hours,omitempty"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

/*
Entry starts the series for whoever matches Rules or the saved audience
AudienceID, or does the event Event. IncludeExisting lets people who
already match when the series goes live enter too. Reentry says whether
someone can travel it again.
*/
type Entry struct {
	AudienceID      string          `json:"audience_id,omitempty"`
	Rules           json.RawMessage `json:"rules,omitempty"`
	Event           string          `json:"event,omitempty"`
	IncludeExisting bool            `json:"include_existing,omitempty"`
	Reentry         Reentry         `json:"reentry"`
}

/*
Reentry is "once" or "again"; again lets someone who left enter anew
after CooldownDays.
*/
type Reentry struct {
	Kind         string `json:"kind"`
	CooldownDays int    `json:"cooldown_days,omitempty"`
}

/*
Exit ends a journey at once when the person matches Rules, reaches the
goal with OnGoal, answers any series message with OnReply, or leaves
the series' topic with OnUnsubscribe.
*/
type Exit struct {
	Rules         json.RawMessage `json:"rules,omitempty"`
	OnGoal        bool            `json:"on_goal,omitempty"`
	OnReply       bool            `json:"on_reply,omitempty"`
	OnUnsubscribe bool            `json:"on_unsubscribe,omitempty"`
}

/*
Goal is reached by doing Event, or by matching Rules, within Days of
entering.
*/
type Goal struct {
	Event string          `json:"event,omitempty"`
	Rules json.RawMessage `json:"rules,omitempty"`
	Days  int             `json:"days"`
}

/*
NewSpec is the series a new draft starts with: an entry step and
nothing after it.
*/
func NewSpec() Spec {
	return Spec{
		Entry: Entry{Reentry: Reentry{Kind: "once"}},
		Nodes: []Node{{ID: KindEntry, Kind: KindEntry, Position: &Position{}}},
		Edges: []Edge{},
	}
}

/*
IsMessage reports whether a kind of step sends a message.
*/
func IsMessage(kind string) bool {
	switch kind {
	case KindEmail, KindChat, KindPost, KindSlack:
		return true
	}

	return false
}

/*
settingsOf reads a step's settings into the kind's shape.
*/
func settingsOf[T any](n Node) T {
	var out T
	if len(n.Settings) > 0 {
		_ = json.Unmarshal(n.Settings, &out)
	}

	return out
}
