-- name: ListArticles :many
-- ListArticles returns a help center's live articles, filtered by status
-- and collection when given, newest edit first.
SELECT * FROM articles
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(collection_id)::uuid IS NULL OR collection_id = sqlc.narg(collection_id)::uuid)
  AND (sqlc.narg(query)::text IS NULL OR title_trgm LIKE '%' || lower(dimmi_unaccent(sqlc.narg(query)::text)) || '%')
ORDER BY updated_at DESC, id DESC
LIMIT @row_limit;

-- name: FindArticle :one
-- FindArticle returns one live article.
SELECT * FROM articles WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: LockArticle :one
-- LockArticle returns the article and holds it until the transaction
-- ends.
SELECT * FROM articles WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: ArticleSlugTaken :one
-- ArticleSlugTaken reports whether another live article of the help
-- center has the slug.
SELECT EXISTS (
    SELECT 1 FROM articles
    WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND slug = @slug
      AND deleted_at IS NULL AND id <> @id
) AS taken;

-- name: NextArticlePosition :one
-- NextArticlePosition is where a new article goes in its collection.
SELECT (coalesce(max(position), 0) + 1024)::integer AS position FROM articles
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
  AND collection_id IS NOT DISTINCT FROM sqlc.narg(collection_id)::uuid;

-- name: CreateArticle :one
-- CreateArticle stores a new draft.
INSERT INTO articles (
    id, workspace_id, help_center_id, collection_id, title, slug, description, body_markdown, visibility,
    content_hash, has_unpublished_changes, position, author_member_id, last_edited_by_type, last_edited_by_id,
    word_count, created_at, updated_at
) VALUES (
    @id, @workspace_id, @help_center_id, @collection_id, @title, @slug, @description, @body_markdown, @visibility,
    @content_hash, true, @position, @author_member_id, @edited_by_type, @edited_by_id, @word_count, @now, @now
)
RETURNING *;

-- name: UpdateArticle :one
-- UpdateArticle writes the working copy, if nobody changed it since the
-- lock version the editor read.
UPDATE articles SET
    collection_id = @collection_id, title = @title, slug = @slug, description = @description,
    body_markdown = @body_markdown, visibility = @visibility, content_hash = @content_hash,
    has_unpublished_changes = @has_unpublished_changes, last_edited_by_type = @edited_by_type,
    last_edited_by_id = @edited_by_id, word_count = @word_count, lock_version = lock_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND lock_version = @lock_version
RETURNING *;

-- name: NextArticleVersionNumber :one
-- NextArticleVersionNumber is the number the article's next version takes.
SELECT (coalesce(max(number), 0) + 1)::integer AS number FROM article_versions
WHERE workspace_id = @workspace_id AND article_id = @article_id;

-- name: CreateArticleVersion :one
-- CreateArticleVersion freezes a copy of the article.
INSERT INTO article_versions (
    id, workspace_id, article_id, number, kind, title, slug, description, body_markdown, body_html, toc,
    visibility, collection_id, content_hash, created_by_type, created_by_id, renderer_version, created_at
) VALUES (
    @id, @workspace_id, @article_id, @number, @kind, @title, @slug, @description, @body_markdown, @body_html, @toc,
    @visibility, @collection_id, @content_hash, @created_by_type, @created_by_id, @renderer_version, @now
)
RETURNING *;

-- name: PublishArticle :one
-- PublishArticle points the article at the version just frozen.
UPDATE articles SET
    status = 'published', published_version_id = @version_id, has_unpublished_changes = false,
    published_at = @now::timestamptz, first_published_at = coalesce(first_published_at, @now::timestamptz),
    archived_at = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetArticleStatus :one
-- SetArticleStatus unpublishes or archives an article, keeping its
-- versions.
UPDATE articles SET
    status = @status, published_version_id = CASE WHEN @status::text = 'published' THEN published_version_id END,
    has_unpublished_changes = true,
    archived_at = CASE WHEN @status::text = 'archived' THEN @now::timestamptz END, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteArticle :exec
-- DeleteArticle moves an article to the trash.
UPDATE articles SET deleted_at = @now::timestamptz, status = 'archived', published_version_id = NULL,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: ListArticleVersions :many
-- ListArticleVersions returns an article's versions, newest first.
SELECT id, number, kind, title, created_by_type, created_by_id, created_at FROM article_versions
WHERE workspace_id = @workspace_id AND article_id = @article_id
ORDER BY number DESC;

-- name: FindArticleVersion :one
-- FindArticleVersion returns one version of an article.
SELECT * FROM article_versions WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpsertRedirect :exec
-- UpsertRedirect points an old path at its new target.
INSERT INTO help_center_redirects (
    workspace_id, help_center_id, from_path, target_type, target_article_id, target_collection_id, source,
    created_at, updated_at
) VALUES (
    @workspace_id, @help_center_id, @from_path, @target_type, @target_article_id, @target_collection_id, @source,
    @now, @now
)
ON CONFLICT (help_center_id, from_path) DO UPDATE SET
    target_type = excluded.target_type, target_article_id = excluded.target_article_id,
    target_collection_id = excluded.target_collection_id, target_url = NULL, source = excluded.source,
    updated_at = excluded.updated_at;

