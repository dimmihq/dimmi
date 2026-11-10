/*
|--------------------------------------------------------------------------
| Article Index
|--------------------------------------------------------------------------
|
| An article's stored chunks follow its live published version inside
| the transaction that changed it: a publish stores the new version's
| chunks and drops the old ones, and an unpublish, archive or trash
| drops them all. The vectors follow in the background, queued in the
| same transaction for every model the workspace embeds with, the
| active one and one being built, so a switch never leaves holes.
| Searches also check the article is still live, so a chunk left
| behind by any other path is never found.
|
*/

package knowledge

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IndexArticle brings the article's stored chunks in line with its live
published version and queues the vectors they lack.
*/
func (v *Vectors) IndexArticle(ctx context.Context, tx *database.Tx, ws, articleID ids.UUID) error {
	q := models.New(tx)
	live, err := q.FindLiveArticleForIndex(ctx, models.FindLiveArticleForIndexParams{WorkspaceID: ws, ID: articleID})
	if errors.Is(err, database.ErrNotFound) {
		return wrapIndex(q.DeleteStaleKnowledgeChunks(ctx, models.DeleteStaleKnowledgeChunksParams{
			WorkspaceID: ws, ArticleID: articleID,
		}))
	}
	if err != nil {
		return fmt.Errorf("find article to index: %w", err)
	}
	if err := q.DeleteStaleKnowledgeChunks(ctx, models.DeleteStaleKnowledgeChunksParams{
		WorkspaceID: ws, ArticleID: articleID, KeepVersionID: &live.VersionID,
	}); err != nil {
		return wrapIndex(err)
	}
	if err := v.storeChunks(ctx, q, live); err != nil {
		return err
	}
	return v.queueArticle(ctx, tx, ws, articleID)
}

/*
storeChunks stores the version's chunks unless they are stored.
*/
func (v *Vectors) storeChunks(ctx context.Context, q *models.Queries, live models.FindLiveArticleForIndexRow) error {
	ws := live.WorkspaceID
	n, err := q.CountKnowledgeChunksForVersion(ctx, models.CountKnowledgeChunksForVersionParams{
		WorkspaceID: ws, VersionID: live.VersionID,
	})
	if err != nil || n > 0 {
		return wrapIndex(err)
	}
	chunks := ChunkArticle(Article{
		Title: live.Title, Body: live.BodyMarkdown, VersionID: live.VersionID,
		Visibility: contracts.KnowledgeVisibility(live.Visibility),
	})
	now := v.Clock.Now()
	for i, c := range chunks {
		err := q.InsertKnowledgeChunk(ctx, models.InsertKnowledgeChunkParams{
			ID: c.ID, WorkspaceID: ws, ArticleID: live.ID, ArticleVersionID: live.VersionID,
			HelpCenterID: live.HelpCenterID, Ordinal: int32(i), Title: c.Title, HeadingPath: c.HeadingPath,
			Body: c.Text, Visibility: string(c.Visibility),
			EmbedHash: EmbedHash(EmbedInput(c.Title, c.HeadingPath, c.Text)), Now: now,
		})
		if err != nil {
			return wrapIndex(err)
		}
	}
	return nil
}

/*
queueArticle queues the article's missing vectors for every model the
workspace embeds with, binding the default model first when the
workspace has none yet.
*/
func (v *Vectors) queueArticle(ctx context.Context, tx *database.Tx, ws, articleID ids.UUID) error {
	q := models.New(tx)
	s, err := v.ensure(ctx, tx, ws)
	if err != nil {
		return err
	}
	for _, model := range embeddingModels(s) {
		hashes, err := q.ListArticleEmbedHashes(ctx, models.ListArticleEmbedHashesParams{
			WorkspaceID: ws, ArticleID: articleID, ModelKey: model,
		})
		if err != nil {
			return wrapIndex(err)
		}
		args := jobs.EmbedKnowledgeChunksArgs{WorkspaceID: ws, ModelKey: model}
		if err := v.pushBatches(ctx, tx, args, hashes); err != nil {
			return err
		}
	}
	return nil
}

/*
pushBatches queues the hashes in batches, inside the transaction when
there is one.
*/
func (v *Vectors) pushBatches(ctx context.Context, tx *database.Tx, args jobs.EmbedKnowledgeChunksArgs,
	hashes []string,
) error {
	for start := 0; start < len(hashes); start += contracts.MaxEmbedBatch {
		batch := args
		batch.Hashes = hashes[start:min(start+contracts.MaxEmbedBatch, len(hashes))]
		var err error
		if tx != nil {
			err = v.Queue.PushTx(ctx, tx, batch)
		} else {
			err = v.Queue.Push(ctx, batch)
		}
		if err != nil {
			return fmt.Errorf("queue embeddings: %w", err)
		}
	}
	return nil
}

/*
embeddingModels are the models new content is embedded with: the
active one and the one being built.
*/
func embeddingModels(s models.KnowledgeSetting) []string {
	var out []string
	for _, m := range []*string{s.ActiveModelKey, s.PendingModelKey} {
		if m != nil && *m != "" {
			out = append(out, *m)
		}
	}
	return out
}

/*
wrapIndex wraps a failure of the index, nil staying nil.
*/
func wrapIndex(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("index article: %w", err)
}
