/*
|--------------------------------------------------------------------------
| Opening A Turn
|--------------------------------------------------------------------------
|
| Before any model is called, one short transaction makes sure Operator
| may answer: the thread is the teammate's, a thread about a
| conversation is about one they can still see, Operator is on, the
| workspace has a model to answer with and the month's AI budget is not
| spent. It stores the question, reads the thread's recent turns and the
| teammate's memories, and opens the run that will carry the cost.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
historySize is how many of a thread's latest turns the model reads.
*/
const historySize = 30

/*
memorySize is how many of a teammate's memories every run reads.
*/
const memorySize = 50

/*
session is one run's state: who asked on which thread, the settings and
model it runs with, what the thread said before, and what the run has
done so far. Tainted turns true once the run read text a customer
wrote; from then on no change it proposes applies on its own.
Background marks a run Operator started on its own, with no thread and
no teammate waiting.
*/
type session struct {
	m            authz.Membership
	thread       models.OperatorThread
	conversation *models.Conversation
	settings     Settings
	agent        models.AiAgent
	key          models.AiProviderKey
	model        string
	llm          contracts.LLM
	history      []models.OperatorMessage
	memories     []models.OperatorMemory
	question     string
	askerName    string
	run          models.AiRun
	tally        tally
	steps        []Step
	proposed     []models.OperatorChangeset
	actions      []actionkit.Live
	identity     actionkit.Identity
	tainted      bool
	background   bool
	failure      error
	toolCalls    int
}

/*
open checks Operator may answer, stores the question and opens the run.
*/
func (e *Engine) open(ctx context.Context, q Question) (*session, *models.OperatorMessage, error) {
	text := strings.TrimSpace(q.Text)
	if text == "" {
		return nil, nil, fault.New(fault.Invalid, "message_required", "Ask Operator something.").OnField("body")
	}
	s := &session{m: q.Membership, question: text}
	var asked *models.OperatorMessage
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		mq := models.New(tx)
		if err := e.load(ctx, mq, s, q); err != nil {
			return err
		}
		if !q.Hidden {
			msg, err := e.store(ctx, mq, s, said{role: roleUser, body: text, status: "done"})
			if err != nil {
				return err
			}
			asked = &msg
		}
		run, err := e.startRun(ctx, mq, s)
		s.run = run
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	llm, err := e.LLM.ForKey(ctx, s.key)
	if err != nil {
		return nil, nil, fmt.Errorf("open operator model: %w", err)
	}
	s.llm = llm
	e.offerActions(ctx, s)
	return s, asked, nil
}

