-- name: CountArticleView :exec
-- CountArticleView adds one view of an article on the day from the source.
INSERT INTO article_views (workspace_id, help_center_id, article_id, day, source, views)
VALUES (@workspace_id, @help_center_id, @article_id, @counted_at::timestamptz, @source, 1)
ON CONFLICT (article_id, day, source) DO UPDATE SET views = article_views.views + 1;

-- name: CountHelpSearch :exec
-- CountHelpSearch adds one search for the words on the day, and one miss
-- when it found nothing.
INSERT INTO help_search_terms (workspace_id, help_center_id, day, term, searches, misses)
VALUES (@workspace_id, @help_center_id, @counted_at::timestamptz, @term, 1, @misses)
ON CONFLICT (help_center_id, day, term) DO UPDATE SET
    searches = help_search_terms.searches + 1, misses = help_search_terms.misses + excluded.misses;

-- name: DailyArticleViews :many
-- DailyArticleViews totals a help center's views per day from a day on.
SELECT to_char(day, 'YYYY-MM-DD')::text AS day, sum(views)::bigint AS views,
    coalesce(sum(views) FILTER (WHERE source = 'messenger'), 0)::bigint AS messenger_views
FROM article_views
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND day >= @since::timestamptz
GROUP BY article_views.day ORDER BY article_views.day;

-- name: TopViewedArticles :many
-- TopViewedArticles returns a help center's most read articles from a
-- day on, with what readers answered about them over the same days.
SELECT v.article_id, a.title, a.slug, sum(v.views)::bigint AS views,
    (SELECT count(*) FROM article_ratings r
        WHERE r.workspace_id = v.workspace_id AND r.article_id = v.article_id AND r.helpful
          AND r.updated_at >= @since::timestamptz)::bigint AS helpful,
    (SELECT count(*) FROM article_ratings r
        WHERE r.workspace_id = v.workspace_id AND r.article_id = v.article_id AND NOT r.helpful
          AND r.updated_at >= @since::timestamptz)::bigint AS unhelpful
FROM article_views v
JOIN articles a ON a.id = v.article_id AND a.deleted_at IS NULL
WHERE v.workspace_id = @workspace_id AND v.help_center_id = @help_center_id AND v.day >= @since::timestamptz
GROUP BY v.article_id, v.workspace_id, a.title, a.slug
ORDER BY views DESC, a.title
LIMIT @row_limit;

-- name: HelpCenterRatingTotals :one
-- HelpCenterRatingTotals counts the yes and no answers about a help
-- center's articles from a day on.
SELECT count(*) FILTER (WHERE r.helpful)::bigint AS helpful, count(*) FILTER (WHERE NOT r.helpful)::bigint AS unhelpful
FROM article_ratings r
JOIN articles a ON a.id = r.article_id
WHERE r.workspace_id = @workspace_id AND a.help_center_id = @help_center_id AND r.updated_at >= @since::timestamptz;

-- name: HelpSearchTotals :one
-- HelpSearchTotals counts a help center's searches and those that found
-- nothing from a day on.
SELECT coalesce(sum(searches), 0)::bigint AS searches, coalesce(sum(misses), 0)::bigint AS misses
FROM help_search_terms
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND day >= @since::timestamptz;

-- name: TopMissedSearches :many
-- TopMissedSearches returns the words readers searched for most that
-- found nothing from a day on.
SELECT term, sum(misses)::bigint AS misses, to_char(max(day), 'YYYY-MM-DD')::text AS last_day
FROM help_search_terms
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND day >= @since::timestamptz AND misses > 0
GROUP BY term
ORDER BY misses DESC, term
LIMIT @row_limit;

-- name: PruneHelpAnalytics :exec
-- PruneHelpAnalytics drops the days of every workspace older than kept.
SELECT help_analytics_prune(@views_before::timestamptz, @terms_before::timestamptz);
