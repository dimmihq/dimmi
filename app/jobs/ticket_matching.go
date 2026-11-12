/*
|--------------------------------------------------------------------------
| Ticket Matching Jobs
|--------------------------------------------------------------------------
|
| After a customer starts a conversation, the AI reads it against the
| incidents active now and the open trackers: a match to an incident
| links it and tells the customer the latest at once, a match to a
| tracker is suggested to the team, or linked when the workspace says
| so.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
MatchTicketConversationArgs names a new conversation to match.
*/
type MatchTicketConversationArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	ConversationID ids.UUID `json:"conversation_id"`
}

/*
Kind names the job.
*/
func (MatchTicketConversationArgs) Kind() string { return "match_ticket_conversation" }

/*
Options keep one job per conversation.
*/
func (MatchTicketConversationArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
ConversationMatcher matches a new conversation to incidents and
trackers.
*/
type ConversationMatcher interface {
	/*
		MatchConversation matches the conversation the args name.
	*/
	MatchConversation(ctx context.Context, args MatchTicketConversationArgs) error
}

/*
MatchTicketConversation matches a new conversation.
*/
type MatchTicketConversation struct {
	Work ConversationMatcher
}

/*
Handle runs the match.
*/
func (w *MatchTicketConversation) Handle(ctx context.Context, job *queue.Job[MatchTicketConversationArgs]) error {
	return w.Work.MatchConversation(ctx, job.Args)
}
