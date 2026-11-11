/*
|--------------------------------------------------------------------------
| Importing Into the Help Center
|--------------------------------------------------------------------------
|
| An import turns a source's documents into editable articles of the
| chosen help center, a batch at a time, each document in its own
| transaction so one bad article never stops the rest. Its place at the
| source becomes collections, its pictures are copied so nothing points
| back at the old tool, a translation lands in the help center of its
| language when the workspace has one, and its old address redirects to
| the new article. A document imported before updates its article
| instead of making another, and a document that did not change since is
| left alone. Synced sources can be turned into articles the same way.
|
*/

package knowledgesources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxImage is the largest picture copied into an article.
*/
const maxImage = 8 << 20

/*
Convert turns a synced source's documents into articles of the help
center.
*/
func (e *Engine) Convert(ctx context.Context, m authz.Membership, sourceID, helpCenterID ids.UUID) error {
	if !m.Can("knowledge:manage") || !m.Can("articles:write") {
		return exceptions.ErrForbidden
	}

	return e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		s, err := findSource(ctx, q, m.Workspace.ID, sourceID)
		if err != nil {
			return err
		}

		if s.DocumentCount == 0 {
			return ErrNothingToImport
		}

		if _, err := q.UpdateKnowledgeSource(ctx, models.UpdateKnowledgeSourceParams{
			Name: s.Name, Targets: s.Targets, HelpCenterID: &helpCenterID, Config: s.Config, Include: s.Include,
			Exclude: s.Exclude, Schedule: s.Schedule, NextSyncAt: s.NextSyncAt, Now: e.Clock.Now(),
			WorkspaceID: s.WorkspaceID, ID: s.ID,
		}); err != nil {
			return fmt.Errorf("point source at help center: %w", err)
		}

		args := jobs.ImportKnowledgeSourceArgs{WorkspaceID: s.WorkspaceID, SourceID: s.ID}

		return wrap("queue import", e.Queue.PushTx(ctx, tx, args))
	})
}

/*
Import writes one batch of a source's changed documents as articles and
queues the next batch while there are more.
*/
func (e *Engine) Import(ctx context.Context, job *queue.Job[jobs.ImportKnowledgeSourceArgs]) error {
	a := job.Args

	var (
		source models.KnowledgeSource
		batch  []models.KnowledgeDocument
	)

	err := e.inWorkspace(ctx, a.WorkspaceID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error
		if source, err = findSource(ctx, q, a.WorkspaceID, a.SourceID); err != nil {
			return err
		}

		batch, err = q.ListKnowledgeDocumentsToImport(ctx, models.ListKnowledgeDocumentsToImportParams{
			WorkspaceID: a.WorkspaceID, SourceID: a.SourceID, RowLimit: importBatch,
		})

		return err
	})
	if err != nil || source.HelpCenterID == nil {
		return cancelMissing(err)
	}

	for _, doc := range batch {
		if err := e.importOne(ctx, source, doc); err != nil {
			return err
		}
	}

	if len(batch) == importBatch {
		return wrap("queue next import batch", e.Queue.Push(ctx, a))
	}

	return nil
}

/*
importOne writes one document as an article in its own transaction.
*/
func (e *Engine) importOne(ctx context.Context, s models.KnowledgeSource, doc models.KnowledgeDocument) error {
	body := e.copyImages(ctx, doc)

	return e.inWorkspace(ctx, s.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		center, ok, err := helpCenterFor(ctx, tx, s, doc)
		if err != nil {
			return err
		}

		if !ok {
			return e.link(ctx, q, doc, nil)
		}

		collection, err := e.collectionsFor(ctx, tx, placeIn{ws: s.WorkspaceID, center: center}, doc)
		if err != nil {
			return err
		}

		art, err := e.Articles.ImportArticle(ctx, tx, helpcenters.ImportArticleInput{
			WorkspaceID: s.WorkspaceID, HelpCenterID: center, CollectionID: collection, ArticleID: doc.LinkedArticleID,
			Title: doc.Title, Body: body, Visibility: doc.Visibility, Publish: !doc.Draft,
		})
		if err != nil {
			return fmt.Errorf("import %s: %w", doc.ExternalID, err)
		}

		if err := e.Articles.ImportRedirect(ctx, tx, art, oldPath(deref(doc.Url))); err != nil {
			return err
		}

		return e.link(ctx, q, doc, &art.ID)
	})
}

/*
link records the article a document became, and that its current text
was imported.
*/
func (e *Engine) link(ctx context.Context, q *models.Queries, doc models.KnowledgeDocument, article *ids.UUID) error {
	if article == nil {
		article = doc.LinkedArticleID
	}

	return wrap("link imported article", q.LinkKnowledgeDocumentArticle(ctx, models.LinkKnowledgeDocumentArticleParams{
		ArticleID: article, ImportedHash: &doc.ContentHash, Now: e.Clock.Now(), WorkspaceID: doc.WorkspaceID,
		ID: doc.ID,
	}))
}

