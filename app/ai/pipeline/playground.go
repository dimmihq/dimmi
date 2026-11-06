/*
|--------------------------------------------------------------------------
| Playground
|--------------------------------------------------------------------------
|
| A teammate asks the agent a question the way a customer would and sees
| everything it did. The same stages run — fast path, classify, route,
| retrieve, generate, post-process — but nothing is committed to a
| conversation: no message, no state change. The run is recorded as a
| playground run and its spend counts toward the workspace's cap.
|
*/

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
playgroundKind is the run kind of a playground question.
*/
const playgroundKind = "playground"

/*
ErrNoAgent marks a workspace without an agent to ask.
*/
var ErrNoAgent = errors.New("pipeline: workspace has no agent")

/*
Ask answers a test question with the full pipeline and writes nothing a
customer sees.
*/
func (r *Runner) Ask(ctx context.Context, q contracts.PlaygroundQuestion) (*contracts.PlaygroundAnswer, error) {
	r.prepare()

	var st *run
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		e, err := r.loadPlayground(ctx, tx, q)
		if err != nil {
			return err
		}
		row, err := r.createRun(ctx, tx, e, Args{
			WorkspaceID: q.WorkspaceID, Kind: playgroundKind, MemberID: &q.MemberID,
		})
		if err != nil {
			return err
		}
		st = &run{env: e, rec: r.newRecorder(row), args: Args{Kind: playgroundKind}}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("playground open: %w", err)
	}

	d := failed("provider_unavailable", errors.New("no answer model"))
	if pauseGate(st.env) != "provider_unavailable" {
		d = r.decide(ctx, st)
	}

	err = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if err := r.finishRun(ctx, tx, st, d); err != nil {
			return err
		}
		return r.recordSpend(ctx, tx, st, d)
	})
	if err != nil {
		return nil, fmt.Errorf("playground finish: %w", err)
	}
	return r.playgroundAnswer(st, d), nil
}

/*
loadPlayground loads the workspace, its agent and connections, and the
named conversation's turns as history; the question is the only
pending customer message.
*/
func (r *Runner) loadPlayground(ctx context.Context, tx *database.Tx, q contracts.PlaygroundQuestion) (*env, error) {
	m := models.New(tx)
	agent, err := m.FindAgent(ctx, q.WorkspaceID)
	if errors.Is(err, database.ErrNotFound) {
		return nil, ErrNoAgent
	}
	if err != nil {
		return nil, fmt.Errorf("find agent: %w", err)
	}
	ws, err := m.FindWorkspace(ctx, q.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("find workspace: %w", err)
	}

	e := &env{
		Workspace: ws, Agent: agent, ChannelType: playgroundKind, now: r.Clock.Now(),
		Conversation: models.Conversation{WorkspaceID: ws.ID},
	}
	e.Pending = []models.Message{{
		AuthorType: string(enums.AuthorTypeContact), Type: string(enums.MessageTypeMessage), Body: &q.Message,
	}}
	if q.ConversationID != nil {
		latest, err := m.ListLatestPublicMessages(ctx, models.ListLatestPublicMessagesParams{
			WorkspaceID: ws.ID, ConversationID: *q.ConversationID, Now: &e.now, RowLimit: transcriptSize,
		})
		if err != nil {
			return nil, fmt.Errorf("list messages: %w", err)
		}
		slices.Reverse(latest)
		e.History = latest
	}

	e.AnswerKey = keyFor(ctx, m, ws.ID, agent.AnswerKeyID)
	e.Version = strv(agent.PromptVersion)
	if r.Prompts != nil {
		e.Version = r.Prompts.Version(e.Version)
		e.Defaults = r.Prompts.Defaults(e.Version)
	}
	online, err := m.CountOnlineMembers(ctx, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("count online members: %w", err)
	}
	e.Facts.onlineMembers = online
	return e, nil
}

/*
playgroundAnswer reports what the agent would have done.
*/
func (r *Runner) playgroundAnswer(st *run, d *decision) *contracts.PlaygroundAnswer {
	rec := st.rec
	out := &contracts.PlaygroundAnswer{
		RunID: &rec.Run.ID, Outcome: d.outcome,
		Classification: contracts.PlaygroundClassification{
			Intent: st.c.Intent, Language: replyLanguage(st.env, st.c), Query: st.query,
		},
		InputTokens: rec.tokens.input, CachedInputTokens: rec.tokens.cached, OutputTokens: rec.tokens.output,
		CostMicros: rec.costMicros, FirstTokenMs: int(rec.firstToken.Milliseconds()),
		LatencyMs: int(r.Clock.Now().Sub(derefTime(rec.Run.StartedAt, r.Clock.Now())).Milliseconds()),
	}
	if d.writesMessage() {
		out.Answer = r.reply(st.env, d)
	}

	cited := map[int]bool{}
	for _, c := range d.citations {
		cited[c.N] = true
		out.Citations = append(out.Citations, contracts.PlaygroundCitation{N: c.N, Title: c.Title, URL: c.URL})
	}
	for _, s := range st.sources {
		out.Retrieved = append(out.Retrieved, contracts.PlaygroundSource{
			N: s.n, Title: s.chunk.Title, URL: s.chunk.URL, Similarity: similarityOf(s.chunk.Similarity),
			Used: cited[s.n], Text: s.chunk.Text,
		})
	}
	return out
}

/*
runSubject is who and what a run row names: the conversation and
trigger of a conversation run, nothing for a playground question.
*/
func runSubject(e *env, kind string) (*ids.UUID, *ids.UUID, *ids.UUID) {
	if kind == playgroundKind {
		return nil, nil, nil
	}
	return &e.Conversation.ID, &e.Trigger.ID, &e.Conversation.ContactID
}
