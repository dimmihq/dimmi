/*
|--------------------------------------------------------------------------
| Publishing A Series
|--------------------------------------------------------------------------
|
| Publishing freezes the working copy into a new version and makes it
| the one that runs. People already on the journey follow the new
| version from the step they stand on, so a fixed typo reaches everyone
| who has not got the message yet. Someone standing on a step the new
| version removed is moved where the publisher chose, by default to the
| first later step that still exists, or leaves. Every message step
| keeps its own broadcast, updated from the new version, so its report
| runs on across versions.
|
*/

package series

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MoveLeave is the move that ends the journey of people on a removed
step.
*/
const MoveLeave = "leave"

/*
PublishInput publishes one series with a note; Moves says, for each
removed step people stand on, the step to move them to or MoveLeave.
*/
type PublishInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Note       string
	Moves      map[string]string
}

/*
Publish makes the working copy the live version.
*/
func (s *Series) Publish(ctx context.Context, in PublishInput) (models.Series, error) {
	m := in.Membership
	if !m.Can("series:publish") {
		return models.Series{}, exceptions.ErrForbidden
	}

	var out models.Series

	err := s.inTx(ctx, m.Workspace.ID, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := q.LockSeries(ctx, models.LockSeriesParams{WorkspaceID: m.Workspace.ID, ID: in.ID})
		if err != nil {
			return notFound(err, "lock series")
		}

		if row.Status == StatusArchived {
			return ErrArchived
		}

		spec, err := SpecOf(row)
		if err != nil {
			return err
		}

		if problems := Check(spec, row.Subject); len(problems) > 0 {
			return ErrProblems.With("problems", problems)
		}

		if out, err = s.release(ctx, tx, release{row: row, spec: spec, in: in}); err != nil {
			return err
		}

		return s.published(ctx, tx, out, m.Member.ID)
	})

	return out, err
}

/*
release is one publication under way.
*/
type release struct {
	row  models.Series
	spec Spec
	in   PublishInput
}

/*
release stores the version, readies its message steps, moves the
travelers and sets the series live.
*/
func (s *Series) release(ctx context.Context, tx *database.Tx, r release) (models.Series, error) {
	q := models.New(tx)
	ws, now := r.row.WorkspaceID, s.Clock.Now()

	raw, hash, err := encode(r.spec)
	if err != nil {
		return r.row, err
	}

	moves, _ := json.Marshal(r.in.Moves)
	version, err := q.CreateSeriesVersion(ctx, models.CreateSeriesVersionParams{
		WorkspaceID: ws, SeriesID: r.row.ID, Spec: raw, SpecHash: hash, Note: cut(r.in.Note, 280), Moves: moves,
		MemberID: &r.in.Membership.Member.ID, Now: now,
	})
	if err != nil {
		return r.row, wrap("create series version", err)
	}

	status := StatusLive
	if r.row.Status == StatusPaused {
		status = StatusPaused
	}

	if err := s.readyMessages(ctx, q, r); err != nil {
		return r.row, err
	}

	if r.row.LiveVersionID != nil {
		if err := s.follow(ctx, tx, r, version); err != nil {
			return r.row, err
		}
	}

	out, err := q.SetSeriesStatus(ctx, models.SetSeriesStatusParams{
		WorkspaceID: ws, ID: r.row.ID, Status: status, LiveVersionID: &version.ID, PublishedAt: &now,
		PausedAt: r.row.PausedAt, Now: now,
	})
	if err != nil {
		return r.row, wrap("set series live", err)
	}

	if r.row.LiveVersionID != nil || !hasEntryRules(r.spec) {
		return out, nil
	}

	return out, s.Queue.PushTx(ctx, tx, jobs.EnrollSeriesArgs{
		WorkspaceID: ws, SeriesID: r.row.ID, Baseline: !r.spec.Entry.IncludeExisting,
	})
}

/*
readyMessages keeps one broadcast per message step in step with the
version and checks each can send.
*/
func (s *Series) readyMessages(ctx context.Context, q *models.Queries, r release) error {
	row, sending := r.row, "sending"
	if row.Status == StatusPaused {
		sending = StatusPaused
	}

	for _, n := range compile(r.spec).messageNodes() {
		b, err := q.UpsertSeriesBroadcast(ctx, broadcastParams(row, n, sending, s.Clock.Now()))
		if err != nil {
			return wrap("keep series broadcast", err)
		}

		if err := s.Broadcasts.CheckSeriesMessage(ctx, q, b); err != nil {
			return ErrProblems.With("problems", []Problem{{
				Code: "message_not_ready", NodeID: n.ID, Message: err.Error(),
			}})
		}
	}

	return nil
}

