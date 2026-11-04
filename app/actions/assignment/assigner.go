/*
|--------------------------------------------------------------------------
| Assigner
|--------------------------------------------------------------------------
|
| The face automatic assignment shows the queue: assign one
| conversation, drain a workspace, and find the workspaces with anything
| waiting. Jobs only ever reach assignment through it.
|
*/

package assignment

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Assigner runs the assignment actions for the queue's workers.
*/
type Assigner struct {
	Arrival *AssignOnArrival
	Drain   *DrainAssignmentQueue
}

/*
AssignConversation assigns one conversation for the reason.
*/
func (a *Assigner) AssignConversation(
	ctx context.Context, workspaceID, conversationID ids.UUID, reason string,
) error {
	_, err := a.Arrival.Handle(ctx, ArrivalInput{
		WorkspaceID: workspaceID, ConversationID: conversationID, Reason: reason,
	})

	return err
}

/*
DrainWorkspace assigns the workspace's waiting conversations.
*/
func (a *Assigner) DrainWorkspace(ctx context.Context, workspaceID ids.UUID) error {
	_, err := a.Drain.Handle(ctx, workspaceID)

	return err
}

/*
WaitingWorkspaces returns the workspaces with waiting conversations.
*/
func (a *Assigner) WaitingWorkspaces(ctx context.Context, limit int32) ([]ids.UUID, error) {
	return WaitingWorkspaces(ctx, a.Arrival.DB, limit)
}
