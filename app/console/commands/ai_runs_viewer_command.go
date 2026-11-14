//go:build dev

/*
|--------------------------------------------------------------------------
| Benchmark Run Viewer
|--------------------------------------------------------------------------
|
| dimmi ai:runs-viewer writes plain pages for every benchmark run under
| tmp/: an index, a page per run and a page per conversation with its
| transcript and every model call. ai:simulate and ai:quality rebuild
| them after each run. Development builds only; it never runs in
| production.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/ai/costsim"
)

/*
The viewer's folders: where reports are read from, and where the pages go.
*/
const (
	viewerSource = "tmp"
	viewerSite   = "tmp/benchmarks/site"
)

/*
AIRunsViewerCommand writes the run viewer.
*/
type AIRunsViewerCommand struct {
	App *app.App
}

/*
Name returns ai:runs-viewer.
*/
func (AIRunsViewerCommand) Name() string { return "ai:runs-viewer" }

/*
Description says what the command does.
*/
func (AIRunsViewerCommand) Description() string {
	return "Write browsable pages for every benchmark run under tmp/benchmarks/site"
}

/*
Flags declares nothing.
*/
func (AIRunsViewerCommand) Flags(*console.Flags) {}

/*
Handle writes the pages.
*/
func (c AIRunsViewerCommand) Handle(ctx context.Context, _ *foundation.Application, _ *console.Input,
	out *console.Output,
) error {
	n, err := buildViewer(ctx, c.App)
	if err != nil {
		return err
	}
	out.Info("Wrote %d runs. Open %s/index.html", n, viewerSite)
	return nil
}

/*
buildViewer writes the pages for every run under tmp/.
*/
func buildViewer(ctx context.Context, a *app.App) (int, error) {
	if a.Application.Config.App.Env == foundation.EnvProduction {
		return 0, errors.New("the run viewer never runs in production")
	}
	from, err := a.Storage.Build("viewer-source", config.Section{"driver": "local", "root": viewerSource})
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", viewerSource, err)
	}
	to, err := a.Storage.Build("viewer-site", config.Section{"driver": "local", "root": viewerSite})
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", viewerSite, err)
	}
	return (&costsim.Viewer{App: a, From: from, To: to}).Build(ctx)
}

/*
refreshViewer rebuilds the pages after a run; a failure only warns, as
the run's report is already written.
*/
func refreshViewer(ctx context.Context, a *app.App, out *console.Output) {
	if _, err := buildViewer(ctx, a); err != nil {
		out.Warn("Could not rebuild the run viewer: %v", err)
		return
	}
	out.Line("Viewer: %s/index.html", viewerSite)
}
