-- name: CountChangelogEntryView :exec
-- CountChangelogEntryView adds one view of an update on the day.
INSERT INTO changelog_entry_views (workspace_id, help_center_id, entry_id, day, views)
VALUES (@workspace_id, @help_center_id, @entry_id, @counted_at::timestamptz, 1)
ON CONFLICT (entry_id, day) DO UPDATE SET views = changelog_entry_views.views + 1;

-- name: DailyChangelogViews :many
-- DailyChangelogViews totals a changelog's views per day from a day on.
SELECT to_char(day, 'YYYY-MM-DD')::text AS day, sum(views)::bigint AS views
FROM changelog_entry_views
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND day >= @since::timestamptz
GROUP BY changelog_entry_views.day ORDER BY changelog_entry_views.day;

-- name: TopViewedChangelogEntries :many
-- TopViewedChangelogEntries returns a changelog's most read updates from
-- a day on, with the reactions readers left over the same days.
SELECT v.entry_id, e.title, e.slug, e.published_at, sum(v.views)::bigint AS views,
    (SELECT count(*) FROM changelog_reactions r
        WHERE r.workspace_id = v.workspace_id AND r.entry_id = v.entry_id
          AND r.created_at >= @since::timestamptz)::bigint AS reactions
FROM changelog_entry_views v
JOIN changelog_entries e ON e.id = v.entry_id AND e.deleted_at IS NULL
WHERE v.workspace_id = @workspace_id AND v.help_center_id = @help_center_id AND v.day >= @since::timestamptz
GROUP BY v.entry_id, v.workspace_id, e.title, e.slug, e.published_at
ORDER BY views DESC, e.title
LIMIT @row_limit;

-- name: ChangelogReaderTotals :one
-- ChangelogReaderTotals counts a changelog's reactions from a day on, its
-- confirmed subscribers now, and those who confirmed or left from that
-- day on.
SELECT
    (SELECT count(*) FROM changelog_reactions r JOIN changelog_entries e ON e.id = r.entry_id
        WHERE r.workspace_id = @workspace_id AND e.help_center_id = @help_center_id
          AND r.created_at >= @since::timestamptz)::bigint AS reactions,
    (SELECT count(*) FROM changelog_subscribers s
        WHERE s.workspace_id = @workspace_id AND s.help_center_id = @help_center_id
          AND s.status = 'active')::bigint AS subscribers,
    (SELECT count(*) FROM changelog_subscribers s
        WHERE s.workspace_id = @workspace_id AND s.help_center_id = @help_center_id
          AND s.status = 'active' AND s.confirmed_at >= @since::timestamptz)::bigint AS joined,
    (SELECT count(*) FROM changelog_subscribers s
        WHERE s.workspace_id = @workspace_id AND s.help_center_id = @help_center_id
          AND s.status = 'unsubscribed' AND s.unsubscribed_at >= @since::timestamptz)::bigint AS left_count;
