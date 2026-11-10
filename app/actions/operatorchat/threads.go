/*
|--------------------------------------------------------------------------
| Operator Threads
|--------------------------------------------------------------------------
|
| Each teammate talks with Operator in their own threads: as many about
| each conversation and about the whole workspace as they like, the
| latest about a conversation opening first. A thread is private to its
| teammate; a thread about a conversation they can no longer see is not
| found. Questions are counted per teammate, since every one spends real
| tokens.
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
  - Opening a conversation's thread returns the teammate's latest one
    about it; asking for a fresh one starts another.
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
ListFilter narrows a teammate's threads: only those about the whole
workspace, or only those about one conversation.
*/
type ListFilter struct {
	WorkspaceOnly bool
	Conversation  *ids.UUID
}

/*
List returns the teammate's threads with messages, latest first,
narrowed by the filter. Threads about a conversation the teammate can
no longer see are refused with that conversation.
*/
func (a *Threads) List(ctx context.Context, m authz.Membership, filter ListFilter) ([]models.OperatorThread, error) {
	if err := conversations.CanRead(m); err != nil {
		return nil, err
	}
	var out []models.OperatorThread
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if filter.Conversation != nil {
			if _, err := conversations.Find(ctx, q, m, conversations.Ref{ID: *filter.Conversation}); err != nil {
				return err
			}
		}
		var err error
		out, err = q.ListOperatorThreads(ctx, models.ListOperatorThreadsParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, WorkspaceOnly: filter.WorkspaceOnly,
			ConversationID: filter.Conversation, RowLimit: threadsListed,
		})
		return err
	})
	return out, err
}

/*
Open returns the teammate's latest thread about a conversation, starting
one when there is none or when fresh asks for a new one; without a
conversation it starts a thread about the whole workspace.
*/
func (a *Threads) Open(ctx context.Context, m authz.Membership, conversation *ids.UUID, fresh bool) (Thread, error) {
	if err := conversations.CanRead(m); err != nil {
		return Thread{}, err
	}
	var out Thread
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		thread, err := a.thread(ctx, q, m, opening{conversation: conversation, fresh: fresh})
		if err != nil {
			return err
		}
		out, err = load(ctx, q, m, thread)
		return err
	})
	return out, err
}

/*
opening is what Open was asked for: a conversation, nil for the whole
workspace, and whether to start a new thread even when one exists.
*/
type opening struct {
	conversation *ids.UUID
	fresh        bool
}

/*
thread finds the thread Open returns, or starts it.
*/
func (a *Threads) thread(ctx context.Context, q *models.Queries, m authz.Membership, in opening,
) (models.OperatorThread, error) {
	if in.conversation != nil {
		if _, err := conversations.Find(ctx, q, m, conversations.Ref{ID: *in.conversation}); err != nil {
			return models.OperatorThread{}, err
		}
	}
	if in.conversation != nil && !in.fresh {
		latest, err := q.FindConversationOperatorThread(ctx, models.FindConversationOperatorThreadParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ConversationID: in.conversation,
		})
		if err == nil || !errors.Is(err, database.ErrNotFound) {
			return latest, wrapFind(err)
		}
	}
	thread, err := q.CreateOperatorThread(ctx, models.CreateOperatorThreadParams{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ConversationID: in.conversation, Now: a.Clock.Now(),
	})
	if err != nil {
		return thread, fmt.Errorf("open operator thread: %w", err)
	}
	return thread, nil
}

/*
wrapFind says what failed when finding the latest thread did.
*/
func wrapFind(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("find operator thread: %w", err)
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
BriefInput asks for a brief on a conversation: on one of the teammate's
threads about it, or their latest, and kept on the conversation as a
note when Attach says so.
*/
type BriefInput struct {
	Membership   authz.Membership
	Conversation ids.UUID
	Thread       *ids.UUID
	Attach       bool
}

/*
Brief asks Operator to brief the teammate on a conversation, on their
thread about it, and keeps the brief on the conversation when asked.
*/
func (a *Threads) Brief(ctx context.Context, in BriefInput) (Thread, error) {
	thread, err := a.briefThread(ctx, in)
	if err != nil {
		return thread, err
	}
	if err := a.throttle(ctx, in.Membership); err != nil {
		return thread, err
	}
	answer, err := a.Operator.Ask(ctx, operator.Question{
		Membership: in.Membership, ThreadID: thread.Thread.ID, Text: operator.BriefQuestion, Hidden: true,
	})
	if err != nil {
		return thread, err
	}
	if in.Attach {
		if err := a.Operator.NoteBrief(ctx, in.Membership, in.Conversation, answer.Message); err != nil {
			return thread, fmt.Errorf("keep brief: %w", err)
		}
	}
	return a.Show(ctx, in.Membership, thread.Thread.ID)
}

/*
briefThread is the thread a brief goes on: the one named, which must be
about the conversation, or the teammate's latest about it.
*/
func (a *Threads) briefThread(ctx context.Context, in BriefInput) (Thread, error) {
	if in.Thread == nil {
		return a.Open(ctx, in.Membership, &in.Conversation, false)
	}
	thread, err := a.Show(ctx, in.Membership, *in.Thread)
	if err != nil {
		return thread, err
	}
	if thread.Thread.ConversationID == nil || *thread.Thread.ConversationID != in.Conversation {
		return thread, exceptions.ErrNotFound
	}
	return thread, nil
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
