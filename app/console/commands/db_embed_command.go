//go:build dev

/*
|--------------------------------------------------------------------------
| Database Seed Embeddings
|--------------------------------------------------------------------------
|
| db:seed leaves the Stowline help center on keyword search by default so
| normal seeding never calls a paid embeddings API. Passing --embed to
| db:seed, or running db:embed afterward, binds the install's built-in
| embedding model and builds vectors for the seeded help center. It checks
| that built-in embedding credentials (Voyage AI) are configured first and
| fails immediately if they are missing.
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
	"github.com/dimmihq/dimmi/app/enums"
)

/*
DBEmbedCommand builds vector search for the seeded Stowline help center.
*/
type DBEmbedCommand struct {
	App *app.App
}

/*
Name returns db:embed.
*/
func (c DBEmbedCommand) Name() string {
	return "db:embed"
}

/*
Description says what the command does.
*/
func (c DBEmbedCommand) Description() string {
	return "Build vector search for the seeded Stowline help center using the configured built-in embedding model"
}

/*
Flags configures inline vs queued execution.
*/
func (c DBEmbedCommand) Flags(flags *console.Flags) {
	flags.Bool("now", "embed in this process instead of the queue, so no other queued job runs")
}

/*
embedOptions configures embedStowline output and inline vs queued execution.
*/
type embedOptions struct {
	Out *console.Output
	Now bool
}

/*
Handle verifies embedding credentials and builds vectors for Stowline.
*/
func (c DBEmbedCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	return embedStowline(ctx, c.App, embedOptions{Out: out, Now: in.Bool("now")})
}

/*
requireEmbeddingCredentials checks that the install has built-in embedding
credentials configured before starting an embedding run.
*/
func requireEmbeddingCredentials(a *app.App) error {
	if a.Embeddings == nil || !a.Embeddings.Builtin() {
		return errors.New("built-in embeddings are not configured; set ai.managed.embeddings.api_key " +
			"(DIMMI_AI__MANAGED__EMBEDDINGS__API_KEY) and try again")
	}
	return nil
}

/*
embedStowline binds the built-in embedding model on the seeded Stowline
workspace and queues or runs the vector build immediately.
*/
func embedStowline(ctx context.Context, a *app.App, opts embedOptions) error {
	if err := requireEmbeddingCredentials(a); err != nil {
		return err
	}
	w, found, err := stowline.Find(ctx, a)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("no seeded Stowline workspace; run db:seed first")
	}
	ctx = tenancy.With(ctx, w.ID.String())

	var chunks, chars int64
	if err := a.DB.QueryRow(ctx, `SELECT count(*), coalesce(sum(length(title) + length(body)), 0)
		FROM knowledge_chunks WHERE workspace_id = $1`, w.ID).Scan(&chunks, &chars); err != nil {
		return fmt.Errorf("measure chunks: %w", err)
	}
	tokens := chars / 4
	opts.Out.Line("%d chunks, about %d tokens to embed.", chunks, tokens)

	model := a.Embeddings.BuiltinModelKey()
	printEmbedPrice(a, opts.Out, tokens)

	owner, err := demoOwner(ctx, a)
	if err != nil {
		return err
	}
	if _, err := a.Actions.KnowledgeSettings.Update(ctx, owner, knowledge.SettingsInput{}); err != nil {
		return fmt.Errorf("bind embedding model: %w", err)
	}
	if !opts.Now {
		opts.Out.Info(
			"Bound %s and queued the build; the queue worker embeds the chunks and switches search to hybrid.", model,
		)
		return nil
	}
	if err := a.Vectors.BuildNow(ctx, w.ID, model); err != nil {
		return fmt.Errorf("build embeddings: %w", err)
	}
	opts.Out.Info("Bound %s and embedded the chunks here; search is hybrid once the build is complete.", model)
	return nil
}

/*
printEmbedPrice prints what embedding the tokens costs on the built-in model,
from the price table.
*/
func printEmbedPrice(a *app.App, out *console.Output, tokens int64) {
	if a.AI == nil || a.AI.Prices == nil {
		return
	}
	for _, p := range a.AI.Prices.Models {
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
