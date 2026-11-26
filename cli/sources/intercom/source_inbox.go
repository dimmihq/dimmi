package intercom

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

/*
search reads one page of a search for what changed after since: every
conversation or ticket on a first run.
*/
func (s *Source) search(ctx context.Context, path string, page searchPage, out any) error {
	value := int64(0)
	if !page.since.IsZero() {
		value = page.since.Unix()
	}

	return s.Client.post(ctx, path, changedSince(value, page.cursor), out)
}

/*
changedSince is the body of a search for what changed after the Unix
time, oldest change first. Intercom sorts newest first unless told; read
that way, a record that changes during a long read moves behind the
cursor and is missed until the next sync. Oldest first, it moves ahead
and is read again.
*/
func changedSince(unix int64, cursor string) map[string]any {
	return map[string]any{
		"query":      map[string]any{"field": "updated_at", "operator": ">", "value": unix},
		"pagination": pagination(cursor),
		"sort":       map[string]any{"field": "updated_at", "order": "ascending"},
	}
}

/*
searchPage is the page of a search to read: what changed after since,
from the cursor on.
*/
type searchPage struct {
	since  time.Time
	cursor string
}

func (s *Source) readConversations(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	known, err := s.knownAdmins(ctx)
	if err != nil {
		return err
	}

	cursor := opts.Cursor

	for {
		var out struct {
			Conversations []conversation `json:"conversations"`
			Pages         struct {
				Next json.RawMessage `json:"next"`
			} `json:"pages"`
		}

		if err := s.search(ctx, "/conversations/search", searchPage{opts.Since, cursor}, &out); err != nil {
			return err
		}

		recs, err := parallel(ctx, s.Workers, out.Conversations,
			func(ctx context.Context, c conversation) ([]format.Record, error) {
				return s.conversationRecords(ctx, c.ID, known)
			})
		if err != nil {
			return err
		}

		next := nextCursor(out.Pages.Next)
		if err := emit(engine.Page{Records: recs, Cursor: next}); err != nil {
			return err
		}

		if next == "" || len(out.Conversations) == 0 {
			return nil
		}

		cursor = next
	}
}

/*
newFormer returns teammate records for former teammates not seen yet.
*/
func (s *Source) newFormer(list []Former) []format.Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.former == nil {
		s.former = map[string]Former{}
	}

	var recs []format.Record

	for _, f := range list {
		if _, ok := s.former[f.ID]; ok {
			continue
		}

		s.former[f.ID] = f

		// A teammate is known to dimmi by their email; one Intercom gives
		// no email for is left out, and the guard drops what names them.
		if f.Email != "" {
			recs = append(recs, translateFormer(f))
		}
	}

	return recs
}

/*
conversation reads a conversation and every one of its parts. Without
page, Intercom returns only the latest 500 parts; with it, parts come
oldest first, 500 a page, for conversation_parts.pages.total_pages
pages.
*/
func (s *Source) conversation(ctx context.Context, id ID) (conversation, []part, error) {
	var first conversation
	var parts []part

	for page := 1; page < 10000; page++ {
		var c conversation

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"500"}}
		if err := s.Client.get(ctx, "/conversations/"+url.PathEscape(string(id)), q, &c); err != nil {
			return first, nil, err
		}

		if page == 1 {
			first = c
		}

		parts = append(parts, c.Parts.Parts...)

		if lastPartsPage(page, len(c.Parts.Parts), int(c.Parts.Pages.TotalPages)) {
			break
		}
	}

	return first, parts, nil
}

/*
lastPartsPage reports whether a page of n parts is the last: an empty
page, the last of the pages Intercom counts, or a short page when it
counts none.
*/
func lastPartsPage(page, n, totalPages int) bool {
	if n == 0 {
		return true
	}

	if totalPages > 0 {
		return page >= totalPages
	}

	return n < 500
}

func (s *Source) readTickets(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	known, err := s.knownAdmins(ctx)
	if err != nil {
		return err
	}

	cursor := opts.Cursor

	for {
		var out struct {
			Tickets []ticket `json:"tickets"`
			Pages   struct {
				Next json.RawMessage `json:"next"`
			} `json:"pages"`
		}

		if err := s.search(ctx, "/tickets/search", searchPage{opts.Since, cursor}, &out); err != nil {
			return err
		}

		recs, err := parallel(ctx, s.Workers, out.Tickets,
			func(ctx context.Context, t ticket) ([]format.Record, error) {
				return s.ticketRecords(ctx, t.ID, known)
			})
		if err != nil {
			return err
		}

		next := nextCursor(out.Pages.Next)
		if err := emit(engine.Page{Records: recs, Cursor: next}); err != nil {
			return err
		}

		if next == "" || len(out.Tickets) == 0 {
			return nil
		}

		cursor = next
	}
}

/*
conversationRecords reads and turns one conversation, after the former
teammates it names who weren't seen yet. One deleted since the search
gives nothing.
*/
func (s *Source) conversationRecords(ctx context.Context, id ID, known map[string]bool) ([]format.Record, error) {
	full, parts, err := s.conversation(ctx, id)
	if isNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	rec, former := translateConversation(full, parts, known)

	return append(s.newFormer(former), rec), nil
}

/*
ticketRecords reads and turns one ticket, after the former teammates it
names who weren't seen yet. One deleted since the search gives nothing.
*/
func (s *Source) ticketRecords(ctx context.Context, id ID, known map[string]bool) ([]format.Record, error) {
	var full ticket

	err := s.Client.get(ctx, "/tickets/"+url.PathEscape(string(id)), nil, &full)
	if isNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	rec, former := translateTicket(full, known)

	return append(s.newFormer(former), rec), nil
}
