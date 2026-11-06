/*
|--------------------------------------------------------------------------
| Handoff Routing
|--------------------------------------------------------------------------
|
| A handed-off conversation goes where the workspace said: to its
| target team when nobody holds it yet, and up in priority when the
| workspace asked for that. A customer who may be in danger always gets
| a high priority, whatever the setting.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
handoffSettings is the agent's stored handoff routing.
*/
type handoffSettings struct {
	TargetTeamID  *ids.UUID `json:"target_team_id"`
	RaisePriority bool      `json:"raise_priority"`
}

/*
routeHandoff assigns the target team and raises the priority as the
agent's settings ask. A target team that no longer exists is skipped.
*/
func (r *Runner) routeHandoff(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	e := st.env
	var h handoffSettings
	_ = json.Unmarshal(e.Agent.Handoff, &h)
	w := r.agentWriter(tx, e)

	c := e.Conversation
	if h.TargetTeamID != nil && c.AssigneeTeamID == nil && c.AssigneeMemberID == nil {
		assigned, _, err := w.Assign(ctx, c, conversations.Assignment{Team: h.TargetTeamID})
		switch {
		case err == nil:
			c = assigned
		case !errors.Is(err, exceptions.ErrNotFound) && !errors.Is(err, database.ErrNotFound):
			return fmt.Errorf("assign handoff team: %w", err)
		}
	}

	high := enums.ConversationPriorityHigh
	if level, ok := high.Level(); ok && (h.RaisePriority || d.handoffReason == "safety") && c.Priority < level {
		raised, _, err := w.SetAttributes(ctx, c, conversations.Attributes{Priority: &high})
		if err != nil {
			return fmt.Errorf("raise handoff priority: %w", err)
		}
		c = raised
	}
	e.Conversation = c
	return nil
}

/*
agentWriter writes to conversations as the agent.
*/
func (r *Runner) agentWriter(tx *database.Tx, e *env) *conversations.Writer {
	return &conversations.Writer{
		Q: models.New(tx), Tx: tx, Events: r.Events, Now: r.Clock.Now(),
		Actor: conversations.Actor{Type: string(enums.AuthorTypeAgent), ID: e.Agent.ID, Name: e.Agent.Name},
	}
}
