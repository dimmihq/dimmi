/*
|--------------------------------------------------------------------------
| Knowledge Recut
|--------------------------------------------------------------------------
|
| Published articles are cut into passages once, when they are
| published. When a release changes how articles are cut, this command
| cuts every workspace's live articles again, so the whole help center
| is searched the new way and not only what is published afterwards.
| The queue worker then embeds the new passages with each workspace's
| own embedding model, which is paid for like any other embedding.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
KnowledgeRecutCommand cuts every workspace's live articles again.
*/
type KnowledgeRecutCommand struct {
	App *app.App
}

/*
Name returns knowledge:recut.
*/
func (c KnowledgeRecutCommand) Name() string {
	return "knowledge:recut"
}

/*
Description says what the command does and that embedding follows.
*/
func (c KnowledgeRecutCommand) Description() string {
	return "Cut every workspace's published articles into search passages again, after a release changed " +
		"how they are cut. The queue worker then embeds the new passages, which calls the embeddings API"
}

/*
Flags asks for the confirmation.
*/
func (c KnowledgeRecutCommand) Flags(flags *console.Flags) {
	flags.Bool("confirm", "cut the articles again and queue their embeddings")
}

/*
Handle cuts each workspace's articles again, one workspace at a time.
*/
func (c KnowledgeRecutCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	if c.App.Vectors == nil {
		return errors.New("the knowledge index is not set up on this install")
	}
	workspaces, err := models.New(c.App.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}
	if !in.Bool("confirm") {
		out.Warn("Nothing was done. Run again with --confirm to cut the articles of %d workspaces again.",
			len(workspaces))
		return nil
	}
	for _, w := range workspaces {
		n, err := c.App.Vectors.Recut(ctx, w.ID)
		if err != nil {
			return fmt.Errorf("recut %s: %w", w.Slug, err)
		}
		out.Line("%s: %d passages.", w.Slug, n)
	}
	out.Info("Done. The queue worker embeds the new passages.")
	return nil
}
