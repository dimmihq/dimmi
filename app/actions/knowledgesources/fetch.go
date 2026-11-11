/*
|--------------------------------------------------------------------------
| Fetching Documents
|--------------------------------------------------------------------------
|
| Each document is read in its own job. A busy source is waited out
| without using up an attempt; a passing failure is retried; a document
| still failing after its last attempt is marked failed and the run goes
| on. A document whose text hashes the same as before is only touched,
| so nothing is chunked or embedded again. A synced document is chunked
| into the index in the same transaction that stores it; an imported
| one waits for the import to become an article. Links a crawl hands
| back become new documents of the same run, within the source's limit.
|
*/

package knowledgesources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
fetched is the result of one fetch: the counter it moves and the links
it found.
*/
type fetched struct {
	kind  string
	links []string
}

/*
Fetch reads and indexes one document of a run.
*/
func (e *Engine) Fetch(ctx context.Context, job *queue.Job[jobs.FetchKnowledgeDocumentArgs]) error {
	a := job.Args

	st, err := e.load(ctx, a.WorkspaceID, a.RunID)
	if err != nil || !st.live {
		return err
	}

	var doc models.KnowledgeDocument

	err = e.inWorkspace(ctx, a.WorkspaceID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		doc, err = q.FindKnowledgeDocument(ctx, models.FindKnowledgeDocumentParams{
			WorkspaceID: a.WorkspaceID, ID: a.DocumentID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return e.settle(ctx, st, fetched{kind: "removed"})
	}

	if err != nil {
		return fmt.Errorf("find document: %w", err)
	}

	ref := sources.Ref{
		ExternalID: doc.ExternalID, URL: deref(doc.Url), Path: doc.Path, Title: doc.Title, Version: a.Version,
	}

	got, err := st.driver.Fetch(ctx, st.cfg, ref)
	if err != nil {
		return e.fetchFailed(ctx, job, failure{st: st, doc: doc, err: err, links: got.Links})
	}

	return e.store(ctx, st, doc, read{version: a.Version, doc: got})
}

/*
failure is a fetch that failed: the run, the document, the error and
the links the driver still handed back.
*/
type failure struct {
	st    runState
	doc   models.KnowledgeDocument
	err   error
	links []string
}

/*
fetchFailed decides what a failed fetch means for the document.
*/
func (e *Engine) fetchFailed(ctx context.Context, job *queue.Job[jobs.FetchKnowledgeDocumentArgs], f failure) error {
	st, doc, err := f.st, f.doc, f.err

	if limited, ok := errors.AsType[*sources.RateLimitError](err); ok {
		return queue.Postpone(limited.RetryAfter)
	}

	if excluded, ok := errors.AsType[*sources.ExcludedError](err); ok {
		return e.putAside(ctx, st, doc, aside{reason: excluded.Reason, links: f.links})
	}

	switch {
	case errors.Is(err, sources.ErrNotFound):
		return e.markGone(ctx, st, doc)
	case errorCode(err) == "failed" && job.Attempt < job.MaxAttempts && ctx.Err() == nil:
		return err
	}

	return e.inWorkspace(ctx, doc.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		if err := lockRun(ctx, q, st.run); err != nil {
			return err
		}

		if err := q.FailKnowledgeDocument(ctx, models.FailKnowledgeDocumentParams{
			ErrorCode: new(errorCode(err)), Error: new(message(err)), Now: e.Clock.Now(),
			WorkspaceID: doc.WorkspaceID, ID: doc.ID,
		}); err != nil {
			return fmt.Errorf("fail document: %w", err)
		}

		return e.settleTx(ctx, tx, st, fetched{kind: "failed"})
	})
}

/*
aside is why a document is set aside, and the links it still handed
back.
*/
type aside struct {
	reason string
	links  []string
}

/*
putAside sets a document aside with the reason the driver gave and
follows the links it still handed back.
*/
func (e *Engine) putAside(ctx context.Context, st runState, doc models.KnowledgeDocument, a aside) error {
	reason, links := a.reason, a.links

	return e.inWorkspace(ctx, doc.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		if err := lockRun(ctx, q, st.run); err != nil {
			return err
		}

		if err := q.ExcludeKnowledgeDocument(ctx, models.ExcludeKnowledgeDocumentParams{
			ExcludedBy: &reason, Now: e.Clock.Now(), WorkspaceID: doc.WorkspaceID, ID: doc.ID,
		}); err != nil {
			return fmt.Errorf("exclude document: %w", err)
		}

		if err := q.DeleteKnowledgeDocumentChunks(ctx, models.DeleteKnowledgeDocumentChunksParams{
			WorkspaceID: doc.WorkspaceID, DocumentID: doc.ID,
		}); err != nil {
			return fmt.Errorf("drop excluded chunks: %w", err)
		}

		kind := "excluded"
		if reason == "js_rendered" {
			kind = "skipped_js"
		}

		return e.settleTx(ctx, tx, st, fetched{kind: kind, links: links})
	})
}

