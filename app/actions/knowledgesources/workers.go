/*
|--------------------------------------------------------------------------
| Knowledge Source Workers
|--------------------------------------------------------------------------
|
| The queue's way into the engine: one worker per job, each a thin call
| to the step that does the work.
|
*/

package knowledgesources

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/jobs"
)

/*
DiscoverWorker reads listing pages.
*/
type DiscoverWorker struct {
	Engine *Engine
}

/*
Handle reads one page.
*/
func (w *DiscoverWorker) Handle(ctx context.Context, job *queue.Job[jobs.DiscoverKnowledgeSourceArgs]) error {
	return w.Engine.Discover(ctx, job)
}

/*
FetchWorker reads documents.
*/
type FetchWorker struct {
	Engine *Engine
}

/*
Handle reads one document.
*/
func (w *FetchWorker) Handle(ctx context.Context, job *queue.Job[jobs.FetchKnowledgeDocumentArgs]) error {
	return w.Engine.Fetch(ctx, job)
}

/*
FinishWorker closes runs.
*/
type FinishWorker struct {
	Engine *Engine
}

/*
Handle closes one run.
*/
func (w *FinishWorker) Handle(ctx context.Context, job *queue.Job[jobs.FinishKnowledgeSyncArgs]) error {
	return w.Engine.Finish(ctx, job)
}

/*
ImportWorker writes articles.
*/
type ImportWorker struct {
	Engine *Engine
}

/*
Handle writes one batch.
*/
func (w *ImportWorker) Handle(ctx context.Context, job *queue.Job[jobs.ImportKnowledgeSourceArgs]) error {
	return w.Engine.Import(ctx, job)
}

/*
ScheduleWorker starts scheduled runs.
*/
type ScheduleWorker struct {
	Engine *Engine
}

/*
Handle starts the runs whose time came.
*/
func (w *ScheduleWorker) Handle(ctx context.Context, job *queue.Job[jobs.ScheduleKnowledgeSyncsArgs]) error {
	return w.Engine.ScheduleDue(ctx, job)
}
