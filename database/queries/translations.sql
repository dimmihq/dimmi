-- name: ListArticleTranslations :many
-- ListArticleTranslations returns an article's translations, each with
-- whether the source changed since it last caught up.
SELECT t.*, (t.source_hash IS DISTINCT FROM help_text_hash(
        coalesce(v.title, a.title), coalesce(v.description, a.description), coalesce(v.body_markdown, a.body_markdown)
    ))::boolean AS outdated
FROM article_translations t
JOIN articles a ON a.id = t.article_id
LEFT JOIN article_versions v ON v.id = a.published_version_id
WHERE t.workspace_id = @workspace_id AND t.article_id = @article_id
ORDER BY t.locale;

-- name: FindArticleTranslation :one
-- FindArticleTranslation returns one translation of an article, with
-- whether the source changed since it last caught up.
SELECT t.*, (t.source_hash IS DISTINCT FROM help_text_hash(
        coalesce(v.title, a.title), coalesce(v.description, a.description), coalesce(v.body_markdown, a.body_markdown)
    ))::boolean AS outdated
FROM article_translations t
JOIN articles a ON a.id = t.article_id
LEFT JOIN article_versions v ON v.id = a.published_version_id
WHERE t.workspace_id = @workspace_id AND t.article_id = @article_id AND t.locale = @locale;

-- name: LockArticleTranslation :one
-- LockArticleTranslation returns a translation and holds it until the
-- transaction ends.
SELECT * FROM article_translations
WHERE workspace_id = @workspace_id AND article_id = @article_id AND locale = @locale
FOR UPDATE;

-- name: ArticleSourceHash :one
-- ArticleSourceHash is the text hash of what the source says: its live
-- version, or its working copy when it was never published.
SELECT help_text_hash(
        coalesce(v.title, a.title), coalesce(v.description, a.description), coalesce(v.body_markdown, a.body_markdown)
    )::text AS hash
FROM articles a
LEFT JOIN article_versions v ON v.id = a.published_version_id
WHERE a.workspace_id = @workspace_id AND a.id = @id;

-- name: ArticleTranslationSlugTaken :one
-- ArticleTranslationSlugTaken reports whether another article's
-- translation into the language has the slug.
SELECT EXISTS (
    SELECT 1 FROM article_translations t
    JOIN articles a ON a.id = t.article_id AND a.deleted_at IS NULL
    WHERE t.workspace_id = @workspace_id AND t.help_center_id = @help_center_id AND t.locale = @locale
      AND t.slug = @slug AND t.article_id <> @article_id
) AS taken;

-- name: FreeArticleTranslationSlug :exec
-- FreeArticleTranslationSlug lets a translation of an article in the
-- trash give up its slug, so a live article may take it.
UPDATE article_translations t SET slug = left(t.slug, 110) || '-' || substr(md5(t.id::text), 1, 8),
    updated_at = @now
FROM articles a
WHERE a.id = t.article_id AND a.deleted_at IS NOT NULL
  AND t.workspace_id = @workspace_id AND t.help_center_id = @help_center_id AND t.locale = @locale
  AND t.slug = @slug;

-- name: CreateArticleTranslation :one
-- CreateArticleTranslation stores a new draft translation.
INSERT INTO article_translations (
    id, workspace_id, help_center_id, article_id, locale, title, slug, description, body_markdown, content_hash,
    source_hash, author_member_id, last_edited_by_type, last_edited_by_id, word_count, created_at, updated_at
) VALUES (
    @id, @workspace_id, @help_center_id, @article_id, @locale, @title, @slug, @description, @body_markdown,
    @content_hash, @source_hash, @author_member_id, @edited_by_type, @edited_by_id, @word_count, @now, @now
)
RETURNING *;

-- name: UpdateArticleTranslation :one
-- UpdateArticleTranslation writes a translation's working copy, if
-- nobody changed it since the lock version the editor read.
UPDATE article_translations SET
    title = @title, slug = @slug, description = @description, body_markdown = @body_markdown,
    content_hash = @content_hash, has_unpublished_changes = @has_unpublished_changes,
    last_edited_by_type = @edited_by_type, last_edited_by_id = @edited_by_id, word_count = @word_count,
    lock_version = lock_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND lock_version = @lock_version
RETURNING *;

-- name: CreateArticleTranslationVersion :one
-- CreateArticleTranslationVersion freezes a copy of a translation,
-- numbered in the same run as the article's other versions.
INSERT INTO article_versions (
    id, workspace_id, article_id, locale, number, kind, title, slug, description, body_markdown, body_html, toc,
    visibility, collection_id, content_hash, created_by_type, created_by_id, renderer_version, created_at
) VALUES (
    @id, @workspace_id, @article_id, @locale, @number, @kind, @title, @slug, @description, @body_markdown,
    @body_html, @toc, @visibility, @collection_id, @content_hash, @created_by_type, @created_by_id,
    @renderer_version, @now
)
RETURNING *;

