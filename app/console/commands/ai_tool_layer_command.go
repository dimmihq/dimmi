/*
|--------------------------------------------------------------------------
| AI Tool Layer Command
|--------------------------------------------------------------------------
|
| dimmi ai:tool-layer turns a workspace's token savers on or off, for
| its agent and for Operator: the tool layer, which offers a model a few
| tools and loads the rest on request; the agent's search before its
| first model call; and compact tool results. --features picks which.
| New workspaces start with all of them on.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrLayerChoice refuses a call that does not say on or off.
*/
var ErrLayerChoice = errors.New("pass exactly one of --on or --off, and known --features")

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
	return "Turn the token savers of a workspace's agent and Operator on or off"
}

/*
Flags declares the workspace and the choice.
*/
func (AIToolLayerCommand) Flags(flags *console.Flags) {
	flags.Argument("slug", "the workspace")
	flags.Bool("on", "offer a few tools and load the rest on request")
	flags.Bool("off", "offer every tool on every call")
	flags.String("features", "layer,prefetch,compact", "which savers to switch: layer, prefetch, compact, carry")
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
	features := strings.Split(in.String("features"), ",")
	err = c.App.DB.Transaction(tenancy.With(ctx, ws.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		return c.save(ctx, models.New(tx), saverChoice{ws: ws.ID, features: features, on: on})
	})
	if err != nil {
		return err
	}
	state := "off"
	if on {
		state = "on"
	}
	out.Info("%s of %s: %s", strings.Join(features, ", "), ws.Slug, state)
	return nil
}

/*
saverChoice is the workspace, the savers to switch and whether on.
*/
type saverChoice struct {
	ws       ids.UUID
	features []string
	on       bool
}

/*
save switches the chosen savers for the agent and Operator.
*/
func (c AIToolLayerCommand) save(ctx context.Context, q *models.Queries, choice saverChoice) error {
	ws, features, on := choice.ws, choice.features, choice.on
	now := c.App.Clock.Now()
	agent, err := q.FindAgent(ctx, ws)
	if err != nil {
		return fmt.Errorf("find agent: %w", err)
	}
	prefetch, compact, carry := agent.PrefetchSearch, agent.CompactResults, agent.CarryPassages
	for _, f := range features {
		switch strings.TrimSpace(f) {
		case "layer":
			if err := q.SetAgentToolLayer(ctx, models.SetAgentToolLayerParams{
				WorkspaceID: ws, ID: agent.ID, ToolLayer: on, Now: now,
			}); err != nil {
				return err
			}
			if err := q.SetOperatorToolLayer(ctx, models.SetOperatorToolLayerParams{
				WorkspaceID: ws, ToolLayer: on, Now: now,
			}); err != nil {
				return err
			}
		case "prefetch":
			prefetch = on
		case "carry":
			carry = on
		case "compact":
			compact = on
			if err := q.SetOperatorCompactResults(ctx, models.SetOperatorCompactResultsParams{
				WorkspaceID: ws, CompactResults: on, Now: now,
			}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: unknown feature %q", ErrLayerChoice, f)
		}
	}
	return q.SetAgentTokenSavers(ctx, models.SetAgentTokenSaversParams{
		WorkspaceID: ws, ID: agent.ID, PrefetchSearch: prefetch, CompactResults: compact, CarryPassages: carry, Now: now,
	})
}
