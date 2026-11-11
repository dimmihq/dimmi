/*
|--------------------------------------------------------------------------
| Source Documents
|--------------------------------------------------------------------------
|
| A source's page lists its documents by where they sit, filtered by
| status: live, failed or excluded. A member may set one document aside
| for good, and it stays aside through every later sync until they let
| it back in; letting it back in, or asking for one document again,
| reads just that document without walking the whole source.
|
*/

package knowledgesources

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DocumentFilter narrows a source's documents: statuses, words in the
title or path, and the last document of the previous page.
*/
type DocumentFilter struct {
	SourceID  ids.UUID
	Statuses  []string
	Query     string
	AfterPath *string
	AfterID   ids.UUID
	Limit     int32
}

/*
Documents lists a source's documents, a page at a time.
*/
func (e *Engine) Documents(ctx context.Context, m authz.Membership, f DocumentFilter,
) ([]models.ListKnowledgeDocumentsRow, error) {
	if !m.Can("settings:read") {
		return nil, exceptions.ErrForbidden
	}

	limit := f.Limit
	if limit <= 0 || limit > documentsLimit {
		limit = documentsLimit
	}

	var out []models.ListKnowledgeDocumentsRow

	err := e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		if _, err := findSource(ctx, q, m.Workspace.ID, f.SourceID); err != nil {
			return err
		}

		statuses := f.Statuses
		if statuses == nil {
			statuses = []string{}
		}

		var err error

		out, err = q.ListKnowledgeDocuments(ctx, models.ListKnowledgeDocumentsParams{
			WorkspaceID: m.Workspace.ID, SourceID: f.SourceID, Statuses: statuses, Query: f.Query,
			AfterPath: f.AfterPath, AfterID: f.AfterID, RowLimit: limit,
		})

		return wrap("list source documents", err)
	})

	return out, err
}

/*
Exclude sets a document aside for good and drops it from the index.
*/
func (e *Engine) Exclude(ctx context.Context, m authz.Membership, sourceID, documentID ids.UUID) error {
	if !m.Can("knowledge:manage") {
		return exceptions.ErrForbidden
	}

	return e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		doc, err := findDocument(ctx, q, m.Workspace.ID, docKey{source: sourceID, id: documentID})
		if err != nil {
			return err
		}

		if err := q.ExcludeKnowledgeDocument(ctx, models.ExcludeKnowledgeDocumentParams{
			ExcludedBy: new("member"), Now: e.Clock.Now(), WorkspaceID: doc.WorkspaceID, ID: doc.ID,
		}); err != nil {
			return fmt.Errorf("exclude document: %w", err)
		}

		return wrap("drop excluded chunks", q.DeleteKnowledgeDocumentChunks(ctx,
			models.DeleteKnowledgeDocumentChunksParams{WorkspaceID: doc.WorkspaceID, DocumentID: doc.ID}))
	})
}

/*
Include lets a set-aside document back in and reads it again.
*/
func (e *Engine) Include(ctx context.Context, m authz.Membership, sourceID, documentID ids.UUID,
) (models.KnowledgeSyncRun, error) {
	if !m.Can("knowledge:manage") {
		return models.KnowledgeSyncRun{}, exceptions.ErrForbidden
	}

	return e.refetch(ctx, m, docKey{source: sourceID, id: documentID}, e.letIn)
}

/*
Resync reads one document again now.
*/
func (e *Engine) Resync(ctx context.Context, m authz.Membership, sourceID, documentID ids.UUID,
) (models.KnowledgeSyncRun, error) {
	if !m.Can("knowledge:sync") {
		return models.KnowledgeSyncRun{}, exceptions.ErrForbidden
	}

	return e.refetch(ctx, m, docKey{source: sourceID, id: documentID}, nil)
}

/*
prepareDoc changes a document before it is fetched again.
*/
type prepareDoc func(ctx context.Context, q *models.Queries, doc models.KnowledgeDocument) error

/*
letIn lets a set-aside document back in.
*/
func (e *Engine) letIn(ctx context.Context, q *models.Queries, doc models.KnowledgeDocument) error {
	return wrap("include document", q.IncludeKnowledgeDocument(ctx, models.IncludeKnowledgeDocumentParams{
		Now: e.Clock.Now(), WorkspaceID: doc.WorkspaceID, ID: doc.ID,
	}))
}

/*
refetch queues one document's fetch on the running sync, or on a small
run of its own that removes nothing.
*/
func (e *Engine) refetch(ctx context.Context, m authz.Membership, key docKey, first prepareDoc,
) (models.KnowledgeSyncRun, error) {
	var out models.KnowledgeSyncRun

	err := e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		doc, err := findDocument(ctx, q, m.Workspace.ID, key)
		if err != nil {
			return err
		}

		if first != nil {
			if err := first(ctx, q, doc); err != nil {
				return err
			}
		}

		if out, err = e.documentRun(ctx, tx, doc); err != nil {
			return err
		}

		args := jobs.FetchKnowledgeDocumentArgs{WorkspaceID: doc.WorkspaceID, RunID: out.ID, DocumentID: doc.ID}

		return wrap("queue document fetch", e.Queue.PushTx(ctx, tx, args))
	})

	return out, err
}

/*
documentRun returns the running sync with one more fetch counted, or
starts a run for the one document.
*/
func (e *Engine) documentRun(ctx context.Context, tx *database.Tx, doc models.KnowledgeDocument,
) (models.KnowledgeSyncRun, error) {
	q, now := models.New(tx), e.Clock.Now()

	run, err := q.FindActiveKnowledgeSyncRun(ctx, models.FindActiveKnowledgeSyncRunParams{
		WorkspaceID: doc.WorkspaceID, SourceID: doc.SourceID,
	})
	if err == nil {
		return e.addWork(ctx, q, run, work{pending: 1, deltas: map[string]int64{"discovered": 1}})
	}

	if !errors.Is(err, database.ErrNotFound) {
		return run, fmt.Errorf("find running sync: %w", err)
	}

	run, err = q.CreateKnowledgeSyncRun(ctx, models.CreateKnowledgeSyncRunParams{
		ID: ids.NewV7At(now), WorkspaceID: doc.WorkspaceID, SourceID: doc.SourceID, Trigger: "document",
		FullRun: false, Now: &now,
	})
	if err != nil {
		return run, fmt.Errorf("start document sync: %w", err)
	}

	if _, err := q.MarkKnowledgeSyncDiscovered(ctx, models.MarkKnowledgeSyncDiscoveredParams{
		Now: now, WorkspaceID: run.WorkspaceID, ID: run.ID,
	}); err != nil {
		return run, fmt.Errorf("mark document sync discovered: %w", err)
	}

	return e.addWork(ctx, q, run, work{pending: 1, deltas: map[string]int64{"discovered": 1}})
}

/*
docKey names a document of a source.
*/
type docKey struct {
	source ids.UUID
	id     ids.UUID
}

/*
findDocument returns a document of the source, or 404.
*/
func findDocument(ctx context.Context, q *models.Queries, ws ids.UUID, key docKey) (models.KnowledgeDocument, error) {
	doc, err := q.FindKnowledgeDocument(ctx, models.FindKnowledgeDocumentParams{WorkspaceID: ws, ID: key.id})
	if errors.Is(err, database.ErrNotFound) || (err == nil && doc.SourceID != key.source) {
		return doc, ErrDocumentNotFound
	}

	return doc, wrap("find document", err)
}
