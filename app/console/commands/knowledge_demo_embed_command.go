//go:build dev

/*
|--------------------------------------------------------------------------
| Knowledge Demo Embed
|--------------------------------------------------------------------------
|
| workspace:demo leaves the Stowline help center on keyword search, so
| seeding never calls a paid embeddings API. This is the separate step
| that builds its vectors: it binds the install's default embedding
| model, which makes one tiny check call, and queues the build, which
| the queue worker runs batch by batch. It prints the size of the job
| and its cost from the price table first, and does nothing without
| --confirm.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/demo/stowline"
	"github.com/dimmihq/dimmi/app/demo/wixqa"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
KnowledgeDemoEmbedCommand builds the Stowline help center's vectors.
*/
type KnowledgeDemoEmbedCommand struct {
	App *app.App
}

/*
Name returns knowledge:demo-embed.
*/
func (c KnowledgeDemoEmbedCommand) Name() string {
	return "knowledge:demo-embed"
}

/*
Description says what the command does and that it costs money.
*/
func (c KnowledgeDemoEmbedCommand) Description() string {
	return "Build vector search for the Stowline demo help center with the install's embedding model. " +
		"Calls a paid embeddings API through the queue worker; prints the size and cost and needs --confirm"
}

/*
Flags asks for the confirmation.
*/
func (c KnowledgeDemoEmbedCommand) Flags(flags *console.Flags) {
	flags.Bool("confirm", "bind the model and queue the paid build")
	flags.String("workspace", "stowline", "which demo workspace: stowline or wixqa")
}

/*
Handle reports the job's size and, when confirmed, starts it.
*/
func (c KnowledgeDemoEmbedCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	find := stowline.Find
	if in.String("workspace") == wixqa.Seed {
		find = wixqa.Find
	}
	w, found, err := find(ctx, c.App)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("no such demo workspace; run workspace:demo or workspace:wixqa first")
	}
	ctx = tenancy.With(ctx, w.ID.String())

	var chunks, chars int64
	if err := c.App.DB.QueryRow(ctx, `SELECT count(*), coalesce(sum(length(title) + length(body)), 0)
		FROM knowledge_chunks WHERE workspace_id = $1`, w.ID).Scan(&chunks, &chars); err != nil {
		return fmt.Errorf("measure chunks: %w", err)
	}
	tokens := chars / 4
	out.Line("%d chunks, about %d tokens to embed.", chunks, tokens)
	if c.App.Embeddings == nil || !c.App.Embeddings.Builtin() {
		return errors.New("this install has no built-in embedding model; pick one under Settings, AI, Knowledge")
	}
	model := c.App.Embeddings.BuiltinModelKey()
	c.price(out, tokens)
	if !in.Bool("confirm") {
		out.Warn("Nothing was done. Run again with --confirm to bind %s and queue the build.", model)
		return nil
	}

	owner, err := demoOwner(ctx, c.App)
	if in.String("workspace") == wixqa.Seed {
		owner, _, _, err = wixqaPlace(ctx, c.App)
	}
	if err != nil {
		return err
	}
	if _, err := c.App.Actions.KnowledgeSettings.Update(ctx, owner, knowledge.SettingsInput{}); err != nil {
		return fmt.Errorf("bind embedding model: %w", err)
	}
	out.Info("Bound %s and queued the build; the queue worker embeds the chunks and switches search to hybrid.", model)
	return nil
}

/*
price prints what embedding the tokens costs on the built-in model,
from the price table.
*/
func (c KnowledgeDemoEmbedCommand) price(out *console.Output, tokens int64) {
	if c.App.AI == nil || c.App.AI.Prices == nil {
		return
	}
	for _, p := range c.App.AI.Prices.Models {
		if p.Provider == string(enums.AIProviderDimmi) && slices.Contains(p.Roles, "embed") {
			out.Line("Price table: %s at $%s per million tokens, so about $%.4f.", p.Model, p.InputPerMtok,
				float64(tokens)/1e6*atof(p.InputPerMtok))
		}
	}
}

func atof(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%g", &f)
	return f
}
