/*
|--------------------------------------------------------------------------
| Operator Engine
|--------------------------------------------------------------------------
|
| One question from a teammate, one answer from Operator. Like the
| customer agent, a turn happens in three moves so no transaction ever
| waits on a model: a short transaction checks Operator may answer,
| stores the question and opens the run; the model and its tools work
| outside any transaction while the text and steps stream to the asker;
| a last transaction stores the answer, ties it to the changes it
| proposed and counts what it cost.
|
| Operator acts as the teammate who asked: every tool goes through the
| action the API calls, with their role and what they may see. The
| answer is stored even when the teammate stops it or the model fails,
| so the thread never loses what happened.
|
*/

package operator

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aiprices"
	"github.com/dimmihq/dimmi/config"
)

/*
Services are the actions Operator's tools read and change the workspace
through, each checking the asking teammate's permissions.
*/
type Services struct {
	Knowledge  contracts.Knowledge
	Show       *conversations.ShowConversation
	Search     *conversations.Search
	List       *conversations.ListConversations
	Context    *conversations.ShowContext
	Messages   *messages.ListMessages
	Runs       *ai.ListConversationRuns
	Change     *conversations.ApplyConversationChanges
	Note       *messages.AddNote
	Delete     *messages.DeleteMessage
	Contacts   *contacts.UpdateContact
	Articles   *helpcenters.Articles
	Procedures *procedures.Procedures
	Audit      *audit.RecordAuditLog
	Actions    *actionkit.Kit
}

/*
Engine runs Operator. Publisher streams each run to the teammate who
asked; IDs reads and writes the public ids tools speak in;
StorePayloads keeps request and response text on the run's steps.
*/
type Engine struct {
	DB            *database.DB
	IDs           *ids.Registry
	Clock         clock.Clock
	LLM           *managers.LLMManager
	Prices        *aiprices.Table
	Publisher     appbroadcasting.Publisher
	Timeouts      config.AITimeoutsConfig
	Services      Services
	StorePayloads bool
}

/*
Question is one turn from a teammate on one of their threads. Hidden
questions, such as the brief Operator writes when a conversation
opens, are sent to the model but never shown as the teammate's words.
*/
type Question struct {
	Membership authz.Membership
	ThreadID   ids.UUID
	Text       string
	Hidden     bool
}

/*
Answer is the turn as stored: the teammate's message, nil when the
question was hidden, Operator's reply, and the changes it proposed.
*/
type Answer struct {
	Asked      *models.OperatorMessage
	Message    models.OperatorMessage
	Changesets []models.OperatorChangeset
}

/*
The reasons Operator cannot answer. Each is the workspace's state, not
a failure of the server, so they are conflicts the teammate can fix.
*/
var (
	ErrDisabled = fault.New(fault.Conflict, "operator_disabled", "Operator is switched off for this workspace.")
	ErrNoModel  = fault.New(fault.Conflict, "operator_no_model",
		"Operator needs an AI connection. Connect one in Settings, AI.")
	ErrCapped = fault.New(fault.Conflict, "ai_spend_capped",
		"This month's AI spend reached its cap, so Operator is paused until the cap is raised.")
)

/*
Ask answers one question. The teammate stopping the request stops the
model, and what it wrote so far is kept.
*/
func (e *Engine) Ask(ctx context.Context, q Question) (Answer, error) {
	e.prepare()
	s, asked, err := e.open(ctx, q)
	if err != nil {
		return Answer{}, err
	}
	text, status := e.converse(ctx, s)
	answer, err := e.finish(context.WithoutCancel(ctx), s, text, status)
	answer.Asked = asked
	return answer, err
}

/*
prepare fills the defaults a zero-configured engine lacks.
*/
func (e *Engine) prepare() {
	if e.Clock == nil {
		e.Clock = clock.System()
	}
}

/*
statusOf names how a run ended from the error that ended it.
*/
func statusOf(err error) string {
	switch {
	case err == nil:
		return "done"
	case errors.Is(err, context.Canceled):
		return "stopped"
	default:
		return "failed"
	}
}