/*
markGone removes a document its source no longer has.
*/
func (e *Engine) markGone(ctx context.Context, st runState, doc models.KnowledgeDocument) error {
	return e.inWorkspace(ctx, doc.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		if err := lockRun(ctx, q, st.run); err != nil {
			return err
		}

		if err := q.RemoveKnowledgeDocument(ctx, models.RemoveKnowledgeDocumentParams{
			Now: e.Clock.Now(), WorkspaceID: doc.WorkspaceID, ID: doc.ID,
		}); err != nil {
			return fmt.Errorf("remove document: %w", err)
		}

		if err := q.DeleteKnowledgeDocumentChunks(ctx, models.DeleteKnowledgeDocumentChunksParams{
			WorkspaceID: doc.WorkspaceID, DocumentID: doc.ID,
		}); err != nil {
			return fmt.Errorf("drop removed chunks: %w", err)
		}

		return e.settleTx(ctx, tx, st, fetched{kind: "removed"})
	})
}

/*
read is what a fetch read and the version the listing gave.
*/
type read struct {
	version string
	doc     sources.Document
}

/*
store keeps what a fetch read, indexing it when its text changed.
*/
func (e *Engine) store(ctx context.Context, st runState, doc models.KnowledgeDocument, r read) error {
	version, got := r.version, r.doc

	if got.Draft && st.source.Mode == string(sources.ModeSync) {
		return e.putAside(ctx, st, doc, aside{reason: "draft", links: got.Links})
	}

	w, reason := prepare(doc, version, got)
	if reason != "" {
		return e.putAside(ctx, st, doc, aside{reason: reason, links: got.Links})
	}

	return e.inWorkspace(ctx, doc.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		if err := lockRun(ctx, q, st.run); err != nil {
			return err
		}

		if w.hash == doc.ContentHash && doc.Status == "active" {
			if err := q.TouchKnowledgeDocument(ctx, models.TouchKnowledgeDocumentParams{
				Version: w.version, Now: new(e.Clock.Now()), WorkspaceID: doc.WorkspaceID, ID: doc.ID,
			}); err != nil {
				return fmt.Errorf("touch document: %w", err)
			}

			return e.settleTx(ctx, tx, st, fetched{kind: "unchanged", links: got.Links})
		}

		kind := "updated"
		if doc.ContentHash == "" {
			kind = "created"
		}

		if err := e.write(ctx, tx, st, w); err != nil {
			return err
		}

		return e.settleTx(ctx, tx, st, fetched{kind: kind, links: got.Links})
	})
}

/*
prepare readies a fetched document to store, or names why it is set
aside: a draft of a synced source, or no text at all.
*/
func prepare(doc models.KnowledgeDocument, version string, got sources.Document) (written, string) {
	body := truncateUTF8(got.Markdown, maxMarkdown)
	if strings.TrimSpace(body) == "" {
		return written{}, "empty"
	}

	title := strings.TrimSpace(got.Title)
	if title == "" {
		title = cmpString(doc.Title, sources.Humanize(doc.Path))
	}

	return written{
		stored: doc, doc: got, title: title, body: body, hash: hashing.SHA256Hex([]byte(title + "\n" + body)),
		version: cmpString(got.Version, version),
	}, ""
}

/*
written is a fetched document ready to store over the stored one.
*/
type written struct {
	stored  models.KnowledgeDocument
	doc     sources.Document
	title   string
	body    string
	hash    string
	version string
}

