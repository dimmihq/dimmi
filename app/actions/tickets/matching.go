/*
|--------------------------------------------------------------------------
| Matching New Conversations
|--------------------------------------------------------------------------
|
| Once a customer starts a conversation, a job asks the model whether it
| reports an incident active now or a problem an open tracker follows.
| A match to an incident links the conversation and tells the customer
| where it stands at once: the state's customer label and the latest
| public post. A match to a tracker waits as a suggestion on the
| conversation's panel, or is linked straight away when the workspace
| turned automatic linking on.
|
*/

package tickets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxTrackerSuggestions is how many trackers one conversation is
suggested.
*/
const maxTrackerSuggestions = 3

/*
ListenForMatches queues the match of every conversation a customer
starts, once it is saved.
*/
func (t *Tickets) ListenForMatches(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MessageCreated) error {
		if !e.ConversationCreated || e.Type != string(enums.MessageTypeMessage) ||
			e.AuthorType != string(enums.AuthorTypeContact) {
			return nil
		}

		return t.Queue.Push(context.WithoutCancel(ctx), jobs.MatchTicketConversationArgs{
			WorkspaceID: e.WorkspaceID, ConversationID: e.ConversationID,
		})
	})
}

/*
matchInput is what the match reads: the conversation's words, the
incidents active now, the open trackers and the workspace's settings.
*/
type matchInput struct {
	text      string
	incidents []models.Ticket
	trackers  []models.Ticket
	settings  WorkspaceSettings
}

/*
MatchConversation matches a new conversation to an active incident and
to open trackers.
*/
func (t *Tickets) MatchConversation(ctx context.Context, args jobs.MatchTicketConversationArgs) error {
	if t.Filler == nil {
		return nil
	}

	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	in, err := t.readMatch(ctx, args)
	if err != nil || strings.TrimSpace(in.text) == "" {
		return err
	}

	return errors.Join(t.matchIncident(ctx, args, in), t.matchTrackers(ctx, args, in))
}

/*
readMatch loads what the match reads; a conversation gone reads empty.
*/
func (t *Tickets) readMatch(ctx context.Context, args jobs.MatchTicketConversationArgs) (matchInput, error) {
	var in matchInput

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: args.WorkspaceID, ID: args.ConversationID,
		})
		if err != nil || c.DeletedAt != nil || c.Spam {
			return nil //nolint:nilerr // a conversation gone matches nothing
		}

		if in.text, err = transcript(ctx, q, c); err != nil {
			return err
		}

		if in.incidents, err = q.ListActiveIncidents(ctx, args.WorkspaceID); err != nil {
			return fmt.Errorf("list active incidents: %w", err)
		}

		if in.trackers, err = q.ListOpenTrackers(ctx, args.WorkspaceID); err != nil {
			return fmt.Errorf("list open trackers: %w", err)
		}

		in.settings, err = readSettings(ctx, q, args.WorkspaceID)

		return err
	})

	return in, err
}

/*
matchIncident links the conversation to the incident it reports and
tells the customer the latest.
*/
func (t *Tickets) matchIncident(ctx context.Context, args jobs.MatchTicketConversationArgs, in matchInput) error {
	if len(in.incidents) == 0 {
		return nil
	}

	best, err := t.bestMatch(ctx, MatchQuestion{
		WorkspaceID: args.WorkspaceID, ConversationID: args.ConversationID, Text: in.text,
		Candidates: candidates(in.incidents),
	})
	if err != nil || best == nil {
		return err
	}

	return t.linkAll(ctx, args.WorkspaceID, *best, []ids.UUID{args.ConversationID})
}

/*
matchTrackers suggests the trackers the conversation reports, or links
the best when the workspace links duplicates itself.
*/
func (t *Tickets) matchTrackers(ctx context.Context, args jobs.MatchTicketConversationArgs, in matchInput) error {
	if len(in.trackers) == 0 {
		return nil
	}

	found, err := t.Filler.Match(ctx, MatchQuestion{
		WorkspaceID: args.WorkspaceID, ConversationID: args.ConversationID, Text: in.text,
		Candidates: candidates(in.trackers),
	})
	if err != nil {
		return fmt.Errorf("match trackers: %w", err)
	}

	found = keepKnown(found, ticketIDs(in.trackers))
	if len(found) > maxTrackerSuggestions {
		found = found[:maxTrackerSuggestions]
	}

	if len(found) == 0 {
		return nil
	}

	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.systemWriter(ctx, tx, args.WorkspaceID)
		if err != nil {
			return err
		}

		if in.settings.AutoLinkDuplicates {
			err := w.linkConversations(ctx, args.WorkspaceID, found[0], []ids.UUID{args.ConversationID})
			if err != nil {
				return err
			}
		}

		best := SuggestionOpen
		if in.settings.AutoLinkDuplicates {
			best = SuggestionAccepted
		}

		return w.suggestTrackers(ctx, args, found, best)
	})
}

