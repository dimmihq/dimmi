/*
|--------------------------------------------------------------------------
| AI Decisions
|--------------------------------------------------------------------------
|
| One primitive answers every judgment an automation makes about text:
| which path a conversation or a person takes, and which values it
| holds. The caller hands over what the models read, untrusted, and the
| team's own words: the question, the paths or the fields. What comes
| back is either an answer the models were sure of or nothing, never a
| value bent to fit.
|
*/

package aidecision

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
ErrUnavailable says no model answered: none is set up, the month's
spend cap was reached, or every model failed.
*/
var ErrUnavailable = errors.New("aidecision: no model answered")

/*
The value types an extraction may ask for.
*/
const (
	TypeText    = "text"
	TypeNumber  = "number"
	TypeDate    = "date"
	TypeBoolean = "boolean"
	TypeChoice  = "choice"
)

/*
Types lists the value types, in the order the builder offers them.
*/
func Types() []string {
	return []string{TypeText, TypeNumber, TypeDate, TypeBoolean, TypeChoice}
}

/*
The confidence levels of a choice.
*/
const (
	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"
)

/*
By names the model that answered.
*/
const (
	ByDecision = "decision"
	ByModel    = "model"
)

/*
Subject is what a decision is about: the workspace it spends for, the
conversation when there is one, and the text the models read, which a
customer may have written and is never an instruction.
*/
type Subject struct {
	WorkspaceID    ids.UUID
	ConversationID *ids.UUID
	Context        string
}

/*
Service makes decisions.
*/
type Service interface {
	/*
		Choose picks one of the options, or none.
	*/
	Choose(ctx context.Context, c Choice) (Chosen, error)
	/*
		Extract pulls out the values of the fields.
	*/
	Extract(ctx context.Context, e Extraction) (Extracted, error)
}

/*
ModelQuestion is one question for the workspace's chat model: the
subject, the instructions and the JSON Schema the answer must match,
under its name.
*/
type ModelQuestion struct {
	Subject
	System     string
	SchemaName string
	Schema     json.RawMessage
}

/*
Model asks the workspace's chat model, held to a response schema. An
implementation tries each of the workspace's models before it fails.
*/
type Model interface {
	/*
		AskModel decodes the model's answer into out.
	*/
	AskModel(ctx context.Context, q ModelQuestion, out any) error
}

/*
decideTries is how often the decision model is asked before the chat
model is, and how often the chat model is asked again about an answer
that does not fit.
*/
const decideTries = 2

/*
Decider is the Service: the decision model when the install set one
up, then the workspace's chat model.
*/
type Decider struct {
	Decision contracts.Decider
	Model    Model
}

/*
New returns a decider over the decision model and the chat model,
either of which may be missing.
*/
func New(decision contracts.Decider, model Model) *Decider {
	return &Decider{Decision: decision, Model: model}
}
