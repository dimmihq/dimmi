/*
|--------------------------------------------------------------------------
| Backfilling the Log
|--------------------------------------------------------------------------
|
| Conversations that started before the log existed get their changes
| from what their timeline recorded: closes, reopens, snoozes, wake-ups,
| merges, the AI taking back over and each assignment, plus a start, a
| close or a hand-off the timeline missed. Only what happened before
| the log started is written, a thousand conversations per transaction,
| and the rows are tagged as backfilled. Once the log is complete, the
| workspace's facts are rebuilt.
|
*/

package reports

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
backfill fills the workspace's log and asks for a rebuild.
*/
func (m *Maintenance) backfill(ctx context.Context, ws ids.UUID) error {
	var (
		state  models.ReportState
		bounds []ids.UUID
	)

	err := m.read(ctx, ws, func(q *models.Queries) error {
		var err error
		if state, err = q.FindReportState(ctx, ws); err != nil {
			return err
		}

		bounds, err = q.ReportBackfillBounds(ctx, ws)

		return err
	})
	if err != nil {
		return fmt.Errorf("plan backfill: %w", err)
	}

	after := ids.Nil()
	for _, until := range append(bounds, lastID()) {
		chunk := chunkRange{after: after, until: until, start: state.LogStartedAt}
		if err := m.backfillChunk(ctx, ws, chunk); err != nil {
			return err
		}

		after = until
	}

	now := m.Builder.Clock.Now()

	return m.read(ctx, ws, func(q *models.Queries) error {
		finish := models.FinishReportBackfillParams{Now: now, WorkspaceID: ws}
		if err := q.FinishReportBackfill(ctx, finish); err != nil {
			return fmt.Errorf("finish backfill: %w", err)
		}

		return q.RequestReportRebuild(ctx, models.RequestReportRebuildParams{WorkspaceID: ws, Now: now})
	})
}

/*
chunkRange is the ids after one and up to another, and when the log
started.
*/
type chunkRange struct {
	after, until ids.UUID
	start        time.Time
}

/*
backfillChunk logs one chunk's history.
*/
func (m *Maintenance) backfillChunk(ctx context.Context, ws ids.UUID, r chunkRange) error {
	return m.read(ctx, ws, func(q *models.Queries) error {
		n, err := q.BackfillConversationEvents(ctx, models.BackfillConversationEventsParams{
			WorkspaceID: ws, After: r.after, Until: r.until, LogStartedAt: r.start,
		})
		if err != nil {
			return fmt.Errorf("backfill events: %w", err)
		}

		if n == 0 {
			return nil
		}

		assigned, err := q.ReportTimelineAssignments(ctx, models.ReportTimelineAssignmentsParams{
			WorkspaceID: ws, After: r.after, Until: r.until, LogStartedAt: r.start,
		})
		if err != nil {
			return fmt.Errorf("read timeline assignments: %w", err)
		}

		return m.logAssignments(ctx, q, ws, assigned)
	})
}

/*
backfillRow is one backfilled change, named like the log's columns.
*/
type backfillRow struct {
	WorkspaceID    ids.UUID  `json:"workspace_id"`
	ConversationID ids.UUID  `json:"conversation_id"`
	Kind           string    `json:"kind"`
	OccurredAt     time.Time `json:"occurred_at"`
	MemberID       *ids.UUID `json:"member_id"`
	TeamID         *ids.UUID `json:"team_id"`
}

/*
logAssignments logs the assignments the timeline recorded: the member
and the team taking over, nobody when it names none.
*/
func (m *Maintenance) logAssignments(
	ctx context.Context, q *models.Queries, ws ids.UUID, rows []models.ReportTimelineAssignmentsRow,
) error {
	if len(rows) == 0 {
		return nil
	}

	out := make([]backfillRow, 0, 2*len(rows))
	for _, r := range rows {
		base := backfillRow{WorkspaceID: ws, ConversationID: r.ConversationID, OccurredAt: r.CreatedAt}

		member, team := base, base
		member.Kind, member.MemberID = kindAssigned, m.decode(string(models.PrefixMember), r.Member)
		team.Kind, team.TeamID = kindTeamAssigned, m.decode(string(models.PrefixTeam), r.Team)
		out = append(out, member, team)
	}

	data, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("encode backfilled assignments: %w", err)
	}

	if err := q.InsertBackfillEvents(ctx, data); err != nil {
		return fmt.Errorf("log backfilled assignments: %w", err)
	}

	return nil
}

/*
decode reads a public id, nil for none or one that cannot be read.
*/
func (m *Maintenance) decode(prefix, public string) *ids.UUID {
	if public == "" || m.IDs == nil {
		return nil
	}

	id, err := m.IDs.Decode(prefix, public)
	if err != nil {
		return nil
	}

	return &id
}

/*
lastID is the greatest id, the end of the last chunk.
*/
func lastID() ids.UUID {
	var id ids.UUID
	for i := range id {
		id[i] = 0xff
	}

	return id
}
