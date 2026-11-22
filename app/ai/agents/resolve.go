/*
|--------------------------------------------------------------------------
| Agent Resolution
|--------------------------------------------------------------------------
|
| Which agent answers a conversation, in one fixed order: the agent a
| workflow step names; the agent already on the conversation, so a
| customer keeps talking to the same one; the agent its Slack channel,
| channel connection or inbox names; and last the workspace's default.
| The first agent that answers a conversation is pinned to it.
|
*/

package agents

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrNoAgent says the workspace has no agent at all yet.
*/
var ErrNoAgent = errors.New("agents: workspace has no agent")

/*
Where is what a conversation offers for picking its agent. Every field
may be empty; WorkspaceID may not.
*/
type Where struct {
	WorkspaceID         ids.UUID
	WorkflowAgentID     *ids.UUID
	ConversationAgentID *ids.UUID
	ChannelRoomID       *ids.UUID
	ChannelConnectionID *ids.UUID
	InboxID             *ids.UUID
}

/*
ForConversation is the Where of a stored conversation, with the agent
a workflow step names, if any.
*/
func ForConversation(c models.Conversation, workflowAgentID *ids.UUID) Where {
	conn, inbox := c.ChannelConnectionID, c.InboxID
	return Where{
		WorkspaceID: c.WorkspaceID, WorkflowAgentID: workflowAgentID, ConversationAgentID: c.AiAgentID,
		ChannelConnectionID: &conn, InboxID: &inbox,
	}
}

/*
Resolve returns the agent that answers, by the fixed order.

Errors: [ErrNoAgent] when the workspace has no agent, or a wrapped
database error.
*/
func Resolve(ctx context.Context, q *models.Queries, w Where) (models.AiAgent, error) {
	row, err := q.ResolveAgent(ctx, models.ResolveAgentParams{
		WorkspaceID: w.WorkspaceID, WorkflowAgentID: w.WorkflowAgentID, ConversationAgentID: w.ConversationAgentID,
		ChannelRoomID: w.ChannelRoomID, ChannelConnectionID: w.ChannelConnectionID, InboxID: w.InboxID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return models.AiAgent{}, ErrNoAgent
	}
	if err != nil {
		return models.AiAgent{}, fmt.Errorf("resolve agent: %w", err)
	}
	return row.AiAgent, nil
}

/*
Default returns the workspace's default agent.

Errors: [ErrNoAgent], or a wrapped database error.
*/
func Default(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (models.AiAgent, error) {
	agent, err := q.FindDefaultAgent(ctx, workspaceID)
	if errors.Is(err, database.ErrNotFound) {
		return models.AiAgent{}, ErrNoAgent
	}
	if err != nil {
		return models.AiAgent{}, fmt.Errorf("find default agent: %w", err)
	}
	return agent, nil
}

/*
Pin puts the agent on a conversation that has none yet.
*/
func Pin(ctx context.Context, q *models.Queries, c models.Conversation, agentID ids.UUID) error {
	if err := q.PinConversationAgent(ctx, models.PinConversationAgentParams{
		WorkspaceID: c.WorkspaceID, ID: c.ID, AgentID: &agentID,
	}); err != nil {
		return fmt.Errorf("pin agent: %w", err)
	}
	return nil
}

/*
Placements returns where each agent of the workspace takes new
conversations, by agent.
*/
func Placements(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (map[ids.UUID]Placement, error) {
	rows, err := q.ListAgentPlacements(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list agent placements: %w", err)
	}
	out := map[ids.UUID]Placement{}
	for _, r := range rows {
		p := out[r.AgentID]
		switch r.Kind {
		case "connection":
			p.ChannelConnectionIDs = append(p.ChannelConnectionIDs, r.ID)
		case "room":
			p.ChannelRoomIDs = append(p.ChannelRoomIDs, r.ID)
		case "inbox":
			p.InboxIDs = append(p.InboxIDs, r.ID)
		}
		out[r.AgentID] = p
	}
	return out, nil
}

/*
Place writes where an agent takes new conversations: it comes off
every row it was on, then onto the ones it names, taking those from
whichever agent had them.
*/
func Place(ctx context.Context, q *models.Queries, agent models.AiAgent, p Placement) error {
	workspaceID := agent.WorkspaceID
	if err := Unplace(ctx, q, workspaceID, agent.ID); err != nil {
		return err
	}
	id := &agent.ID
	if err := q.PlaceAgentOnConnections(ctx, models.PlaceAgentOnConnectionsParams{
		WorkspaceID: workspaceID, AgentID: id, Ids: nonNilIDs(p.ChannelConnectionIDs),
	}); err != nil {
		return fmt.Errorf("place agent on connections: %w", err)
	}
	if err := q.PlaceAgentOnRooms(ctx, models.PlaceAgentOnRoomsParams{
		WorkspaceID: workspaceID, AgentID: id, Ids: nonNilIDs(p.ChannelRoomIDs),
	}); err != nil {
		return fmt.Errorf("place agent on rooms: %w", err)
	}
	if err := q.PlaceAgentOnInboxes(ctx, models.PlaceAgentOnInboxesParams{
		WorkspaceID: workspaceID, AgentID: id, Ids: nonNilIDs(p.InboxIDs),
	}); err != nil {
		return fmt.Errorf("place agent on inboxes: %w", err)
	}
	return nil
}

/*
Unplace takes an agent off every channel connection, Slack channel and
inbox.
*/
func Unplace(ctx context.Context, q *models.Queries, workspaceID, agentID ids.UUID) error {
	id := &agentID
	err := q.ClearAgentConnections(ctx, models.ClearAgentConnectionsParams{WorkspaceID: workspaceID, AgentID: id})
	if err != nil {
		return fmt.Errorf("clear agent connections: %w", err)
	}
	err = q.ClearAgentRooms(ctx, models.ClearAgentRoomsParams{WorkspaceID: workspaceID, AgentID: id})
	if err != nil {
		return fmt.Errorf("clear agent rooms: %w", err)
	}
	err = q.ClearAgentInboxes(ctx, models.ClearAgentInboxesParams{WorkspaceID: workspaceID, AgentID: id})
	if err != nil {
		return fmt.Errorf("clear agent inboxes: %w", err)
	}
	return nil
}

/*
AnswersOn reports whether an agent answers a conversation on a channel:
it must be on. An agent placed on the channel, or named by a workflow
step, answers there; the default agent only on the channel types it
ticked and not on a connection it left out.
*/
func AnswersOn(agent models.AiAgent, channelType string, connectionID ids.UUID) bool {
	if !agent.Enabled {
		return false
	}
	if !agent.IsDefault {
		return true
	}
	open := len(agent.EnabledChannels) == 0 || slices.Contains(agent.EnabledChannels, channelType)
	return open && !slices.Contains(agent.ExcludedChannelConnectionIds, connectionID)
}
