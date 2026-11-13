/*
|--------------------------------------------------------------------------
| AI Tool Layer Command
|--------------------------------------------------------------------------
|
| dimmi ai:tool-layer turns a workspace's tool layer on or off, for its
| agent and for Operator. With the layer on, a model is offered a few
| tools and loads the rest on request; off, it is offered every tool on
| every call. New workspaces start with it on.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrLayerChoice refuses a call that does not say on or off.
*/
var ErrLayerChoice = errors.New("pass exactly one of --on or --off")

/*
AIToolLayerCommand turns a workspace's tool layer on or off.
*/
type AIToolLayerCommand struct {
	App *app.App
}

var _ foundation.Command = AIToolLayerCommand{}

/*
Name returns ai:tool-layer.
*/
func (AIToolLayerCommand) Name() string {
	return "ai:tool-layer"
}

/*
Description says what the command does.
*/
func (AIToolLayerCommand) Description() string {
	return "Turn the tool layer of a workspace's agent and Operator on or off"
}

/*
Flags declares the workspace and the choice.
*/
func (AIToolLayerCommand) Flags(flags *console.Flags) {
	flags.Argument("slug", "the workspace")
	flags.Bool("on", "offer a few tools and load the rest on request")
	flags.Bool("off", "offer every tool on every call")
}

/*
Handle saves the choice for the agent and Operator.
*/
func (c AIToolLayerCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	on, off := in.Bool("on"), in.Bool("off")
	if on == off {
		return ErrLayerChoice
	}
	ws, err := models.New(c.App.DB).FindWorkspaceBySlug(ctx, in.Argument("slug"))
	if err != nil {
		return fmt.Errorf("find workspace: %w", err)
	}
	now := c.App.Clock.Now()
	err = c.App.DB.Transaction(tenancy.With(ctx, ws.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		agent, err := q.FindAgent(ctx, ws.ID)
		if err != nil {
			return fmt.Errorf("find agent: %w", err)
		}
		if err := q.SetAgentToolLayer(ctx, models.SetAgentToolLayerParams{
			WorkspaceID: ws.ID, ID: agent.ID, ToolLayer: on, Now: now,
		}); err != nil {
			return err
		}
		return q.SetOperatorToolLayer(ctx, models.SetOperatorToolLayerParams{
			WorkspaceID: ws.ID, ToolLayer: on, Now: now,
		})
	})
	if err != nil {
		return err
	}
	state := "off"
	if on {
		state = "on"
	}
	out.Info("The tool layer of %s is %s", ws.Slug, state)
	return nil
}
