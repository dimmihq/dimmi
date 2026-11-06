/*
|--------------------------------------------------------------------------
| Ask Playground
|--------------------------------------------------------------------------
|
| A teammate tries the agent before customers do: a question goes
| through the whole pipeline and comes back with how it was read, what
| was found, the answer, its citations, tokens, cost and timing. No
| message is written and no conversation changes, even when the
| question borrows a conversation's history. Each member may ask a
| few dozen questions a minute, since every one spends real tokens.
|
*/

package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
playgroundPerMinute is how many questions one member may ask a minute.
*/
const playgroundPerMinute = 30

/*
PlaygroundInput is one test question, optionally reading a
conversation's history.
*/
type PlaygroundInput struct {
	Membership     authz.Membership
	Message        string
	ConversationID *ids.UUID
}

/*
AskPlayground runs a test question through the agent.

Rules:
  - The caller needs ai:write; a named conversation must be one the
    caller can see.
  - The question is not empty; each member asks at most 30 a minute.
  - It works while the agent is switched off, so it can be tried first.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], a
[fault] when the question is empty, the playground is unavailable or
the member asks too often, or whatever the pipeline returns.
*/
type AskPlayground struct {
	DB         *database.DB
	Playground contracts.AIPlayground
	Limiter    *ratelimit.Limiter
}

/*
Handle asks the question and returns what the agent would do.
*/
func (a *AskPlayground) Handle(ctx context.Context, in PlaygroundInput) (*contracts.PlaygroundAnswer, error) {
	if err := RequirePlayground(in.Membership); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Message) == "" {
		return nil, fault.New(fault.Invalid, "message_required", "a question is required").OnField("message")
	}
	if a.Playground == nil {
		return nil, fault.New(fault.Unavailable, "playground_unavailable", "the playground is not available yet")
	}
	if err := a.throttle(ctx, in.Membership); err != nil {
		return nil, err
	}
	if err := a.checkConversation(ctx, in); err != nil {
		return nil, err
	}

	answer, err := a.Playground.Ask(ctx, contracts.PlaygroundQuestion{
		WorkspaceID: in.Membership.Workspace.ID, MemberID: in.Membership.Member.ID,
		Message: strings.TrimSpace(in.Message), ConversationID: in.ConversationID,
	})
	if err != nil {
		return nil, fmt.Errorf("ask playground: %w", err)
	}

	return answer, nil
}

/*
throttle counts the question against the member's minute.
*/
func (a *AskPlayground) throttle(ctx context.Context, m authz.Membership) error {
	if a.Limiter == nil {
		return nil
	}

	return auth.Throttle(ctx, a.Limiter, auth.Rate{
		Key: "ai-playground:" + m.Member.ID.String(), Limit: playgroundPerMinute, Window: time.Minute,
	})
}

/*
checkConversation refuses a conversation the caller cannot see.
*/
func (a *AskPlayground) checkConversation(ctx context.Context, in PlaygroundInput) error {
	if in.ConversationID == nil {
		return nil
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := conversations.Find(ctx, models.New(tx), in.Membership, conversations.Ref{ID: *in.ConversationID})
		return err
	})
}
