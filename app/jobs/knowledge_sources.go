/*
|--------------------------------------------------------------------------
| Knowledge Source Jobs
|--------------------------------------------------------------------------
|
| A sync run is a chain of small jobs, so a source of thousands of
| documents never holds one worker for long and a failure costs one
| document, not the run. Discovery reads one page of the source's
| listing at a time and queues the documents that changed; each fetch
| reads and indexes one document; the finish closes the run once both
| are done. A periodic job starts the runs whose schedule came, and an
| import turns a source's documents into help center articles.
|
*/

package jobs

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
DiscoverKnowledgeSourceArgs reads one page of a run's listing, from
Cursor, empty for the first.
*/
type DiscoverKnowledgeSourceArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RunID       ids.UUID `json:"run_id"`
	Cursor      string   `json:"cursor"`
}

/*
Kind names the job.
*/
func (DiscoverKnowledgeSourceArgs) Kind() string { return "discover_knowledge_source" }

/*
Options try a page a few times before the run fails.
*/
func (DiscoverKnowledgeSourceArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 4, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
FetchKnowledgeDocumentArgs reads and indexes one document of a run.
Version is the version the listing gave, empty when it gave none.
*/
type FetchKnowledgeDocumentArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RunID       ids.UUID `json:"run_id"`
	DocumentID  ids.UUID `json:"document_id"`
	Version     string   `json:"version"`
}

/*
Kind names the job.
*/
func (FetchKnowledgeDocumentArgs) Kind() string { return "fetch_knowledge_document" }

/*
Options try a document four times; after that it is marked failed and
the run goes on.
*/
func (FetchKnowledgeDocumentArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 4, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
FinishKnowledgeSyncArgs closes a run whose work is done.
*/
type FinishKnowledgeSyncArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	RunID       ids.UUID `json:"run_id"`
}

/*
Kind names the job.
*/
func (FinishKnowledgeSyncArgs) Kind() string { return "finish_knowledge_sync" }

/*
Options close a run once.
*/
func (FinishKnowledgeSyncArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 5, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
ImportKnowledgeSourceArgs turns a source's changed documents into help
center articles.
*/
type ImportKnowledgeSourceArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	SourceID    ids.UUID `json:"source_id"`
}

/*
Kind names the job.
*/
func (ImportKnowledgeSourceArgs) Kind() string { return "import_knowledge_source" }

/*
Options keep one import per source waiting at a time.
*/
func (ImportKnowledgeSourceArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
ScheduleKnowledgeSyncsArgs names the periodic job that starts the runs
whose time came; it takes no arguments.
*/
type ScheduleKnowledgeSyncsArgs struct{}

/*
Kind names the job.
*/
func (ScheduleKnowledgeSyncsArgs) Kind() string { return "schedule_knowledge_syncs" }

/*
Options run the scheduler once every few minutes at most.
*/
func (ScheduleKnowledgeSyncsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1, Unique: queue.UniqueOpts{ByPeriod: 4 * time.Minute}}
}
