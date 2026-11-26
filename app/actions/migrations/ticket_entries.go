/*
|--------------------------------------------------------------------------
| Ticket Entries
|--------------------------------------------------------------------------
|
| What was written on a ticket lands where a ticket made here keeps it. A
| customer ticket's messages, replies and notes go into its conversation:
| the one it was linked to, or one opened for it with its first message
| or reply, kept in the ledger as the ticket's so an undo removes it.
| Back-office tickets and trackers, which have no conversation, keep them
| as notes on the ticket, at their time and by whoever wrote them. Event
| entries are left out: state_history says how the state moved.
|
*/

package migrations

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
kindTicketConversation is the ledger kind of a conversation opened for a
ticket's entries.
*/
const kindTicketConversation = "ticket_conversation"

/*
ticketPrefix keeps the conversation of a ticket apart from a
conversation the other tool gave the same id.
*/
const ticketPrefix = "ticket:"

/*
written returns the entries a ticket keeps, oldest first: its messages,
replies and notes.
*/
func (t *Ticket) written() []Entry {
	out := slices.DeleteFunc(slices.Clone(t.Entries), func(e Entry) bool { return e.Type == EntryEvent })
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })

	return out
}

/*
checkEntries refuses entries that cannot be written.
*/
func (t *Ticket) checkEntries() error {
	if len(t.Entries) > maxEntries {
		return invalid("entries", fmt.Sprintf("A ticket carries at most %d entries.", maxEntries))
	}

	seen := map[string]bool{}

	for i, e := range t.Entries {
		if err := e.check(fmt.Sprintf("entries[%d]", i)); err != nil {
			return err
		}

		if seen[e.SourceID] {
			return invalid(fmt.Sprintf("entries[%d].source_id", i), "Each entry has its own source_id.")
		}

		seen[e.SourceID] = true
	}

	return nil
}

/*
entryRefs returns who wrote the ticket's entries.
*/
func (t *Ticket) entryRefs() []Ref {
	var out []Ref

	for _, e := range t.written() {
		out = append(out, authorRefs(&e.Author)...)
	}

	return out
}

/*
asConversation returns the ticket as a conversation, for its entries.
*/
func (t *Ticket) asConversation() *Conversation {
	return &Conversation{
		Base: t.Base, ContactRef: t.ContactRef, Subject: t.Title, CreatedAt: t.CreatedAt,
		Status: "open", Entries: t.written(), Source: ConversationSource{Type: "ticket"},
	}
}

/*
threadFor opens a conversation for a customer ticket that names none,
with its first message or reply, before the ticket is made. It returns
nil when the ticket keeps its entries as notes instead.
*/
func (s *step) threadFor(ctx context.Context, t *Ticket, kind string, out *outcome) (*replay, error) {
	entries := t.written()
	opens := slices.ContainsFunc(entries, func(e Entry) bool { return e.Type == EntryMessage || e.Type == EntryReply })
	contact, known := s.refs.id(KindContact, t.ContactRef)

	if kind != tickets.KindCustomer || !known || !opens {
		return nil, nil //nolint:nilnil // no conversation: the entries become notes
	}

	r := s.ticketReplay(t, out)
	if err := r.open(ctx, contact, entries); err != nil {
		return nil, err
	}

	if err := s.keepRow(ctx, s.q, kept{
		kind: kindTicketConversation, sourceID: t.SourceID, id: r.conv.ID, created: true,
	}); err != nil {
		return nil, err
	}

	return r, nil
}

/*
ticketReplay returns the replay that writes a ticket's entries into a
conversation.
*/
func (s *step) ticketReplay(t *Ticket, out *outcome) *replay {
	return &replay{
		s: s, c: t.asConversation(), out: out, state: convState{Fields: map[string]string{}}, prefix: ticketPrefix,
	}
}

/*
entryRun is a ticket's entries being written: the record, the ticket it
became, the conversation that keeps them when one was just opened, the
entries written before, and the outcome to warn on.
*/
type entryRun struct {
	t      *Ticket
	ticket models.Ticket
	r      *replay
	done   []string
	out    *outcome
}

/*
writeEntries writes the ticket's entries not written before: into its
conversation when it has one, else as notes on it. It returns every
entry written so far.
*/
func (s *step) writeEntries(ctx context.Context, run entryRun) ([]string, error) {
	t, ticket, r, done, out := run.t, run.ticket, run.r, run.done, run.out

	if r != nil {
		done = append(done, r.state.Entries...)
	} else {
		var err error
		if r, err = s.ticketThread(ctx, t, ticket, out); err != nil {
			return done, err
		}
	}

	for _, e := range t.written() {
		if slices.Contains(done, e.SourceID) {
			continue
		}

		var err error
		if r != nil {
			err = r.entry(ctx, e)
		} else {
			err = s.ticketNote(ctx, ticket, e)
		}

		if err != nil {
			return done, err
		}

		done = append(done, e.SourceID)
	}

	return done, nil
}

/*
ticketThread picks up the conversation a ticket keeps its entries in,
nil when it has none.
*/
func (s *step) ticketThread(ctx context.Context, t *Ticket, ticket models.Ticket, out *outcome) (*replay, error) {
	if ticket.ConversationID == nil {
		return nil, nil //nolint:nilnil // no conversation: the entries become notes
	}

	found, err := s.q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: s.ws(), ID: *ticket.ConversationID,
	})
	if err != nil || found.DeletedAt != nil {
		return nil, nil //nolint:nilerr,nilnil // a conversation gone since: the entries become notes
	}

	r := s.ticketReplay(t, out)
	r.conv = found
	err = r.load(ctx)

	return r, err
}

/*
ticketNote keeps an entry as a note on a ticket with no conversation, at
its time and by whoever wrote it.
*/
func (s *step) ticketNote(ctx context.Context, ticket models.Ticket, e Entry) error {
	w := s.ticketWriter(&e.Author)

	tw, err := w.TicketWriter(ctx, s.tx, e.CreatedAt.UTC())
	if err != nil {
		return err
	}

	if _, err := tw.Note(ctx, ticket, cut(entryBody(e), 20000)); err != nil {
		return fmt.Errorf("write ticket note: %w", err)
	}

	return nil
}
