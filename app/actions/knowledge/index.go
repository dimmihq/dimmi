/*
|--------------------------------------------------------------------------
| Article Index
|--------------------------------------------------------------------------
|
| An article's stored chunks follow its live published version, and the
| live versions of its published translations, inside the transaction
| that changed it: a publish stores the new version's
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
	"strings"

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
	var keep []ids.UUID
	var live []chunkable
	source, err := q.FindLiveArticleForIndex(ctx, models.FindLiveArticleForIndexParams{WorkspaceID: ws, ID: articleID})
	switch {
	case err == nil:
		keep = append(keep, source.VersionID)
		live = append(live, chunkable{
			article: articleID, center: source.HelpCenterID, version: source.VersionID, title: source.Title,
			body: source.BodyMarkdown, visibility: source.Visibility,
		})
	case !errors.Is(err, database.ErrNotFound):
		return fmt.Errorf("find article to index: %w", err)
	}
	translations, err := q.ListLiveTranslationsForIndex(ctx, models.ListLiveTranslationsForIndexParams{
		WorkspaceID: ws, ArticleID: articleID,
	})
	if err != nil {
		return fmt.Errorf("find translations to index: %w", err)
	}
	if len(translations) > 0 && len(live) == 0 {
		a, err := q.FindArticle(ctx, models.FindArticleParams{WorkspaceID: ws, ID: articleID})
		if err != nil {
			return fmt.Errorf("find article to index: %w", err)
		}
		source.HelpCenterID = a.HelpCenterID
	}
	for _, t := range translations {
		keep = append(keep, t.VersionID)
		live = append(live, chunkable{
			article: articleID, center: source.HelpCenterID, version: t.VersionID, title: t.Title,
			body: t.BodyMarkdown, visibility: t.Visibility, locale: &t.Locale,
		})
	}
	if err := q.DeleteStaleKnowledgeChunks(ctx, models.DeleteStaleKnowledgeChunksParams{
		WorkspaceID: ws, ArticleID: articleID, KeepVersionIds: orNoVersions(keep),
	}); err != nil {
		return wrapIndex(err)
	}
	if len(live) == 0 {
		return nil
	}
	for _, c := range live {
		if err := v.storeChunks(ctx, q, ws, c); err != nil {
			return err
		}
	}
	return v.queueArticle(ctx, tx, ws, articleID)
}

/*
chunkable is one live version of an article, the source's or a
translation's, as the index chunks it.
*/
type chunkable struct {
	article    ids.UUID
	center     ids.UUID
	version    ids.UUID
	title      string
	body       string
	visibility string
	locale     *string
}

func orNoVersions(list []ids.UUID) []ids.UUID {
	if list == nil {
		return []ids.UUID{}
	}
	return list
}

/*
storeChunks stores the version's chunks unless they are stored.
*/
func (v *Vectors) storeChunks(ctx context.Context, q *models.Queries, ws ids.UUID, live chunkable) error {
	n, err := q.CountKnowledgeChunksForVersion(ctx, models.CountKnowledgeChunksForVersionParams{
		WorkspaceID: ws, VersionID: live.version,
	})
	if err != nil || n > 0 {
		return wrapIndex(err)
	}
	chunks := ChunkArticle(Article{
		Title: live.title, Body: live.body, VersionID: live.version,
		Visibility: contracts.KnowledgeVisibility(live.visibility),
	})
	now := v.Clock.Now()
	for i, c := range chunks {
		err := q.InsertKnowledgeChunk(ctx, models.InsertKnowledgeChunkParams{
			ID: c.ID, WorkspaceID: ws, ArticleID: live.article, ArticleVersionID: live.version,
			HelpCenterID: live.center, Ordinal: int32(i), Title: c.Title, HeadingPath: c.HeadingPath,
			Body: c.Text, Visibility: string(c.Visibility),
			EmbedHash: EmbedHash(EmbedInput(c.Title, c.HeadingPath, c.Text)), Locale: live.locale, Now: now,
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

/*
ReaderLocales are the languages an article may answer a reader in, best
first: the reader's own tag and its base language, so fr-CA finds a
French translation too. None for a reader of no known language.
*/
func ReaderLocales(raw string) []string {
	tag := strings.ReplaceAll(strings.TrimSpace(raw), "_", "-")
	if tag == "" {
		return []string{}
	}
	lang, region, hasRegion := strings.Cut(tag, "-")
	lang = strings.ToLower(lang)
	if !hasRegion {
		return []string{lang}
	}
	if len(region) == 2 {
		region = strings.ToUpper(region)
	}
	return []string{lang + "-" + region, lang}
}