/*
write stores the document's new text and, for a synced source, its
chunks.
*/
func (e *Engine) write(ctx context.Context, tx *database.Tx, st runState, w written) error {
	doc := w.stored

	collection, _ := json.Marshal(nonNil(w.doc.Collections))
	assets, _ := json.Marshal(nonNil(w.doc.Assets))
	visibility := visibilityOf(w.doc.Visibility)

	stored, err := models.New(tx).StoreKnowledgeDocument(ctx, models.StoreKnowledgeDocumentParams{
		Title: w.title, Url: nonEmpty(cmpString(w.doc.URL, deref(doc.Url))), Path: cmpString(w.doc.Path, doc.Path),
		Locale: w.doc.Locale, TranslationOf: nonEmpty(w.doc.TranslationOf), Collection: collection,
		Version: w.version, ContentMarkdown: w.body, ContentHash: w.hash, Visibility: string(visibility),
		Draft:  w.doc.Draft,
		Assets: assets, Now: new(e.Clock.Now()), WorkspaceID: doc.WorkspaceID, ID: doc.ID,
	})
	if err != nil {
		return fmt.Errorf("store document: %w", err)
	}

	if st.source.Mode != string(sources.ModeSync) {
		return nil
	}

	_, err = e.Vectors.IndexDocument(ctx, tx, knowledge.SourceDocument{
		ID: stored.ID, WorkspaceID: stored.WorkspaceID, SourceID: stored.SourceID, Title: stored.Title,
		Markdown: stored.ContentMarkdown, ContentHash: stored.ContentHash, Visibility: visibility,
	})

	return err
}

/*
settle records one finished fetch on its run.
*/
func (e *Engine) settle(ctx context.Context, st runState, f fetched) error {
	return e.inWorkspace(ctx, st.run.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		if err := lockRun(ctx, q, st.run); err != nil {
			return err
		}

		return e.settleTx(ctx, tx, st, f)
	})
}

/*
settleTx counts one finished fetch, queues the new links it found and
closes the run when it was the last.
*/
func (e *Engine) settleTx(ctx context.Context, tx *database.Tx, st runState, f fetched) error {
	q := models.New(tx)

	current, err := findRun(ctx, q, st.run.WorkspaceID, st.run.ID)
	if err != nil {
		return err
	}

	st.run = current
	var added int32

	deltas := map[string]int64{f.kind: 1}
	if current.Status == "running" {
		if added, err = e.follow(ctx, tx, st, f.links); err != nil {
			return err
		}

		deltas["discovered"] = int64(added)
	}

	run, err := e.addWork(ctx, q, current, work{pending: added - 1, deltas: deltas})
	if err != nil {
		return err
	}

	return e.finishWhenDone(ctx, tx, run)
}

/*
follow records the links a document handed back as documents of the
run, within the source's limit, and queues their fetch. Links the
source's patterns leave out are not followed at all, so they never use
up the limit.
*/
func (e *Engine) follow(ctx context.Context, tx *database.Tx, st runState, links []string) (int32, error) {
	room := capped(st.limit - int(stat(st.run, "discovered")))

	var added int32

	for _, link := range links {
		if added >= room {
			break
		}

		if !sources.Allowed(st.source.Include, st.source.Exclude, pathOf(link)) {
			continue
		}

		queued, _, err := e.see(ctx, tx, st, sources.Ref{ExternalID: link, URL: link, Path: pathOf(link)})
		if err != nil {
			return added, err
		}

		if queued {
			added++
		}
	}

	return added, nil
}

/*
visibilityOf maps a driver's visibility to the index's, internal when
the driver did not say.
*/
func visibilityOf(v sources.Visibility) contracts.KnowledgeVisibility {
	if v == sources.VisibilityPublic {
		return contracts.KnowledgeVisibilityPublic
	}

	return contracts.KnowledgeVisibilityAgentOnly
}

/*
truncateUTF8 cuts text to at most n bytes without splitting a rune.
*/
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}

	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}

	return s[:n]
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func cmpString(a, b string) string {
	if a != "" {
		return a
	}

	return b
}

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}

	return list
}

/*
lockRun locks the run, so the writes of one run's jobs take turns.
*/
func lockRun(ctx context.Context, q *models.Queries, run models.KnowledgeSyncRun) error {
	_, err := q.LockKnowledgeSyncRun(ctx, models.LockKnowledgeSyncRunParams{WorkspaceID: run.WorkspaceID, ID: run.ID})

	return wrap("lock sync run", err)
}

/*
capped keeps a count within what a run counter holds.
*/
func capped(n int) int32 {
	if n > maxDocuments {
		return maxDocuments
	}

	if n < 0 {
		return 0
	}

	return int32(n)
}
