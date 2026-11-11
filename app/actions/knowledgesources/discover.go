/*
|--------------------------------------------------------------------------
| Discovery
|--------------------------------------------------------------------------
|
| Discovery reads the source's listing one page per job. Each document
| found is recorded as seen by the run; it is fetched again only when
| its version moved, it never read, or it failed before. Documents the
| member's patterns leave out are set aside as such, and documents a
| member excluded by hand stay excluded. A source never holds more than
| its limit. When the listing ends and nothing waits to be fetched, the
| run is closed.
|
*/

package knowledgesources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
runState is what a run's jobs work with: the run, its source, the
source's driver and settings, and the most documents it may hold. Once
the run ended it is not live, so leftover jobs do nothing.
*/
type runState struct {
	run    models.KnowledgeSyncRun
	source models.KnowledgeSource
	driver sources.Driver
	cfg    sources.Config
	limit  int
	live   bool
}

/*
load reads a run's state.
*/
func (e *Engine) load(ctx context.Context, ws, runID ids.UUID) (runState, error) {
	var st runState

	err := e.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		run, err := findRun(ctx, q, ws, runID)
		if errors.Is(err, ErrRunNotFound) {
			return nil
		}

		if err != nil {
			return err
		}

		st.run = run
		if run.Status != "running" {
			return nil
		}

		if st.source, err = findSource(ctx, q, ws, run.SourceID); err != nil {
			return err
		}

		st.live = true

		return nil
	})
	if err != nil || !st.live {
		return st, err
	}

	if st.driver, err = e.driverOf(st.source.Type); err != nil {
		return st, err
	}

	if st.cfg, err = e.settingsOf(ctx, st.source); err != nil {
		return st, err
	}

	st.limit = maxDocuments
	if capped, ok := st.driver.(sources.Capped); ok {
		st.limit = min(st.limit, capped.MaxDocuments(st.cfg))
	}

	return st, nil
}

/*
Discover reads one page of a run's listing.
*/
func (e *Engine) Discover(ctx context.Context, job *queue.Job[jobs.DiscoverKnowledgeSourceArgs]) error {
	a := job.Args

	st, err := e.load(ctx, a.WorkspaceID, a.RunID)
	if err != nil || !st.live {
		return err
	}

	page, err := st.driver.List(ctx, st.cfg, a.Cursor)
	if err != nil {
		return e.listFailed(ctx, job, st, err)
	}

	return e.inWorkspace(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		if err := lockRun(ctx, q, st.run); err != nil {
			return err
		}

		return e.recordPage(ctx, tx, st, page)
	})
}

/*
listFailed waits out a busy source, retries a passing failure, and
ends the run once the listing cannot be read.
*/
func (e *Engine) listFailed(ctx context.Context, job *queue.Job[jobs.DiscoverKnowledgeSourceArgs], st runState,
	err error,
) error {
	if limited, ok := errors.AsType[*sources.RateLimitError](err); ok {
		return queue.Postpone(limited.RetryAfter)
	}

	if wait, ok := errors.AsType[*sources.WaitError](err); ok {
		return queue.Postpone(wait.After)
	}

	code := errorCode(err)
	if code == "failed" && job.Attempt < job.MaxAttempts && ctx.Err() == nil {
		return err
	}

	ws := job.Args.WorkspaceID

	return e.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, _ *models.Queries) error {
		_, endErr := e.end(ctx, tx, st.run, outcome{status: "failed", code: code, message: message(err)})

		return endErr
	})
}

/*
recordPage records one listing page, queues what must be fetched and
moves discovery on.
*/
func (e *Engine) recordPage(ctx context.Context, tx *database.Tx, st runState, page sources.Page) error {
	q := models.New(tx)
	if err := e.recordCrawl(ctx, q, st, page.Usage); err != nil {
		return err
	}
	room := st.limit - int(stat(st.run, "discovered"))
	deltas := map[string]int64{}

	var pending int32

	for _, ref := range page.Refs {
		if room <= 0 {
			break
		}

		queued, kind, err := e.see(ctx, tx, st, ref)
		if err != nil {
			return err
		}

		if kind != "" {
			room--
			deltas["discovered"]++
			deltas[kind]++
		}

		if queued {
			pending++
		}
	}

	run, err := e.addWork(ctx, q, st.run, work{pending: pending, deltas: deltas})
	if err != nil {
		return err
	}

	if page.Next != "" && room > 0 {
		next := jobs.DiscoverKnowledgeSourceArgs{WorkspaceID: run.WorkspaceID, RunID: run.ID, Cursor: page.Next}

		return wrap("queue next page", e.Queue.PushTx(ctx, tx, next))
	}

	if run, err = q.MarkKnowledgeSyncDiscovered(ctx, models.MarkKnowledgeSyncDiscoveredParams{
		Now: e.Clock.Now(), WorkspaceID: run.WorkspaceID, ID: run.ID,
	}); err != nil {
		return fmt.Errorf("mark discovered: %w", err)
	}

	return e.finishWhenDone(ctx, tx, run)
}

