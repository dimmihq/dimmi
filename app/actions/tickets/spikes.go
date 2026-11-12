/*
|--------------------------------------------------------------------------
| Spike Detection
|--------------------------------------------------------------------------
|
| Every minute the detector reads the last fifteen minutes of new
| conversations of each workspace that had any, and asks the model to
| group them by what they are about. A group at or above the
| workspace's threshold becomes a suggestion to declare an incident, or
| grows the one already open under its key, and everyone online hears
| of it. A group the team dismissed stays quiet for an hour; a group
| about an incident already declared joins that incident instead.
|
*/

package tickets

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Spike windows: how far back the detector reads, and how long a
dismissed group stays quiet.
*/
const (
	SpikeWindow = 15 * time.Minute
	SpikeMute   = time.Hour
)

/*
The statuses of a suggestion.
*/
const (
	SuggestionOpen      = "open"
	SuggestionAccepted  = "accepted"
	SuggestionDismissed = "dismissed"
)

/*
Spikes is the sweep that finds spikes of conversations.
*/
type Spikes struct {
	Tickets *Tickets
}

/*
Sweep looks for spikes in every workspace with new conversations.
*/
func (s *Spikes) Sweep(ctx context.Context) error {
	t := s.Tickets
	if t.Filler == nil {
		return nil
	}

	var workspaces []ids.UUID

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		workspaces, err = models.New(tx).ListWorkspacesWithRecentConversations(ctx, t.Clock.Now().Add(-SpikeWindow))

		return err
	})
	if err != nil {
		return fmt.Errorf("list workspaces with new conversations: %w", err)
	}

	var errs []error
	for _, ws := range workspaces {
		errs = append(errs, s.detect(tenancy.With(ctx, ws.String()), ws))
	}

	return errors.Join(errs...)
}

/*
window is what the detector reads of one workspace.
*/
type window struct {
	rows      []models.ListRecentWidgetConversationsRow
	threshold int
	incidents []models.Ticket
}

/*
detect groups one workspace's new conversations and acts on each group
large enough.
*/
func (s *Spikes) detect(ctx context.Context, ws ids.UUID) error {
	win, err := s.read(ctx, ws)
	if err != nil || len(win.rows) < win.threshold {
		return err
	}

	q := GroupQuestion{WorkspaceID: ws}
	for _, r := range win.rows {
		q.Items = append(q.Items, GroupItem{ID: r.ID, Text: itemText(r)})
	}

	groups, err := s.Tickets.Filler.Group(ctx, q)
	if err != nil {
		return fmt.Errorf("group new conversations: %w", err)
	}

	known := make([]ids.UUID, len(win.rows))
	for i, r := range win.rows {
		known[i] = r.ID
	}

	var errs []error

	for _, g := range groups {
		g.IDs = keepKnown(g.IDs, known)
		if len(g.IDs) < win.threshold || strings.TrimSpace(g.Title) == "" {
			continue
		}

		errs = append(errs, s.act(ctx, ws, g, win.incidents))
	}

	return errors.Join(errs...)
}

/*
read loads the window, the threshold and the active incidents.
*/
func (s *Spikes) read(ctx context.Context, ws ids.UUID) (window, error) {
	var out window

	err := s.Tickets.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		settings, err := q.FindWorkspaceTicketSettings(ctx, ws)
		if err != nil {
			return fmt.Errorf("read spike threshold: %w", err)
		}

		out.threshold = int(settings.IncidentSpikeThreshold)

		out.rows, err = q.ListRecentWidgetConversations(ctx, models.ListRecentWidgetConversationsParams{
			WorkspaceID: ws, Since: s.Tickets.Clock.Now().Add(-SpikeWindow),
		})
		if err != nil {
			return fmt.Errorf("list new conversations: %w", err)
		}

		out.incidents, err = q.ListActiveIncidents(ctx, ws)
		if err != nil {
			return fmt.Errorf("list active incidents: %w", err)
		}

		return nil
	})

	return out, err
}

/*
itemText is what the model reads of one new conversation.
*/
func itemText(r models.ListRecentWidgetConversationsRow) string {
	text := r.FirstMessage
	if r.Subject != nil && *r.Subject != "" {
		text = *r.Subject + "\n" + text
	}

	return clip(text, 600)
}

