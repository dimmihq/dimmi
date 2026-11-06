/*
|--------------------------------------------------------------------------
| Handoff Routing
|--------------------------------------------------------------------------
|
| A handed-off conversation goes where the workspace said: to its
| target team when nobody holds it yet, and up in priority when the
| workspace asked for that. A customer who may be in danger always gets
| a high priority, whatever the setting, and so does a security matter
| when no person is offered, so the team still sees it first.
|
*/

package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
routeHandoff assigns the target team and raises the priority as the
turn's settings ask. A target team that no longer exists is skipped.
*/
func (r *Runner) routeHandoff(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	e, s := st.env, st.settings
	w := r.agentWriter(tx, e)

	c := e.Conversation
	if s.HandoffTeamID != nil && c.AssigneeTeamID == nil && c.AssigneeMemberID == nil {
		assigned, _, err := w.Assign(ctx, c, conversations.Assignment{Team: s.HandoffTeamID})
		switch {
		case err == nil:
			c = assigned
		case !errors.Is(err, exceptions.ErrNotFound) && !errors.Is(err, database.ErrNotFound):
			return fmt.Errorf("assign handoff team: %w", err)
		}
	}

	high := enums.ConversationPriorityHigh
	if level, ok := high.Level(); ok && raisesPriority(st, d) && c.Priority < level {
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
raisesPriority reports whether a handoff makes the conversation high
priority: when the settings say so, always for a customer who may be
in danger, and for a security matter when no person is offered.
*/
func raisesPriority(st *run, d *decision) bool {
	switch {
	case st.settings.RaisePriority, d.handoffReason == "safety":
		return true
	case d.handoffReason == "security":
		return st.env.availability() == "unavailable"
	}
	return false
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