/*
see records one listed document as seen by the run and queues its
fetch when needed. It returns whether it queued a fetch and how the
document counts, empty when the run had already seen it.
*/
func (e *Engine) see(ctx context.Context, tx *database.Tx, st runState, ref sources.Ref) (bool, string, error) {
	q, now := models.New(tx), e.Clock.Now()

	doc, err := q.SeeKnowledgeDocument(ctx, models.SeeKnowledgeDocumentParams{
		ID: ids.NewV7At(now), WorkspaceID: st.run.WorkspaceID, SourceID: st.source.ID, ExternalID: ref.ExternalID,
		Url: nonEmpty(ref.URL), Path: ref.Path, Title: ref.Title, RunID: &st.run.ID, Now: now,
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, "", nil
	}

	if err != nil {
		return false, "", fmt.Errorf("record document: %w", err)
	}

	switch {
	case doc.Status == "excluded" && doc.ExcludedBy != nil && *doc.ExcludedBy == "member":
		return false, "excluded", nil
	case !sources.Allowed(st.source.Include, st.source.Exclude, ref.Path):
		return false, "excluded", wrap("exclude document", q.ExcludeKnowledgeDocument(ctx,
			models.ExcludeKnowledgeDocumentParams{
				ExcludedBy: new("pattern"), Now: now, WorkspaceID: doc.WorkspaceID,
				ID: doc.ID,
			}))
	case doc.Status == "active" && ref.Version != "" && doc.Version == ref.Version:
		return false, "unchanged", nil
	case ref.Document != nil:
		kind, err := e.inline(ctx, tx, st, listed{doc: doc, ref: ref})

		return false, kind, err
	}

	args := jobs.FetchKnowledgeDocumentArgs{
		WorkspaceID: doc.WorkspaceID, RunID: st.run.ID, DocumentID: doc.ID, Version: ref.Version,
	}

	return true, "queued", wrap("queue fetch", e.Queue.PushTx(ctx, tx, args))
}

/*
listed is a stored document and the reference a listing gave for it.
*/
type listed struct {
	doc models.KnowledgeDocument
	ref sources.Ref
}

/*
inline stores a document the listing already read, in the listing's
transaction, so a crawl service's pages need no second fetch. It
returns how the document counts.
*/
func (e *Engine) inline(ctx context.Context, tx *database.Tx, st runState, l listed) (string, error) {
	doc, ref := l.doc, l.ref

	got := *ref.Document
	if got.Draft && st.source.Mode == string(sources.ModeSync) {
		return "excluded", wrap("exclude draft", e.setAside(ctx, tx, doc, "draft"))
	}

	w, reason := prepare(doc, ref.Version, got)
	if reason != "" {
		return "excluded", wrap("exclude document", e.setAside(ctx, tx, doc, reason))
	}

	if w.hash == doc.ContentHash && doc.Status == "active" {
		return "unchanged", wrap("touch document", models.New(tx).TouchKnowledgeDocument(ctx,
			models.TouchKnowledgeDocumentParams{
				Version: w.version, Now: new(e.Clock.Now()), WorkspaceID: doc.WorkspaceID, ID: doc.ID,
			}))
	}

	kind := "updated"
	if doc.ContentHash == "" {
		kind = "created"
	}

	return kind, e.write(ctx, tx, st, w)
}

/*
setAside excludes a document for the reason and drops its chunks.
*/
func (e *Engine) setAside(ctx context.Context, tx *database.Tx, doc models.KnowledgeDocument, reason string) error {
	q := models.New(tx)
	if err := q.ExcludeKnowledgeDocument(ctx, models.ExcludeKnowledgeDocumentParams{
		ExcludedBy: &reason, Now: e.Clock.Now(), WorkspaceID: doc.WorkspaceID, ID: doc.ID,
	}); err != nil {
		return err
	}

	return q.DeleteKnowledgeDocumentChunks(ctx, models.DeleteKnowledgeDocumentChunksParams{
		WorkspaceID: doc.WorkspaceID, DocumentID: doc.ID,
	})
}

/*
work is a change to a run: documents queued or settled, and counters
moved.
*/
type work struct {
	pending int32
	deltas  map[string]int64
}

/*
addWork moves the run's pending count and counters.
*/
func (e *Engine) addWork(ctx context.Context, q *models.Queries, run models.KnowledgeSyncRun, w work,
) (models.KnowledgeSyncRun, error) {
	raw, err := json.Marshal(w.deltas)
	if err != nil {
		return run, err
	}

	out, err := q.AddKnowledgeSyncWork(ctx, models.AddKnowledgeSyncWorkParams{
		Pending: w.pending, Deltas: raw, Now: e.Clock.Now(), WorkspaceID: run.WorkspaceID, ID: run.ID,
	})

	return out, wrap("count sync work", err)
}

/*
finishWhenDone queues the run's close once discovery ended and nothing
waits to be fetched.
*/
func (e *Engine) finishWhenDone(ctx context.Context, tx *database.Tx, run models.KnowledgeSyncRun) error {
	if !run.Discovered || run.Pending > 0 {
		return nil
	}

	args := jobs.FinishKnowledgeSyncArgs{WorkspaceID: run.WorkspaceID, RunID: run.ID}

	return wrap("queue sync finish", e.Queue.PushTx(ctx, tx, args))
}

/*
stat reads one of the run's counters.
*/
func stat(run models.KnowledgeSyncRun, key string) int64 {
	var stats map[string]int64
	if err := json.Unmarshal(run.Stats, &stats); err != nil {
		return 0
	}

	return stats[key]
}

/*
nonEmpty is the string, or nil when it is empty.
*/
func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