-- name: FindRedirect :one
-- FindRedirect returns where an old path now leads.
SELECT r.*, a.slug AS article_slug, c.slug AS collection_slug
FROM help_center_redirects r
LEFT JOIN articles a ON a.id = r.target_article_id AND a.deleted_at IS NULL
LEFT JOIN collections c ON c.id = r.target_collection_id AND c.deleted_at IS NULL
WHERE r.workspace_id = @workspace_id AND r.help_center_id = @help_center_id AND r.from_path = @from_path;

-- name: ListPublishedArticles :many
-- ListPublishedArticles returns what readers see of a help center: every
-- published public article with its live version, in collection order.
SELECT a.id, a.collection_id, a.position, a.published_at, v.title, v.slug, v.description, v.body_markdown,
    v.body_html, v.toc, v.id AS version_id, v.created_at AS version_created_at
FROM articles a
JOIN article_versions v ON v.id = a.published_version_id
WHERE a.workspace_id = @workspace_id AND a.help_center_id = @help_center_id AND a.deleted_at IS NULL
  AND a.status = 'published' AND v.visibility = 'public'
ORDER BY a.collection_id NULLS LAST, a.position, a.id;

-- name: FindPublishedArticleBySlug :one
-- FindPublishedArticleBySlug returns a public article as readers see it.
SELECT a.id, a.collection_id, a.published_at, a.updated_at, v.title, v.slug, v.description, v.body_markdown,
    v.body_html, v.toc, v.id AS version_id
FROM articles a
JOIN article_versions v ON v.id = a.published_version_id
WHERE a.workspace_id = @workspace_id AND a.help_center_id = @help_center_id AND a.slug = @slug
  AND a.deleted_at IS NULL AND a.status = 'published' AND v.visibility = 'public';

-- name: SearchPublishedArticles :many
-- SearchPublishedArticles finds published articles matching a query,
-- best first, public ones only unless agent-only ones are asked for.
SELECT a.id, a.collection_id, v.title, v.slug, v.description, v.body_markdown, v.visibility,
    ts_rank_cd(v.search_document, query)::real AS rank,
    ts_headline('dimmi_search', v.body_markdown, query,
        'MaxWords=30, MinWords=12, ShortWord=2, MaxFragments=1, StartSel=<mark>, StopSel=</mark>')::text AS snippet
FROM articles a
JOIN article_versions v ON v.id = a.published_version_id,
    websearch_to_tsquery('dimmi_search', @query::text) AS query
WHERE a.workspace_id = @workspace_id AND a.help_center_id = @help_center_id AND a.deleted_at IS NULL
  AND a.status = 'published' AND v.search_document @@ query
  AND (v.visibility = 'public' OR @include_agent_only::boolean)
ORDER BY rank DESC, a.id
LIMIT @row_limit;

-- name: SearchWorkspaceArticles :many
-- SearchWorkspaceArticles finds the workspace's published articles, in
-- every live help center, matching any of the query's words, best first,
-- with the visibilities asked for. The agent retrieves from it.
SELECT a.id, a.help_center_id, h.subdomain, v.title, v.slug, v.body_markdown, v.visibility, v.created_at,
    ts_rank_cd(v.search_document, to_tsquery('dimmi_search', @query::text))::real AS rank
FROM articles a
JOIN article_versions v ON v.id = a.published_version_id
JOIN help_centers h ON h.id = a.help_center_id
WHERE a.workspace_id = @workspace_id AND a.deleted_at IS NULL AND a.status = 'published' AND h.deleted_at IS NULL
  AND v.search_document @@ to_tsquery('dimmi_search', @query::text)
  AND v.visibility = ANY (@visibilities::text[])
ORDER BY rank DESC, a.id
LIMIT @row_limit;

-- name: CountWorkspaceArticles :one
-- CountWorkspaceArticles counts the workspace's published articles.
SELECT count(*) FROM articles
WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND status = 'published';

-- name: FindWorkspaceArticleDocument :one
-- FindWorkspaceArticleDocument returns one published article as the
-- agent reads it, with the visibilities asked for.
SELECT a.id, a.help_center_id, h.subdomain, v.title, v.slug, v.body_markdown, v.visibility, v.created_at
FROM articles a
JOIN article_versions v ON v.id = a.published_version_id
JOIN help_centers h ON h.id = a.help_center_id
WHERE a.workspace_id = @workspace_id AND a.id = @id AND a.deleted_at IS NULL AND a.status = 'published'
  AND v.visibility = ANY (@visibilities::text[]);

-- name: ListWorkspaceHelpCenterSubdomains :many
-- ListWorkspaceHelpCenterSubdomains lists the subdomains of the
-- workspace's live help centers.
SELECT subdomain FROM help_centers WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND status = 'active';