/*
helpCenterFor picks the help center a document goes to: the source's,
unless the document is in another language a help center of the
workspace is written in. A translation with no help center of its
language is skipped.
*/
func helpCenterFor(ctx context.Context, tx *database.Tx, s models.KnowledgeSource, doc models.KnowledgeDocument,
) (ids.UUID, bool, error) {
	target := *s.HelpCenterID

	locales, err := helpcenters.HelpCenterLocales(ctx, tx, s.WorkspaceID)
	if err != nil {
		return target, false, err
	}

	want := baseLocale(doc.Locale)
	if want == "" || baseLocale(locales[target]) == want {
		return target, true, nil
	}

	for id, locale := range locales {
		if baseLocale(locale) == want {
			return id, true, nil
		}
	}

	return target, doc.TranslationOf == nil, nil
}

/*
baseLocale is a locale's language, lowercase: "en-US" reads "en".
*/
func baseLocale(locale string) string {
	lang, _, _ := strings.Cut(strings.ToLower(strings.ReplaceAll(locale, "_", "-")), "-")

	return lang
}

/*
collectionsFor makes the document's collections in the help center,
folding levels deeper than the help center allows into the deepest,
and returns the innermost.
*/
func (e *Engine) collectionsFor(ctx context.Context, tx *database.Tx, at placeIn, doc models.KnowledgeDocument,
) (*ids.UUID, error) {
	var path []sources.Collection
	if err := json.Unmarshal(doc.Collection, &path); err != nil {
		return nil, fmt.Errorf("read collections of %s: %w", doc.ExternalID, err)
	}

	if len(path) == 0 {
		return nil, nil
	}

	if len(path) > helpcenters.MaxCollectionDepth {
		deepest := path[helpcenters.MaxCollectionDepth-1:]
		names := make([]string, len(deepest))

		for i, c := range deepest {
			names[i] = c.Name
		}

		folded := deepest[len(deepest)-1]
		folded.Name = strings.Join(names, " / ")
		path = append(path[:helpcenters.MaxCollectionDepth-1], folded)
	}

	var parent *ids.UUID

	for _, c := range path {
		id, err := e.Collections.ImportCollection(ctx, tx, helpcenters.ImportCollectionInput{
			WorkspaceID: at.ws, HelpCenterID: at.center, ParentID: parent, Name: c.Name, Description: c.Description,
		})
		if err != nil {
			return nil, err
		}

		if err := e.collectionRedirect(ctx, tx, at, moved{collection: id, from: c.URL}); err != nil {
			return nil, err
		}

		parent = &id
	}

	return parent, nil
}

/*
placeIn is the workspace and help center an import writes to.
*/
type placeIn struct {
	ws     ids.UUID
	center ids.UUID
}

/*
moved is a collection and the old address it had.
*/
type moved struct {
	collection ids.UUID
	from       string
}

/*
collectionRedirect makes a collection's old address lead to it.
*/
func (e *Engine) collectionRedirect(ctx context.Context, tx *database.Tx, at placeIn, m moved) error {
	from := oldPath(m.from)
	if from == "" {
		return nil
	}

	return wrap("record collection redirect", models.New(tx).UpsertCollectionRedirect(ctx,
		models.UpsertCollectionRedirectParams{
			WorkspaceID: at.ws, HelpCenterID: at.center, FromPath: from, CollectionID: &m.collection,
			Now: e.Clock.Now(),
		}))
}

/*
copyImages copies the document's pictures into the help center's
storage and points its Markdown at the copies; a picture that cannot
be copied keeps its old address.
*/
func (e *Engine) copyImages(ctx context.Context, doc models.KnowledgeDocument) string {
	body := doc.ContentMarkdown
	if e.Images == nil || e.HTTP == nil {
		return body
	}

	var assets []sources.Asset
	_ = json.Unmarshal(doc.Assets, &assets)

	for _, asset := range assets {
		if !strings.Contains(body, asset.URL) {
			continue
		}

		resp, err := e.HTTP.New().MaxBody(maxImage).Get(ctx, asset.URL)
		if err != nil || resp.Status != 200 {
			continue
		}

		kept, err := e.Images.Keep(ctx, bytes.NewReader(resp.Body))
		if err != nil {
			continue
		}

		body = strings.ReplaceAll(body, asset.URL, kept)
	}

	return body
}

/*
oldPath is the path of an old address, empty when there is none worth
redirecting.
*/
func oldPath(address string) string {
	u, err := url.Parse(address)
	if err != nil || u.Host == "" || u.Path == "" || u.Path == "/" {
		return ""
	}

	return u.EscapedPath()
}

/*
cancelMissing gives up a job whose source is gone; other errors retry.
*/
func cancelMissing(err error) error {
	if errors.Is(err, ErrSourceNotFound) {
		return queue.Cancel(err)
	}

	return err
}
