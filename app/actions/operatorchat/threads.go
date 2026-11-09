/*
|--------------------------------------------------------------------------
| Operator Threads
|--------------------------------------------------------------------------
|
| Each teammate talks with Operator in their own threads: one per
| conversation they open it on, and as many about the whole workspace as
| they like. A thread is private to its teammate; a thread about a
| conversation they can no longer see is not found. Questions are
| counted per teammate, since every one spends real tokens.
|
*/

package operatorchat

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The limits of threads: how many a list shows, how many messages a
thread shows, and how many questions a teammate asks a minute.
*/
const (
	threadsListed  = 100
	messagesShown  = 200
	askedPerMinute = 30
)

/*
Thread is one thread with its messages, oldest first, and the changes
Operator proposed in it.
*/
type Thread struct {
	Thread     models.OperatorThread
	Messages   []models.OperatorMessage
	Changesets []models.OperatorChangeset
}

/*
AskInput is one question on a thread.
*/
type AskInput struct {
	Membership authz.Membership
	ThreadID   ids.UUID
	Body       string
}

/*
Threads lists, opens, shows and deletes a teammate's threads, and asks
Operator on them.

Rules:
  - Every call needs conversations:read; a thread about a conversation
    needs the teammate to see it.
  - Opening a conversation's thread twice returns the same thread.
  - A teammate asks at most 30 questions a minute.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], a [fault]
when Operator cannot answer or the teammate asks too often, or a
wrapped database error.
*/
type Threads struct {
	DB       *database.DB
	Clock    clock.Clock
	Operator *operator.Engine
	Limiter  *ratelimit.Limiter
}

/*
List returns the teammate's threads, latest first, only those about the
whole workspace when asked.
*/
func (a *Threads) List(ctx context.Context, m authz.Membership, workspaceOnly bool) ([]models.OperatorThread, error) {
	if err := conversations.CanRead(m); err != nil {
		return nil, err
	}
	var out []models.OperatorThread
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListOperatorThreads(ctx, models.ListOperatorThreadsParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, WorkspaceOnly: workspaceOnly, RowLimit: threadsListed,
		})
		return err
	})
	return out, err
}

/*
Open returns the teammate's thread about a conversation, starting it
when there is none, or starts a thread about the whole workspace.
*/
func (a *Threads) Open(ctx context.Context, m authz.Membership, conversation *ids.UUID) (Thread, error) {
	if err := conversations.CanRead(m); err != nil {
		return Thread{}, err
	}
	var out Thread
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if conversation != nil {
			if _, err := conversations.Find(ctx, q, m, conversations.Ref{ID: *conversation}); err != nil {
				return err
			}
		}
		thread, err := q.CreateOperatorThread(ctx, models.CreateOperatorThreadParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ConversationID: conversation, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("open operator thread: %w", err)
		}
		out, err = load(ctx, q, m, thread)
		return err
	})
	return out, err
}

/*
Show returns one of the teammate's threads.
*/
func (a *Threads) Show(ctx context.Context, m authz.Membership, id ids.UUID) (Thread, error) {
	if err := conversations.CanRead(m); err != nil {
		return Thread{}, err
	}
	var out Thread
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		thread, err := find(ctx, q, m, id)
		if err != nil {
			return err
		}
		out, err = load(ctx, q, m, thread)
		return err
	})
	return out, err
}

/*
Delete removes one of the teammate's threads with its messages.
*/
func (a *Threads) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if err := conversations.CanRead(m); err != nil {
		return err
	}
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		n, err := models.New(tx).DeleteOperatorThread(ctx, models.DeleteOperatorThreadParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("delete operator thread: %w", err)
		}
		if n == 0 {
			return exceptions.ErrNotFound
		}
		return nil
	})
}

/*
Ask asks Operator on one of the teammate's threads.
*/
func (a *Threads) Ask(ctx context.Context, in AskInput) (operator.Answer, error) {
	if err := conversations.CanRead(in.Membership); err != nil {
		return operator.Answer{}, err
	}
	if err := a.throttle(ctx, in.Membership); err != nil {
		return operator.Answer{}, err
	}
	return a.Operator.Ask(ctx, operator.Question{Membership: in.Membership, ThreadID: in.ThreadID, Text: in.Body})
}

/*
Brief asks Operator to brief the teammate on a conversation, on their
thread about it.
*/
func (a *Threads) Brief(ctx context.Context, m authz.Membership, conversation ids.UUID) (Thread, error) {
	thread, err := a.Open(ctx, m, &conversation)
	if err != nil {
		return thread, err
	}
	if err := a.throttle(ctx, m); err != nil {
		return thread, err
	}
	_, err = a.Operator.Ask(ctx, operator.Question{
		Membership: m, ThreadID: thread.Thread.ID, Text: operator.BriefQuestion, Hidden: true,
	})
	if err != nil {
		return thread, err
	}
	return a.Show(ctx, m, thread.Thread.ID)
}

/*
throttle counts a question against the teammate's minute.
*/
func (a *Threads) throttle(ctx context.Context, m authz.Membership) error {
	if a.Limiter == nil {
		return nil
	}
	return auth.Throttle(ctx, a.Limiter, auth.Rate{
		Key: "operator:" + m.Member.ID.String(), Limit: askedPerMinute, Window: time.Minute,
	})
}

/*
find returns one of the teammate's threads, refusing one about a
conversation they can no longer see.
*/
func find(ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID) (models.OperatorThread, error) {
	thread, err := q.FindOperatorThread(ctx, models.FindOperatorThreadParams{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return thread, exceptions.ErrNotFound
	}
	if err != nil {
		return thread, fmt.Errorf("find operator thread: %w", err)
	}
	if thread.ConversationID != nil {
		_, err = conversations.Find(ctx, q, m, conversations.Ref{ID: *thread.ConversationID})
	}
	return thread, err
}

/*
load reads a thread's messages, oldest first, and its changesets; a
thread about a conversation also shows what Operator proposed on its
own about that conversation.
*/
func load(ctx context.Context, q *models.Queries, m authz.Membership, thread models.OperatorThread) (Thread, error) {
	out := Thread{Thread: thread}
	messages, err := q.ListOperatorMessages(ctx, models.ListOperatorMessagesParams{
		WorkspaceID: m.Workspace.ID, ThreadID: thread.ID, RowLimit: messagesShown,
	})
	if err != nil {
		return out, fmt.Errorf("list operator messages: %w", err)
	}
	slices.Reverse(messages)
	out.Messages = messages
	out.Changesets, err = q.ListOperatorChangesets(ctx, changesetParams(m, changesetFilter{thread: &thread.ID}))
	if err != nil {
		return out, fmt.Errorf("list operator changesets: %w", err)
	}
	if thread.ConversationID == nil {
		return out, nil
	}
	filter := changesetFilter{conversation: thread.ConversationID}
	about, err := q.ListOperatorChangesets(ctx, changesetParams(m, filter))
	if err != nil {
		return out, fmt.Errorf("list operator changesets: %w", err)
	}
	for _, c := range about {
		if c.MemberID == nil {
			out.Changesets = append(out.Changesets, c)
		}
	}
	return out, nil
}
