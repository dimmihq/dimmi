-- name: RateArticle :one
-- RateArticle keeps one reader's answer about an article, changing it
-- when they answer again. A yes clears any comment left with a no.
INSERT INTO article_ratings (workspace_id, article_id, rater_hash, contact_id, helpful, created_at, updated_at)
VALUES (@workspace_id, @article_id, @rater_hash, sqlc.narg('contact_id'), @helpful, @now, @now)
ON CONFLICT (article_id, rater_hash) DO UPDATE SET
    helpful = EXCLUDED.helpful,
    contact_id = COALESCE(EXCLUDED.contact_id, article_ratings.contact_id),
    comment = CASE WHEN EXCLUDED.helpful THEN NULL ELSE article_ratings.comment END,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: CommentOnArticleRating :execrows
-- CommentOnArticleRating adds a reader's comment to their own no.
UPDATE article_ratings SET comment = @comment, updated_at = @now
WHERE workspace_id = @workspace_id AND article_id = @article_id AND rater_hash = @rater_hash AND NOT helpful;

-- name: CountArticleRatings :one
-- CountArticleRatings counts an article's yes and no answers.
SELECT
    count(*) FILTER (WHERE helpful)::int AS helpful,
    count(*) FILTER (WHERE NOT helpful)::int AS unhelpful,
    count(*) FILTER (WHERE comment IS NOT NULL)::int AS comments
FROM article_ratings
WHERE workspace_id = @workspace_id AND article_id = @article_id;

-- name: ListArticleRatingComments :many
-- ListArticleRatingComments returns the comments readers left with a no,
-- newest first, with who left them when they were signed in.
SELECT r.id, r.comment::text AS comment, r.updated_at, r.contact_id, c.name AS contact_name, c.email AS contact_email
FROM article_ratings r
LEFT JOIN contacts c ON c.workspace_id = r.workspace_id AND c.id = r.contact_id
WHERE r.workspace_id = @workspace_id AND r.article_id = @article_id AND r.comment IS NOT NULL
ORDER BY r.updated_at DESC, r.id DESC
LIMIT @lim;