-- name: PublishArticleTranslation :one
-- PublishArticleTranslation points the translation at the version just
-- frozen; publishing catches it up with the source.
UPDATE article_translations SET
    status = 'published', published_version_id = @version_id, has_unpublished_changes = false,
    source_hash = @source_hash, published_at = @now::timestamptz,
    first_published_at = coalesce(first_published_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: UnpublishArticleTranslation :one
-- UnpublishArticleTranslation takes a translation back to a draft,
-- keeping its versions.
UPDATE article_translations SET
    status = 'draft', published_version_id = NULL, has_unpublished_changes = true, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: MarkArticleTranslationCurrent :one
-- MarkArticleTranslationCurrent says a translation caught up with the
-- source as it now is.
UPDATE article_translations SET source_hash = @source_hash, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteArticleTranslation :exec
-- DeleteArticleTranslation removes a translation; its versions stay in
-- the article's history.
DELETE FROM article_translations WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListArticleTranslationVersions :many
-- ListArticleTranslationVersions returns the versions of an article in
-- one language, newest first.
SELECT id, number, kind, title, created_by_type, created_by_id, created_at FROM article_versions
WHERE workspace_id = @workspace_id AND article_id = @article_id AND locale = @locale
ORDER BY number DESC;

-- name: ListHelpCenterTranslationStates :many
-- ListHelpCenterTranslationStates returns every live article of a help
-- center that is not archived, with its translation into each language
-- it has, for the lists of missing and outdated translations.
SELECT a.id, a.title, a.status, a.collection_id, a.updated_at, t.locale, t.status AS translation_status,
    t.title AS translation_title, t.updated_at AS translation_updated_at,
    (t.id IS NOT NULL AND t.source_hash IS DISTINCT FROM help_text_hash(
        coalesce(v.title, a.title), coalesce(v.description, a.description), coalesce(v.body_markdown, a.body_markdown)
    ))::boolean AS outdated
FROM articles a
LEFT JOIN article_versions v ON v.id = a.published_version_id
LEFT JOIN article_translations t ON t.article_id = a.id
WHERE a.workspace_id = @workspace_id AND a.help_center_id = @help_center_id AND a.deleted_at IS NULL
  AND a.status <> 'archived'
ORDER BY a.updated_at DESC, a.id, t.locale;

-- name: ListPublishedArticleTranslations :many
-- ListPublishedArticleTranslations returns what readers of each other
-- language see of a help center: every published translation of a
-- public article that is not archived, with its live version, in
-- collection order.
SELECT t.locale, a.id, a.collection_id, a.position, t.published_at, v.title, v.slug, v.description,
    v.body_markdown, v.body_html, v.toc, v.id AS version_id, v.created_at AS version_created_at, v.renderer_version
FROM article_translations t
JOIN articles a ON a.id = t.article_id
JOIN article_versions v ON v.id = t.published_version_id
LEFT JOIN article_versions s ON s.id = a.published_version_id
WHERE t.workspace_id = @workspace_id AND t.help_center_id = @help_center_id AND t.status = 'published'
  AND a.deleted_at IS NULL AND a.status <> 'archived' AND coalesce(s.visibility, a.visibility) = 'public'
ORDER BY t.locale, a.collection_id NULLS LAST, a.position, a.id;

-- name: SearchPublishedArticleTranslations :many
-- SearchPublishedArticleTranslations finds the published translations
-- into one language matching a query, best first.
SELECT a.id, a.collection_id, v.title, v.slug, v.description, v.body_markdown,
    ts_rank_cd(v.search_document, query)::real AS rank,
    ts_headline('dimmi_search', v.body_markdown, query,
        'MaxWords=30, MinWords=12, ShortWord=2, MaxFragments=1, StartSel=<mark>, StopSel=</mark>')::text AS snippet
FROM article_translations t
JOIN articles a ON a.id = t.article_id
JOIN article_versions v ON v.id = t.published_version_id
LEFT JOIN article_versions s ON s.id = a.published_version_id,
    websearch_to_tsquery('dimmi_search', @query::text) AS query
WHERE t.workspace_id = @workspace_id AND t.help_center_id = @help_center_id AND t.locale = @locale
  AND t.status = 'published' AND a.deleted_at IS NULL AND a.status <> 'archived'
  AND coalesce(s.visibility, a.visibility) = 'public' AND v.search_document @@ query
ORDER BY rank DESC, a.id
LIMIT @row_limit;

-- name: ListCollectionTranslations :many
-- ListCollectionTranslations returns every collection translation of a
-- help center, with whether the source changed since it was written.
SELECT t.*, (t.source_hash IS DISTINCT FROM help_text_hash(c.name, c.description, ''))::boolean AS outdated
FROM collection_translations t
JOIN collections c ON c.id = t.collection_id AND c.deleted_at IS NULL
WHERE t.workspace_id = @workspace_id AND t.help_center_id = @help_center_id
ORDER BY t.locale, c.position, c.id;

-- name: ListTranslationsOfCollection :many
-- ListTranslationsOfCollection returns one collection's translations.
SELECT t.*, (t.source_hash IS DISTINCT FROM help_text_hash(c.name, c.description, ''))::boolean AS outdated
FROM collection_translations t
JOIN collections c ON c.id = t.collection_id
WHERE t.workspace_id = @workspace_id AND t.collection_id = @collection_id
ORDER BY t.locale;

-- name: FindCollectionTranslation :one
-- FindCollectionTranslation returns one translation of a collection.
SELECT * FROM collection_translations
WHERE workspace_id = @workspace_id AND collection_id = @collection_id AND locale = @locale;

-- name: CollectionTranslationSlugTaken :one
-- CollectionTranslationSlugTaken reports whether another live
-- collection's translation into the language has the slug.
SELECT EXISTS (
    SELECT 1 FROM collection_translations t
    JOIN collections c ON c.id = t.collection_id AND c.deleted_at IS NULL
    WHERE t.workspace_id = @workspace_id AND t.help_center_id = @help_center_id AND t.locale = @locale
      AND t.slug = @slug AND t.collection_id <> @collection_id
) AS taken;

-- name: UpsertCollectionTranslation :one
-- UpsertCollectionTranslation writes a collection's translation into a
-- language, which catches it up with the source as it now is.
INSERT INTO collection_translations (
    workspace_id, help_center_id, collection_id, locale, name, slug, description, source_hash, created_at, updated_at
)
SELECT c.workspace_id, c.help_center_id, c.id, @locale, @name, @slug, @description,
    help_text_hash(c.name, c.description, ''), @now, @now
FROM collections c WHERE c.workspace_id = @workspace_id AND c.id = @collection_id
ON CONFLICT (collection_id, locale) DO UPDATE SET
    name = excluded.name, slug = excluded.slug, description = excluded.description,
    source_hash = excluded.source_hash, updated_at = excluded.updated_at
RETURNING *;

-- name: DeleteCollectionTranslation :exec
-- DeleteCollectionTranslation removes a collection's translation.
DELETE FROM collection_translations
WHERE workspace_id = @workspace_id AND collection_id = @collection_id AND locale = @locale;

-- name: UpsertLocalizedRedirect :exec
-- UpsertLocalizedRedirect points an old path of one language at its new
-- target in that language.
INSERT INTO help_center_redirects (
    workspace_id, help_center_id, from_path, target_type, target_article_id, target_collection_id, target_locale,
    source, created_at, updated_at
) VALUES (
    @workspace_id, @help_center_id, @from_path, @target_type, @target_article_id, @target_collection_id,
    @target_locale, @source, @now, @now
)
ON CONFLICT (help_center_id, from_path) DO UPDATE SET
    target_type = excluded.target_type, target_article_id = excluded.target_article_id,
    target_collection_id = excluded.target_collection_id, target_locale = excluded.target_locale,
    target_url = NULL, source = excluded.source, updated_at = excluded.updated_at;

-- name: FindLocalizedRedirect :one
-- FindLocalizedRedirect returns where an old path now leads, with the
-- slug of the target in the redirect's language.
SELECT r.target_locale, r.target_url,
    coalesce(CASE WHEN r.target_locale IS NULL THEN a.slug ELSE at.slug END, '')::text AS article_slug,
    coalesce(CASE WHEN r.target_locale IS NULL THEN c.slug ELSE ct.slug END, '')::text AS collection_slug
FROM help_center_redirects r
LEFT JOIN articles a ON a.id = r.target_article_id AND a.deleted_at IS NULL
LEFT JOIN article_translations at ON at.article_id = a.id AND at.locale = r.target_locale
LEFT JOIN collections c ON c.id = r.target_collection_id AND c.deleted_at IS NULL
LEFT JOIN collection_translations ct ON ct.collection_id = c.id AND ct.locale = r.target_locale
WHERE r.workspace_id = @workspace_id AND r.help_center_id = @help_center_id AND r.from_path = @from_path;

-- name: SetHelpCenterLocales :one
-- SetHelpCenterLocales writes a help center's default language and the
-- languages readers may pick, and moves its cache version on.
UPDATE help_centers SET locale = @locale, locales = @locales::text[], cache_version = cache_version + 1,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: CountTranslationsInLocale :one
-- CountTranslationsInLocale counts a help center's article and
-- collection translations into one language.
SELECT ((SELECT count(*) FROM article_translations t
         WHERE t.workspace_id = @workspace_id AND t.help_center_id = @help_center_id AND t.locale = @locale)
    + (SELECT count(*) FROM collection_translations c
         WHERE c.workspace_id = @workspace_id AND c.help_center_id = @help_center_id AND c.locale = @locale))::bigint
    AS count;

-- name: FindTranslatedArticleBySlug :one
-- FindTranslatedArticleBySlug returns the article whose published
-- translation into the language readers reach at the slug.
SELECT a.id
FROM article_translations t
JOIN articles a ON a.id = t.article_id
LEFT JOIN article_versions s ON s.id = a.published_version_id
WHERE t.workspace_id = @workspace_id AND t.help_center_id = @help_center_id AND t.locale = @locale
  AND t.slug = @slug AND t.status = 'published' AND a.deleted_at IS NULL AND a.status <> 'archived'
  AND coalesce(s.visibility, a.visibility) = 'public';
