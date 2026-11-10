/*
|--------------------------------------------------------------------------
| Workspace Embedding Rounds
|--------------------------------------------------------------------------
|
| A model is built for a whole workspace by indexing any live article
| not indexed yet and queueing every hash the model lacks; the batches
| switch the model in once they cover everything. A workspace with
| nothing to embed switches at once. The nightly sweep gives every
| workspace its default binding, retries inputs a provider refused a
| few times, rebuilds what is missing for each model in use, and
| deletes the vectors of models out of use for a week; the model a
| workspace switched away from is kept that long, so switching back is
| instant.
|
*/

package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Round sizes: articles indexed per transaction, hashes queued per build,
the tries a refused input gets, vectors deleted per statement and how
long a model out of use is kept.
*/
const (
	indexBatch      = 100
	buildLimit      = 100_000
	refusalAttempts = 5
	purgeBatch      = 1000
	keepUnused      = 7 * 24 * time.Hour
)

/*
ReembedWorkspaceKnowledge is the worker of
jobs.ReembedWorkspaceKnowledgeArgs.
*/
type ReembedWorkspaceKnowledge struct {
	Vectors *Vectors
}

/*
Handle builds the model for the workspace.
*/
func (w *ReembedWorkspaceKnowledge) Handle(ctx context.Context,
	job *queue.Job[jobs.ReembedWorkspaceKnowledgeArgs],
) error {
	return w.Vectors.Reembed(ctx, job.Args.WorkspaceID, job.Args.ModelKey)
}

/*
Reembed indexes the workspace's unindexed articles, queues every hash
the model lacks and records the progress.
*/
func (v *Vectors) Reembed(ctx context.Context, ws ids.UUID, model string) error {
	if err := v.indexUnindexed(ctx, ws); err != nil {
		return err
	}
	var hashes []string
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		_, ok, err := bindingFor(ctx, q, ws, model)
		if err != nil || !ok {
			return err
		}
		hashes, err = q.ListMissingEmbedHashes(ctx, models.ListMissingEmbedHashesParams{
			WorkspaceID: ws, ModelKey: model, RowLimit: buildLimit,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("list missing embeddings: %w", err)
	}
	args := jobs.EmbedKnowledgeChunksArgs{WorkspaceID: ws, ModelKey: model}
	if err := v.pushBatches(ctx, nil, args, hashes); err != nil {
		return err
	}
	return v.progress(ctx, ws, model)
}

/*
indexUnindexed indexes the live articles that have no chunks yet, a
batch per transaction.
*/
func (v *Vectors) indexUnindexed(ctx context.Context, ws ids.UUID) error {
	for {
		var n int
		err := v.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
			articles, err := q.ListUnindexedArticles(ctx, models.ListUnindexedArticlesParams{
				WorkspaceID: ws, RowLimit: indexBatch,
			})
			n = len(articles)
			for _, id := range articles {
				if err == nil {
					err = v.IndexArticle(ctx, tx, ws, id)
				}
			}
			return err
		})
		if err != nil {
			return fmt.Errorf("index unindexed articles: %w", err)
		}
		if n < indexBatch {
			return nil
		}
	}
}

/*
SweepKnowledgeEmbeddings is the worker of
jobs.SweepKnowledgeEmbeddingsArgs.
*/
type SweepKnowledgeEmbeddings struct {
	Vectors *Vectors
}

/*
Handle sweeps every workspace.
*/
func (w *SweepKnowledgeEmbeddings) Handle(ctx context.Context, _ *queue.Job[jobs.SweepKnowledgeEmbeddingsArgs]) error {
	workspaces, err := models.New(w.Vectors.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces to sweep: %w", err)
	}
	for _, workspace := range workspaces {
		if err := w.Vectors.Sweep(ctx, workspace.ID); err != nil {
			return err
		}
	}
	return nil
}

/*
Sweep binds the workspace's default model when it has none, retries
refused inputs, queues a build of every model in use and purges models
out of use.
*/
func (v *Vectors) Sweep(ctx context.Context, ws ids.UUID) error {
	var s models.KnowledgeSetting
	retries := map[string][]string{}
	err := v.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		var err error
		if s, err = v.ensure(ctx, tx, ws); err != nil {
			return err
		}
		for _, model := range embeddingModels(s) {
			if retries[model], err = q.ListFailedEmbedHashes(ctx, models.ListFailedEmbedHashesParams{
				WorkspaceID: ws, ModelKey: model, MaxAttempts: refusalAttempts, RowLimit: buildLimit,
			}); err != nil {
				return fmt.Errorf("list refused inputs: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sweep knowledge: %w", err)
	}
	for model, hashes := range retries {
		args := jobs.EmbedKnowledgeChunksArgs{WorkspaceID: ws, ModelKey: model}
		if err := v.pushBatches(ctx, nil, args, hashes); err != nil {
			return err
		}
		if err := v.Queue.Push(ctx, jobs.ReembedWorkspaceKnowledgeArgs{WorkspaceID: ws, ModelKey: model}); err != nil {
			return fmt.Errorf("queue knowledge build: %w", err)
		}
	}
	return v.purge(ctx, ws, keptModels(s, v.Clock.Now()))
}

/*
keptModels are the models whose vectors stay: those in use and the
one switched away from within the week.
*/
func keptModels(s models.KnowledgeSetting, now time.Time) []string {
	keep := embeddingModels(s)
	if s.PreviousModelKey != nil && s.SwitchedAt != nil && now.Sub(*s.SwitchedAt) < keepUnused {
		keep = append(keep, *s.PreviousModelKey)
	}
	return keep
}

/*
purge deletes the vectors and refusals of every model not kept, a
batch per transaction.
*/
func (v *Vectors) purge(ctx context.Context, ws ids.UUID, keep []string) error {
	scoped := tenancy.With(ctx, ws.String())
	for {
		var n int64
		err := v.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			q := models.New(tx)
			var err error
			if n, err = q.PurgeUnusedEmbeddings(ctx, models.PurgeUnusedEmbeddingsParams{
				WorkspaceID: ws, Keep: keep, Batch: purgeBatch,
			}); err != nil {
				return err
			}
			return q.PurgeUnusedEmbeddingFailures(ctx, models.PurgeUnusedEmbeddingFailuresParams{
				WorkspaceID: ws, Keep: keep,
			})
		})
		if err != nil {
			return fmt.Errorf("purge unused embeddings: %w", err)
		}
		if n < purgeBatch {
			return nil
		}
	}
}