/*
suggestTrackers stores the matches, the best with its own status:
accepted when it was linked at once.
*/
func (w *Writer) suggestTrackers(
	ctx context.Context, args jobs.MatchTicketConversationArgs, found []ids.UUID, best string,
) error {
	for i, id := range found {
		status := SuggestionOpen
		if i == 0 {
			status = best
		}

		err := w.Q.CreateTrackerSuggestion(ctx, models.CreateTrackerSuggestionParams{
			WorkspaceID: args.WorkspaceID, ConversationID: args.ConversationID, TicketID: id, Rank: int32(i),
			Status: status, Now: w.Now,
		})
		if err != nil {
			return fmt.Errorf("store tracker suggestion: %w", err)
		}
	}

	return nil
}

/*
bestMatch returns the candidate the model ranks first, nil for none.
*/
func (t *Tickets) bestMatch(ctx context.Context, q MatchQuestion) (*ids.UUID, error) {
	found, err := t.Filler.Match(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("match conversation: %w", err)
	}

	known := make([]ids.UUID, len(q.Candidates))
	for i, c := range q.Candidates {
		known[i] = c.ID
	}

	found = keepKnown(found, known)
	if len(found) == 0 {
		return nil, nil
	}

	return &found[0], nil
}

/*
candidates are the tickets as the model reads them: the title, then the
description and the affected areas.
*/
func candidates(list []models.Ticket) []MatchCandidate {
	out := make([]MatchCandidate, len(list))

	for i, r := range list {
		desc := r.Description
		if areas := areasOf(r); len(areas) > 0 {
			desc = strings.TrimSpace(desc + " Affected areas: " + strings.Join(areas, ", ") + ".")
		}

		out[i] = MatchCandidate{ID: r.ID, Title: r.Title, Description: desc}
	}

	return out
}

/*
areasOf reads an incident's affected areas.
*/
func areasOf(r models.Ticket) []string {
	var out []string

	switch v := objectOf(r.Fields)["areas"].(type) {
	case []any:
		for _, a := range v {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
	case string:
		out = append(out, v)
	}

	return out
}

/*
ticketIDs returns the tickets' ids.
*/
func ticketIDs(list []models.Ticket) []ids.UUID {
	out := make([]ids.UUID, len(list))
	for i, r := range list {
		out[i] = r.ID
	}

	return out
}

/*
linkAll links the conversations to the incident or tracker as dimmi.
*/
func (t *Tickets) linkAll(ctx context.Context, ws, ticket ids.UUID, convs []ids.UUID) error {
	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.systemWriter(ctx, tx, ws)
		if err != nil {
			return err
		}

		return w.linkConversations(ctx, ws, ticket, convs)
	})
}

/*
linkConversations links each live conversation not linked yet, and
tells each newly linked customer an incident's latest.
*/
func (w *Writer) linkConversations(ctx context.Context, ws, ticket ids.UUID, convs []ids.UUID) error {
	locked, err := w.lock(ctx, ws, ticket)
	if err != nil || locked.DeletedAt != nil || locked.StateGroup == GroupResolved {
		return nil //nolint:nilerr // work gone or resolved takes no more links
	}

	for _, conv := range convs {
		link, err := w.Link(ctx, locked, LinkInput{Kind: LinkTracker, ConversationID: &conv})
		if errors.Is(err, ErrLinkInvalid) || link.ID == ids.Nil() {
			continue
		}

		if err != nil {
			return err
		}

		if locked.Kind == KindIncident {
			if err := w.tellIncident(ctx, locked, conv); err != nil {
				return err
			}
		}
	}

	return nil
}

/*
tellIncident tells a customer whose conversation joined an incident
where it stands: the state's customer label and the latest public
post. A customer who cannot be reached is skipped.
*/
func (w *Writer) tellIncident(ctx context.Context, incident models.Ticket, conv ids.UUID) error {
	if !incident.CustomerVisible {
		return nil
	}

	body, err := w.incidentNotice(ctx, incident)
	if err != nil {
		return err
	}

	key := "incident-status:" + incident.ID.String() + ":" + conv.String()

	_, err = w.Owner.send(ctx, w.Writer, incident.WorkspaceID, sendInput{conversation: conv, body: body, dedupe: key})

	return err
}

/*
incidentNotice writes what a customer reads about an incident.
*/
func (w *Writer) incidentNotice(ctx context.Context, incident models.Ticket) (string, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "**%s**", incident.Title)

	if st, ok := w.Setup.State(incident.StateID); ok && st.CustomerLabel != "" {
		fmt.Fprintf(&b, "\n\n%s", st.CustomerLabel)
	}

	latest, err := w.Q.ListLatestPublicUpdates(ctx, models.ListLatestPublicUpdatesParams{
		WorkspaceID: incident.WorkspaceID, TicketIds: []ids.UUID{incident.ID},
	})
	if err != nil {
		return "", fmt.Errorf("read latest public update: %w", err)
	}

	if len(latest) > 0 {
		fmt.Fprintf(&b, "\n\nLatest update: %s", latest[0].Body)
	}

	return b.String(), nil
}