/*
load reads the thread, the settings, the model and the memory a run
needs, refusing when Operator may not answer.
*/
func (e *Engine) load(ctx context.Context, q *models.Queries, s *session, in Question) error {
	ws := s.m.Workspace.ID
	thread, err := q.FindOperatorThread(ctx, models.FindOperatorThreadParams{
		WorkspaceID: ws, MemberID: s.m.Member.ID, ID: in.ThreadID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("find operator thread: %w", err)
	}
	s.thread = thread
	if err := e.loadConversation(ctx, q, s); err != nil {
		return err
	}
	if s.settings, err = LoadSettings(ctx, q, ws); err != nil {
		return err
	}
	if !s.settings.Enabled {
		return ErrDisabled
	}
	if err := e.loadModel(ctx, q, s); err != nil {
		return err
	}
	asker, err := q.FindAuditActor(ctx, models.FindAuditActorParams{WorkspaceID: ws, ID: s.m.Member.ID})
	if err == nil {
		s.askerName = asker.Name
	}
	return e.loadMemory(ctx, q, s)
}

/*
loadConversation reads the conversation a thread is about, refusing it
when the teammate can no longer see it.
*/
func (e *Engine) loadConversation(ctx context.Context, q *models.Queries, s *session) error {
	if s.thread.ConversationID == nil {
		return nil
	}
	c, err := conversations.Find(ctx, q, s.m, conversations.Ref{ID: *s.thread.ConversationID})
	if err != nil {
		return err
	}
	s.conversation = &c
	return nil
}

/*
loadModel reads the agent's answer connection and model, and refuses
when there is none or the month's budget is spent.
*/
func (e *Engine) loadModel(ctx context.Context, q *models.Queries, s *session) error {
	agent, err := q.FindAgent(ctx, s.m.Workspace.ID)
	if errors.Is(err, database.ErrNotFound) {
		return ErrNoModel
	}
	if err != nil {
		return fmt.Errorf("find agent: %w", err)
	}
	if agent.AnswerKeyID == nil || agent.AnswerModel == nil || *agent.AnswerModel == "" {
		return ErrNoModel
	}
	key, err := q.FindProviderKey(ctx, models.FindProviderKeyParams{
		WorkspaceID: agent.WorkspaceID, ID: *agent.AnswerKeyID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return ErrNoModel
	}
	if err != nil {
		return fmt.Errorf("find provider key: %w", err)
	}
	capped, err := pipeline.SpendCapped(ctx, q, e.Clock.Now(), agent)
	if err != nil {
		return err
	}
	if capped {
		return ErrCapped
	}
	s.agent, s.key, s.model = agent, key, modelFor(s.settings, key, *agent.AnswerModel)
	return nil
}

/*
loadMemory reads the thread's latest turns, oldest first, and what the
teammate asked Operator to remember.
*/
func (e *Engine) loadMemory(ctx context.Context, q *models.Queries, s *session) error {
	history, err := q.ListOperatorMessages(ctx, models.ListOperatorMessagesParams{
		WorkspaceID: s.m.Workspace.ID, ThreadID: s.thread.ID, RowLimit: historySize,
	})
	if err != nil {
		return fmt.Errorf("list operator messages: %w", err)
	}
	slices.Reverse(history)
	s.history = history
	s.memories, err = q.ListOperatorMemories(ctx, models.ListOperatorMemoriesParams{
		WorkspaceID: s.m.Workspace.ID, MemberID: s.m.Member.ID, RowLimit: memorySize,
	})
	if err != nil {
		return fmt.Errorf("list operator memories: %w", err)
	}
	return nil
}

/*
The roles of a thread's messages.
*/
const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

/*
said is one message to store: who said it, what, the steps that led to
it and how its run ended. Run names the run that wrote an answer.
*/
type said struct {
	role, body, status string
	steps              []Step
	run                *ids.UUID
}

/*
store writes one message on the session's thread and marks the thread
as touched, naming it after the first question.
*/
func (e *Engine) store(ctx context.Context, q *models.Queries, s *session, m said) (models.OperatorMessage, error) {
	now := e.Clock.Now()
	encoded, _ := json.Marshal(stepsOrEmpty(m.steps))
	params := models.CreateOperatorMessageParams{
		WorkspaceID: s.m.Workspace.ID, ThreadID: s.thread.ID, Role: m.role, Body: m.body, Steps: encoded,
		Status: m.status, RunID: m.run, Now: now,
	}
	msg, err := q.CreateOperatorMessage(ctx, params)
	if err != nil {
		return msg, fmt.Errorf("store operator message: %w", err)
	}
	err = q.TouchOperatorThread(ctx, models.TouchOperatorThreadParams{
		WorkspaceID: s.m.Workspace.ID, ID: s.thread.ID, Now: now, Title: titleOf(s.question),
	})
	if err != nil {
		return msg, fmt.Errorf("touch operator thread: %w", err)
	}
	return msg, nil
}

/*
titleOf names a thread after its first question, or Brief when it began
with a brief.
*/
func titleOf(question string) string {
	if strings.TrimSpace(question) == strings.TrimSpace(BriefQuestion) {
		return briefTitle
	}
	line, _, _ := strings.Cut(strings.TrimSpace(question), "\n")
	runes := []rune(line)
	if len(runes) > 80 {
		return strings.TrimSpace(string(runes[:79])) + "…"
	}
	return line
}

/*
briefTitle names a thread that began with a brief, since the question
behind a brief is never shown.
*/
const briefTitle = "Brief"

/*
stepsOrEmpty never stores a null list of steps.
*/
func stepsOrEmpty(steps []Step) []Step {
	if steps == nil {
		return []Step{}
	}
	return steps
}