/*
broadcastParams lays out the broadcast of one message step.
*/
func broadcastParams(row models.Series, n Node, status string, now time.Time) models.UpsertSeriesBroadcastParams {
	m := settingsOf[MessageSettings](n)
	track := m.TrackClicks == nil || *m.TrackClicks

	return models.UpsertSeriesBroadcastParams{
		WorkspaceID: row.WorkspaceID, Name: cut(row.Name+": "+n.ID, 120), Channel: n.Kind, Status: status,
		SeriesID: &row.ID, SeriesNodeID: &n.ID, TopicID: row.TopicID, Service: row.Service, Subject: m.Subject,
		Preview: m.Preview, Body: m.Body, SenderMemberID: row.SenderMemberID, EmailAddressID: row.EmailAddressID,
		SlackAs: row.SlackAs, ReplyTeamID: row.ReplyTeamID, ReplyMemberID: row.ReplyMemberID,
		AiReplies: row.AiReplies, TrackClicks: track, Now: now,
	}
}

/*
follow moves the travelers of the old version onto the new one.
*/
func (s *Series) follow(ctx context.Context, tx *database.Tx, r release, version models.SeriesVersion) error {
	q := models.New(tx)
	row, moves := r.row, r.in.Moves

	old, err := s.version(ctx, q, row.WorkspaceID, *row.LiveVersionID)
	if err != nil {
		return err
	}

	next, err := decode(version.Spec)
	if err != nil {
		return err
	}

	travelers, err := q.ListSeriesTravelers(ctx, models.ListSeriesTravelersParams{
		WorkspaceID: row.WorkspaceID, SeriesID: row.ID,
	})
	if err != nil {
		return wrap("list travelers", err)
	}

	g := compile(next)
	for _, e := range travelers {
		e.VersionID = version.ID
		if err := s.followOne(ctx, tx, e, landingAt{g: g, node: landing(old, g, e.NodeID, moves)}); err != nil {
			return err
		}
	}

	return nil
}

/*
landing returns the step of the new version a traveler on the step
continues from, or "" when they leave.
*/
func landing(old, next graph, node string, moves map[string]string) string {
	if _, ok := next.node(node); ok {
		return node
	}

	if to, ok := moves[node]; ok {
		if _, exists := next.node(to); exists && to != MoveLeave {
			return to
		}

		return ""
	}

	to, _ := old.successor(node, next)

	return to
}

/*
landingAt is the step of the new version a traveler continues from,
"" when they leave.
*/
type landingAt struct {
	g    graph
	node string
}

/*
followOne moves one traveler: on the same step it keeps its wait,
anywhere else it starts that step afresh, and with nowhere to go it
leaves.
*/
func (s *Series) followOne(ctx context.Context, tx *database.Tx, e models.SeriesEnrollment, to landingAt) error {
	q := models.New(tx)
	p := moveParams(e, s.Clock.Now())
	g, node := to.g, to.node

	switch node {
	case e.NodeID:
		if err := s.rewait(ctx, q, g, &p); err != nil {
			return err
		}
	case "":
		p.Status, p.Wait, p.WakeAt, p.LeftAt, p.EndReason = "exited", nil, nil, &p.Now, new("node_removed")
	default:
		p.NodeID, p.Status, p.Wait, p.WakeAt = node, "active", nil, nil
	}

	moved, err := q.MoveSeriesEnrollment(ctx, p)
	if err != nil {
		return wrap("move traveler", err)
	}

	if moved.Status == "exited" {
		return s.withdraw(ctx, q, moved)
	}

	return s.Queue.PushTx(ctx, tx, jobs.AdvanceSeriesEnrollmentArgs{
		WorkspaceID: e.WorkspaceID, EnrollmentID: e.ID, Seq: moved.Seq,
	})
}

/*
rewait works a changed wait out again from when the traveler arrived
at it, so a shorter wait lets them on at once and a longer one holds
them longer.
*/
func (s *Series) rewait(ctx context.Context, q *models.Queries, g graph, p *models.MoveSeriesEnrollmentParams) error {
	var w wait
	if p.Status != "waiting" {
		return nil
	}

	if err := json.Unmarshal(p.Wait, &w); err != nil || w.Kind != "timer" {
		return wrap("read wait", err)
	}

	n, ok := g.node(p.NodeID)
	settings := settingsOf[WaitSettings](n)

	if !ok || n.Kind != KindWait || settings.Mode != "duration" {
		return nil
	}

	id, err := parseID(w.StepID)
	if err != nil {
		return wrap("read waiting step", err)
	}

	step, err := q.FindSeriesStep(ctx, models.FindSeriesStepParams{WorkspaceID: p.WorkspaceID, ID: id})
	if err != nil {
		return wrap("find waiting step", err)
	}

	d, _ := durationOf(settings.Amount, settings.Unit)
	w.Until = step.EnteredAt.Add(d)
	p.Wait, _ = json.Marshal(w)
	p.WakeAt = &w.Until

	return nil
}

/*
hasEntryRules reports whether people enter the series by rules alone.
*/
func hasEntryRules(spec Spec) bool {
	return spec.Entry.Event == "" && (spec.Entry.AudienceID != "" || len(spec.Entry.Rules) > 0)
}

/*
cut returns at most limit runes of the text, trimmed.
*/
func cut(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}

	return string(runes[:limit])
}
