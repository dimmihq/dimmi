/*
|--------------------------------------------------------------------------
| Matching New Conversations
|--------------------------------------------------------------------------
|
| Once a customer starts a conversation, a job asks the model whether it
| reports a problem an open tracker follows. A match waits as a
| suggestion on the conversation's panel, or is linked straight away
| when the workspace turned automatic linking on.
|
*/

package tickets

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
The statuses of a tracker suggestion.
*/
const (
	SuggestionOpen      = "open"
	SuggestionAccepted  = "accepted"
	SuggestionDismissed = "dismissed"
)

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
matchInput is what the match reads: the conversation's words, the open
trackers and the workspace's settings.
*/
type matchInput struct {
	text     string
	trackers []models.Ticket
	settings WorkspaceSettings
}

/*
MatchConversation matches a new conversation to open trackers.
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

	return t.matchTrackers(ctx, args, in)
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

		if in.trackers, err = q.ListOpenTrackers(ctx, args.WorkspaceID); err != nil {
			return fmt.Errorf("list open trackers: %w", err)
		}

		in.settings, err = readSettings(ctx, q, args.WorkspaceID)

		return err
	})

	return in, err
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
candidates are the tickets as the model reads them: the title, then the
description.
*/
func candidates(list []models.Ticket) []MatchCandidate {
	out := make([]MatchCandidate, len(list))

	for i, r := range list {
		out[i] = MatchCandidate{ID: r.ID, Title: r.Title, Description: r.Description}
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
linkConversations links each live conversation not linked yet.
*/
func (w *Writer) linkConversations(ctx context.Context, ws, ticket ids.UUID, convs []ids.UUID) error {
	locked, err := w.lockRow(ctx, ws, ticket)
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
	}

	return nil
}

/*
keepKnown keeps the ids of the list that are among the known ones, in
the list's order and once each.
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
