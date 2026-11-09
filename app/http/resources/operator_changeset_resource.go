/*
|--------------------------------------------------------------------------
| Operator Changeset Resource
|--------------------------------------------------------------------------
|
| A changeset reads as the card a teammate decides on: its title and
| reason, each operation's label with the value it replaced and what
| became of it, who decided and when, and why it failed when it did. A
| memory reads as what the teammate asked Operator to keep in mind.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorChangeset is one set of changes Operator proposed. ProposedTo
is nil when Operator proposed it on its own.
*/
type OperatorChangeset struct {
	ID             string              `json:"id"`
	Object         string              `json:"object"`
	ThreadID       *string             `json:"thread_id"`
	MessageID      *string             `json:"message_id"`
	ConversationID *string             `json:"conversation_id"`
	ProposedTo     *string             `json:"proposed_to"`
	Title          string              `json:"title"`
	Reason         string              `json:"reason"`
	Status         string              `json:"status"`
	Auto           bool                `json:"auto"`
	Error          *string             `json:"error"`
	DecidedBy      *string             `json:"decided_by"`
	Operations     []OperatorOperation `json:"operations"`
	AppliedAt      *time.Time          `json:"applied_at"`
	RevertedAt     *time.Time          `json:"reverted_at"`
	CreatedAt      time.Time           `json:"created_at"`
}

/*
OperatorOperation is one change inside a changeset. Number is the
conversation's number when it changes one; Created names the note or
article it made; Text is the full text it writes, such as a note, an
article body or a procedure's working copy, for the reviewer to read
before applying.
*/
type OperatorOperation struct {
	Kind    string          `json:"kind"`
	Label   string          `json:"label"`
	Text    string          `json:"text,omitempty"`
	Number  int64           `json:"conversation_number,omitempty"`
	Before  json.RawMessage `json:"before,omitempty"`
	After   json.RawMessage `json:"after,omitempty"`
	Outcome string          `json:"outcome,omitempty"`
	Created *string         `json:"created_id,omitempty"`
}

/*
NewOperatorChangeset shapes a changeset.
*/
func NewOperatorChangeset(c models.OperatorChangeset) OperatorChangeset {
	out := OperatorChangeset{
		ID: models.PublicID(models.PrefixOperatorChangeset, c.ID), Object: "operator_changeset",
		ThreadID:       optionalID(models.PrefixOperatorThread, c.ThreadID),
		MessageID:      optionalID(models.PrefixOperatorMessage, c.MessageID),
		ConversationID: optionalID(models.PrefixConversation, c.ConversationID),
		ProposedTo:     optionalID(models.PrefixMember, c.MemberID),
		DecidedBy:      optionalID(models.PrefixMember, c.DecidedByMemberID),
		Title:          c.Title, Reason: c.Reason, Status: c.Status, Auto: c.Auto, Error: c.Error,
		AppliedAt: c.AppliedAt, RevertedAt: c.RevertedAt, CreatedAt: c.CreatedAt,
	}
	ops := operator.Operations(c.Operations)
	out.Operations = make([]OperatorOperation, len(ops))
	for i, op := range ops {
		out.Operations[i] = OperatorOperation{
			Kind: op.Kind, Label: op.Label, Text: textOf(op), Number: op.Number, Before: op.Before, After: op.After,
			Outcome: op.Outcome, Created: created(op),
		}
	}
	return out
}

/*
NewOperatorChangesets shapes changesets.
*/
func NewOperatorChangesets(rows []models.OperatorChangeset) []OperatorChangeset {
	out := make([]OperatorChangeset, len(rows))
	for i, c := range rows {
		out[i] = NewOperatorChangeset(c)
	}
	return out
}

/*
textOf is the full text an operation writes, empty when its label says
it all.
*/
func textOf(op operator.Operation) string {
	switch {
	case op.Kind == operator.KindNote:
		return deref(op.Value)
	case op.Body != nil:
		return *op.Body
	case len(op.Definition) > 0:
		return string(op.Definition)
	default:
		return ""
	}
}

/*
created names what an operation made, by the kind of thing it is.
*/
func created(op operator.Operation) *string {
	if op.Created == nil {
		return nil
	}
	if op.Kind == operator.KindNote {
		return optionalID(models.PrefixMessage, op.Created)
	}
	return optionalID(models.PrefixArticle, op.Created)
}

/*
OperatorMemory is something the caller asked Operator to remember.
*/
type OperatorMemory struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

/*
NewOperatorMemories shapes memories.
*/
func NewOperatorMemories(rows []models.OperatorMemory) []OperatorMemory {
	out := make([]OperatorMemory, len(rows))
	for i, m := range rows {
		out[i] = OperatorMemory{
			ID: models.PublicID(models.PrefixOperatorMemory, m.ID), Object: "operator_memory", Body: m.Body,
			CreatedAt: m.CreatedAt,
		}
	}
	return out
}
