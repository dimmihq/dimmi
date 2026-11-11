-- name: EnrollSeriesContacts :many
-- EnrollSeriesContacts starts the series for each contact that may
-- enter: nobody with an active enrollment, and nobody who traveled it
-- before unless re-entry allows it after the cooldown. Test journeys
-- never count against entry.
INSERT INTO series_enrollments (workspace_id, series_id, version_id, subject, contact_id, node_id, seq, entered_at,
    entered_by, trigger_ref, test, created_at, updated_at)
SELECT @workspace_id, @series_id, @version_id, 'contact', c.id, 'entry', 1, @now::timestamptz, @entered_by,
    sqlc.narg(trigger_ref), @test, @now::timestamptz, @now::timestamptz
FROM unnest(@contact_ids::uuid[]) AS c(id)
WHERE @test OR NOT EXISTS (
    SELECT 1 FROM series_enrollments e
    WHERE e.series_id = @series_id AND e.contact_id = c.id AND NOT e.test
      AND (e.status IN ('active', 'waiting') OR NOT @reentry::boolean OR e.left_at > @cooldown_since::timestamptz)
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: EnrollSeriesCompanies :many
-- EnrollSeriesCompanies starts the series for each company that may
-- enter, under the same rules as contacts.
INSERT INTO series_enrollments (workspace_id, series_id, version_id, subject, company_id, node_id, seq, entered_at,
    entered_by, trigger_ref, test, created_at, updated_at)
SELECT @workspace_id, @series_id, @version_id, 'company', c.id, 'entry', 1, @now::timestamptz, @entered_by,
    sqlc.narg(trigger_ref), @test, @now::timestamptz, @now::timestamptz
FROM unnest(@company_ids::uuid[]) AS c(id)
WHERE @test OR NOT EXISTS (
    SELECT 1 FROM series_enrollments e
    WHERE e.series_id = @series_id AND e.company_id = c.id AND NOT e.test
      AND (e.status IN ('active', 'waiting') OR NOT @reentry::boolean OR e.left_at > @cooldown_since::timestamptz)
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: MarkSeriesBaseline :exec
-- MarkSeriesBaseline records the people who already matched when the
-- series went live, so only people who match later enter. The rows are
-- closed at once and left out of every count.
INSERT INTO series_enrollments (workspace_id, series_id, version_id, subject, contact_id, company_id, status,
    node_id, entered_at, entered_by, left_at, end_reason, created_at, updated_at)
SELECT @workspace_id::uuid, @series_id::uuid, @version_id::uuid, @subject::text,
    NULLIF(unnest(@contact_ids::uuid[]), '00000000-0000-0000-0000-000000000000'::uuid),
    NULLIF(unnest(@company_ids::uuid[]), '00000000-0000-0000-0000-000000000000'::uuid), 'exited', 'entry',
    @now::timestamptz, 'rules', @now::timestamptz, 'already_matched', @now::timestamptz, @now::timestamptz;

-- name: FindSeriesEnrollment :one
-- FindSeriesEnrollment returns one enrollment.
SELECT * FROM series_enrollments WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockSeriesEnrollment :one
-- LockSeriesEnrollment returns one enrollment, locked until the
-- transaction ends, so only one job moves it at a time.
SELECT * FROM series_enrollments WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: MoveSeriesEnrollment :one
-- MoveSeriesEnrollment stores where the enrollment stands now and moves
-- its sequence on.
UPDATE series_enrollments SET version_id = @version_id, node_id = @node_id, status = @status, wait = sqlc.narg(wait),
    wake_at = sqlc.narg(wake_at), seq = seq + 1, steps_executed = steps_executed + @steps::integer,
    variants = @variants, goal_at = COALESCE(goal_at, sqlc.narg(goal_at)), left_at = sqlc.narg(left_at),
    end_reason = sqlc.narg(end_reason), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetSeriesEnrollmentWait :exec
-- SetSeriesEnrollmentWait stores what a job outside the transaction
-- learned for the step the enrollment waits on.
UPDATE series_enrollments SET wait = @wait, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND seq = @seq AND status = 'waiting';

-- name: MarkSeriesGoal :execrows
-- MarkSeriesGoal records when an enrollment reached the series' goal.
UPDATE series_enrollments SET goal_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND goal_at IS NULL;

-- name: ListSubjectEnrollments :many
-- ListSubjectEnrollments returns the active enrollments of a contact,
-- and of its company, so a change can move them on.
SELECT * FROM series_enrollments
WHERE workspace_id = @workspace_id AND status IN ('active', 'waiting')
  AND (contact_id = @contact_id OR (sqlc.narg(company_id)::uuid IS NOT NULL AND company_id = sqlc.narg(company_id)::uuid));

-- name: ListGoalWindowEnrollments :many
-- ListGoalWindowEnrollments returns the finished or exited enrollments
-- of a series still inside its goal window that have not reached it.
SELECT * FROM series_enrollments
WHERE workspace_id = @workspace_id AND series_id = @series_id AND goal_at IS NULL AND NOT test
  AND status IN ('finished', 'exited') AND end_reason <> 'already_matched' AND entered_at > @since::timestamptz
ORDER BY id LIMIT 500;

-- name: ListDueSeriesEnrollments :many
-- ListDueSeriesEnrollments returns the enrollments whose wait ended a
-- while ago, or that stopped moving, so their lost job can be pushed
-- again.
SELECT id, seq, series_id FROM series_enrollments
WHERE workspace_id = @workspace_id
  AND ((status = 'waiting' AND wake_at < @before::timestamptz) OR (status = 'active' AND updated_at < @before::timestamptz))
ORDER BY id LIMIT 1000;

-- name: ListSeriesMovable :many
-- ListSeriesMovable returns the active enrollments of a series that a
-- resumed series must move on.
SELECT id, seq FROM series_enrollments
WHERE workspace_id = @workspace_id AND series_id = @series_id
  AND (status = 'active' OR (status = 'waiting' AND (wake_at IS NULL OR wake_at <= @now::timestamptz)))
ORDER BY id;

-- name: ListSeriesTravelers :many
-- ListSeriesTravelers returns the active enrollments of a series, with
-- where each stands, for moving them to a new version.
SELECT * FROM series_enrollments
WHERE workspace_id = @workspace_id AND series_id = @series_id AND status IN ('active', 'waiting')
ORDER BY id;

-- name: CountSeriesTravelersByNode :many
-- CountSeriesTravelersByNode counts the active enrollments of a series
-- on each step.
SELECT node_id, count(*) AS n FROM series_enrollments
WHERE workspace_id = @workspace_id AND series_id = @series_id AND status IN ('active', 'waiting') AND NOT test
GROUP BY node_id;

-- name: EndSeriesEnrollments :many
-- EndSeriesEnrollments ends every active enrollment of a series.
UPDATE series_enrollments SET status = 'exited', end_reason = @end_reason, left_at = @now::timestamptz,
    wait = NULL, wake_at = NULL, seq = seq + 1, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND series_id = @series_id AND status IN ('active', 'waiting')
RETURNING id;

-- name: ListSeriesEnrollmentPage :many
-- ListSeriesEnrollmentPage returns a page of a series' enrollments with
-- who travels, newest first, narrowed by step and status.
SELECT e.*, coalesce(ct.name, '')::text AS contact_name, coalesce(ct.email::text, '')::text AS contact_email,
    coalesce(cm.name, '')::text AS company_name
FROM series_enrollments e
LEFT JOIN contacts ct ON ct.id = e.contact_id
LEFT JOIN companies cm ON cm.id = COALESCE(e.company_id, ct.company_id)
WHERE e.workspace_id = @workspace_id AND e.series_id = @series_id
  AND (e.end_reason IS NULL OR e.end_reason <> 'already_matched')
  AND (sqlc.narg(node_id)::text IS NULL OR (e.node_id = sqlc.narg(node_id)::text AND e.status IN ('active', 'waiting')))
  AND (sqlc.narg(status)::text IS NULL OR e.status = sqlc.narg(status)::text)
ORDER BY e.entered_at DESC, e.id DESC
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: ListContactSeriesEnrollments :many
-- ListContactSeriesEnrollments returns the journeys of a contact and of
-- their company, newest first.
SELECT e.*, s.name AS series_name FROM series_enrollments e
JOIN series s ON s.id = e.series_id
WHERE e.workspace_id = @workspace_id AND (e.end_reason IS NULL OR e.end_reason <> 'already_matched')
  AND (e.contact_id = sqlc.narg(contact_id)::uuid OR e.company_id = sqlc.narg(company_id)::uuid)
ORDER BY e.entered_at DESC LIMIT 100;

-- name: SeriesTotals :one
-- SeriesTotals counts what happened to a series' travelers.
SELECT count(*) AS entered,
    count(*) FILTER (WHERE status IN ('active', 'waiting')) AS active,
    count(*) FILTER (WHERE status = 'finished') AS finished,
    count(*) FILTER (WHERE status = 'exited') AS exited,
    count(*) FILTER (WHERE goal_at IS NOT NULL) AS goal
FROM series_enrollments
WHERE workspace_id = @workspace_id AND series_id = @series_id AND NOT test
  AND (end_reason IS NULL OR end_reason <> 'already_matched');

-- name: SeriesEndReasons :many
-- SeriesEndReasons counts why travelers left a series.
SELECT coalesce(end_reason, '')::text AS reason, count(*) AS n FROM series_enrollments
WHERE workspace_id = @workspace_id AND series_id = @series_id AND NOT test AND status IN ('finished', 'exited')
  AND end_reason <> 'already_matched'
GROUP BY end_reason ORDER BY count(*) DESC;

-- name: SeriesEntriesByDay :many
-- SeriesEntriesByDay counts entries and goals per day since the time.
SELECT date_trunc('day', entered_at)::timestamptz AS day, count(*) AS entered,
    count(*) FILTER (WHERE goal_at IS NOT NULL) AS goal
FROM series_enrollments
WHERE workspace_id = @workspace_id AND series_id = @series_id AND NOT test AND entered_at >= @since::timestamptz
  AND (end_reason IS NULL OR end_reason <> 'already_matched')
GROUP BY 1 ORDER BY 1;

-- name: SeriesVariantGoals :many
-- SeriesVariantGoals counts, for each path of a split step, how many
-- travelers took it and how many reached the goal.
SELECT coalesce(variants ->> @node_id::text, '')::text AS path, count(*) AS entered,
    count(*) FILTER (WHERE goal_at IS NOT NULL) AS goal
FROM series_enrollments
WHERE workspace_id = @workspace_id AND series_id = @series_id AND NOT test AND variants ? @node_id::text
GROUP BY 1;

-- name: EndTestJourneys :many
-- EndTestJourneys ends the test journeys a contact, or their company,
-- still has on a series, so trying it again starts afresh.
UPDATE series_enrollments SET status = 'exited', end_reason = 'replaced', left_at = @now::timestamptz,
    wait = NULL, wake_at = NULL, seq = seq + 1, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND series_id = @series_id AND test AND status IN ('active', 'waiting')
  AND (contact_id = @contact_id OR (sqlc.narg(company_id)::uuid IS NOT NULL AND company_id = sqlc.narg(company_id)::uuid))
RETURNING id;
