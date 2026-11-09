/*
|--------------------------------------------------------------------------
| Operator Resources
|--------------------------------------------------------------------------
|
| How Operator reads over the API: its settings with every group's
| policy spelled out, threads with their messages and the steps behind
| each answer, and changesets with each operation's label, the value it
| replaced and what became of it.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorSettings is the workspace's Operator settings, with the policy
of every group, and which groups may apply on their own.
*/
type OperatorSettings struct {
	Object          string            `json:"object"`
	Enabled         bool              `json:"enabled"`
	BriefOnOpen     bool              `json:"brief_on_open"`
	TriageOnArrival bool              `json:"triage_on_arrival"`
	Instructions    string            `json:"instructions"`
	Policies        map[string]string `json:"policies"`
	AutoGroups      []string          `json:"auto_groups"`
	Model           *string           `json:"model"`
}

/*
NewOperatorSettings shapes the settings.
*/
func NewOperatorSettings(s operator.Settings) OperatorSettings {
	out := OperatorSettings{
		Object: "operator_settings", Enabled: s.Enabled, BriefOnOpen: s.BriefOnOpen,
		TriageOnArrival: s.TriageOnArrival, Instructions: s.Instructions, Policies: map[string]string{},
		AutoGroups: []string{}, Model: optionalText(s.Model),
	}
	for _, g := range operator.Groups() {
		out.Policies[g] = string(s.PolicyFor(g))
		if operator.CanAuto(g) {
			out.AutoGroups = append(out.AutoGroups, g)
		}
	}
	return out
}

/*
OperatorThread is one of the caller's threads with Operator.
*/
type OperatorThread struct {
	ID             string     `json:"id"`
	Object         string     `json:"object"`
	ConversationID *string    `json:"conversation_id"`
	Title          string     `json:"title"`
	LastMessageAt  *time.Time `json:"last_message_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

/*
NewOperatorThread shapes a thread.
*/
func NewOperatorThread(t models.OperatorThread) OperatorThread {
	return OperatorThread{
		ID: models.PublicID(models.PrefixOperatorThread, t.ID), Object: "operator_thread",
		ConversationID: optionalID(models.PrefixConversation, t.ConversationID), Title: t.Title,
		LastMessageAt: t.LastMessageAt, CreatedAt: t.CreatedAt,
	}
}

/*
NewOperatorThreads shapes threads.
*/
func NewOperatorThreads(rows []models.OperatorThread) []OperatorThread {
	out := make([]OperatorThread, len(rows))
	for i, t := range rows {
		out[i] = NewOperatorThread(t)
	}
	return out
}

/*
OperatorThreadView is a thread with its messages, oldest first, and the
changes Operator proposed in it.
*/
type OperatorThreadView struct {
	OperatorThread
	Messages   []OperatorMessage   `json:"messages"`
	Changesets []OperatorChangeset `json:"changesets"`
}

/*
NewOperatorThreadView shapes a thread with what it holds.
*/
func NewOperatorThreadView(t models.OperatorThread, messages []models.OperatorMessage,
	changesets []models.OperatorChangeset,
) OperatorThreadView {
	out := OperatorThreadView{OperatorThread: NewOperatorThread(t), Messages: make([]OperatorMessage, len(messages))}
	for i, m := range messages {
		out.Messages[i] = NewOperatorMessage(m)
	}
	out.Changesets = NewOperatorChangesets(changesets)
	return out
}

/*
OperatorMessage is one turn of a thread: who said it, what, the steps
Operator took to answer, and how its run ended.
*/
type OperatorMessage struct {
	ID        string          `json:"id"`
	Object    string          `json:"object"`
	Role      string          `json:"role"`
	Body      string          `json:"body"`
	Steps     json.RawMessage `json:"steps"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
}

/*
NewOperatorMessage shapes a message.
*/
func NewOperatorMessage(m models.OperatorMessage) OperatorMessage {
	return OperatorMessage{
		ID: models.PublicID(models.PrefixOperatorMessage, m.ID), Object: "operator_message", Role: m.Role,
		Body: m.Body, Steps: m.Steps, Status: m.Status, CreatedAt: m.CreatedAt,
	}
}

/*
OperatorAnswer is one turn as stored: the caller's question, absent
when it was asked on their behalf, Operator's answer and its proposals.
*/
type OperatorAnswer struct {
	Asked      *OperatorMessage    `json:"asked"`
	Message    OperatorMessage     `json:"message"`
	Changesets []OperatorChangeset `json:"changesets"`
}

/*
NewOperatorAnswer shapes an answer.
*/
func NewOperatorAnswer(a operator.Answer) OperatorAnswer {
	out := OperatorAnswer{Message: NewOperatorMessage(a.Message), Changesets: NewOperatorChangesets(a.Changesets)}
	if a.Asked != nil {
		asked := NewOperatorMessage(*a.Asked)
		out.Asked = &asked
	}
	return out
}

/*
optionalText is nil for an empty text.
*/
func optionalText(text string) *string {
	if text == "" {
		return nil
	}
	return &text
}

/*
optionalID writes an optional row id as a public id.
*/
func optionalID(prefix models.Prefix, id *ids.UUID) *string {
	if id == nil {
		return nil
	}
	return new(models.PublicID(prefix, *id))
}