/*
keepKnown keeps the ids the window holds, once each.
*/
func keepKnown(list, known []ids.UUID) []ids.UUID {
	var out []ids.UUID

	for _, id := range list {
		if slices.Contains(known, id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}

	return out
}

/*
act links a group about an active incident to it, or suggests it.
*/
func (s *Spikes) act(ctx context.Context, ws ids.UUID, g Group, incidents []models.Ticket) error {
	incident, err := s.matchIncident(ctx, ws, g, incidents)
	if err != nil {
		return err
	}

	if incident != nil {
		return s.Tickets.linkAll(ctx, ws, *incident, g.IDs)
	}

	return s.Tickets.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := s.Tickets.systemWriter(ctx, tx, ws)
		if err != nil {
			return err
		}

		return s.suggest(ctx, w, ws, g)
	})
}

/*
matchIncident asks which active incident the group is about, if any.
*/
func (s *Spikes) matchIncident(
	ctx context.Context, ws ids.UUID, g Group, incidents []models.Ticket,
) (*ids.UUID, error) {
	if len(incidents) == 0 {
		return nil, nil
	}

	text := g.Title
	if g.Area != "" {
		text += "\nArea: " + g.Area
	}

	return s.Tickets.bestMatch(ctx, MatchQuestion{WorkspaceID: ws, Text: text, Candidates: candidates(incidents)})
}

/*
suggest stores the group as a suggestion, grows the open one, links it
to the incident an accepted one declared, or stays quiet while the
group is muted.
*/
func (s *Spikes) suggest(ctx context.Context, w *Writer, ws ids.UUID, g Group) error {
	key := signature(g)

	last, err := w.Q.FindLatestSuggestionBySignature(ctx, models.FindLatestSuggestionBySignatureParams{
		WorkspaceID: ws, Signature: key,
	})
	if err != nil && !isNoRows(err) {
		return fmt.Errorf("find suggestion: %w", err)
	}

	if err == nil {
		switch {
		case last.Status == SuggestionOpen:
			return s.grow(ctx, w, last, g)
		case last.Status == SuggestionAccepted && last.TicketID != nil:
			return w.linkConversations(ctx, ws, *last.TicketID, g.IDs)
		case last.MutedUntil != nil && w.Now.Before(*last.MutedUntil):
			return nil
		}
	}

	made, err := w.Q.CreateIncidentSuggestion(ctx, models.CreateIncidentSuggestionParams{
		WorkspaceID: ws, Signature: key, Title: clip(g.Title, 200), Summary: summaryOf(g),
		Area: optionalText(clip(g.Area, 80)), ConversationIds: g.IDs, ConversationCount: dbCount(len(g.IDs)),
		WindowMinutes: int32(SpikeWindow / time.Minute), Now: w.Now,
	})
	if err != nil {
		return fmt.Errorf("store suggestion: %w", err)
	}

	return w.emit(ctx, appevents.IncidentSuggested{Base: w.Base(ws), SuggestionID: made.ID})
}

/*
grow adds the group's new conversations to the open suggestion and
announces it again when it grew.
*/
func (s *Spikes) grow(ctx context.Context, w *Writer, cur models.IncidentSuggestion, g Group) error {
	all := slices.Clone(cur.ConversationIds)
	for _, id := range g.IDs {
		if !slices.Contains(all, id) {
			all = append(all, id)
		}
	}

	if len(all) == len(cur.ConversationIds) {
		return nil
	}

	_, err := w.Q.GrowIncidentSuggestion(ctx, models.GrowIncidentSuggestionParams{
		ConversationIds: all, ConversationCount: dbCount(len(all)), Title: clip(g.Title, 200),
		Summary: summaryOf(g), Now: w.Now, WorkspaceID: cur.WorkspaceID, ID: cur.ID,
	})
	if err != nil {
		return fmt.Errorf("grow suggestion: %w", err)
	}

	return w.emit(ctx, appevents.IncidentSuggested{Base: w.Base(cur.WorkspaceID), SuggestionID: cur.ID})
}

/*
signature is the group's stable key, or its title when it gave none.
*/
func signature(g Group) string {
	key := strings.ToLower(strings.TrimSpace(g.Key))
	if key == "" {
		key = strings.ToLower(strings.Join(strings.Fields(g.Title), "_"))
	}

	return clip(key, 200)
}

/*
summaryOf is the line the inbox shows for a group.
*/
func summaryOf(g Group) string {
	return fmt.Sprintf("%d conversations about %s in the last %d minutes.", len(g.IDs),
		strings.ToLower(strings.TrimSpace(g.Title)), int(SpikeWindow/time.Minute))
}

/*
dbCount is a length as the database counts it.
*/
func dbCount(n int) int32 {
	return int32(min(n, 1<<30)) //nolint:gosec // capped above
}

/*
clip cuts text to at most n characters.
*/
func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}

	return s
}

/*
optionalText is nil for empty text.
*/
func optionalText(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	return &s
}
