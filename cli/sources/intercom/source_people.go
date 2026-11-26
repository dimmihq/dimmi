package intercom

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

func (s *Source) admins(ctx context.Context) ([]admin, error) {
	var out struct {
		Admins []admin `json:"admins"`
	}

	err := s.Client.get(ctx, "/admins", url.Values{"display_avatar": {"true"}}, &out)

	return out.Admins, err
}

/*
knownAdmins returns the ids GET /admins lists, read once.
*/
func (s *Source) knownAdmins(ctx context.Context) (map[string]bool, error) {
	s.mu.Lock()
	known := s.known
	s.mu.Unlock()

	if known != nil {
		return known, nil
	}

	list, err := s.admins(ctx)
	if err != nil {
		return nil, err
	}

	known = map[string]bool{}
	for _, a := range list {
		known[string(a.ID)] = true
	}

	s.mu.Lock()
	s.known = known
	if s.former == nil {
		s.former = map[string]Former{}
	}
	s.mu.Unlock()

	return known, nil
}

func (s *Source) readTeammates(ctx context.Context, _ engine.ReadOptions, emit func(engine.Page) error) error {
	list, err := s.admins(ctx)
	if err != nil {
		return err
	}

	recs := make([]format.Record, 0, len(list))
	for _, a := range list {
		if a.Type == "team" {
			continue
		}

		recs = append(recs, translateAdmin(a, true))
	}

	return one(emit, recs)
}

func (s *Source) readTeams(ctx context.Context, _ engine.ReadOptions, emit func(engine.Page) error) error {
	var out struct {
		Teams []team `json:"teams"`
	}

	if err := s.Client.get(ctx, "/teams", nil, &out); err != nil {
		return err
	}

	recs := make([]format.Record, 0, len(out.Teams))
	for _, t := range out.Teams {
		recs = append(recs, translateTeam(t))
	}

	return one(emit, recs)
}

func (s *Source) readTags(ctx context.Context, _ engine.ReadOptions, emit func(engine.Page) error) error {
	var out list[tag]
	if err := s.Client.get(ctx, "/tags", nil, &out); err != nil {
		return err
	}

	recs := make([]format.Record, 0, len(out.Data))
	for _, t := range out.Data {
		recs = append(recs, translateTag(t))
	}

	return one(emit, recs)
}

func (s *Source) readAttributes(ctx context.Context, _ engine.ReadOptions, emit func(engine.Page) error) error {
	var recs []format.Record

	for _, model := range []string{"contact", "company"} {
		var out list[dataAttribute]

		q := url.Values{"model": {model}, "include_archived": {"true"}}
		if err := s.Client.get(ctx, "/data_attributes", q, &out); err != nil {
			return err
		}

		for _, a := range out.Data {
			if a.Model == "" {
				a.Model = model
			}

			if rec, ok := translateDataAttribute(a); ok {
				recs = append(recs, rec)
			}
		}
	}

	conv, err := s.conversationAttributes(ctx)
	if err != nil {
		return err
	}

	return one(emit, append(recs, conv...))
}

/*
conversationAttributes reads the conversation attributes, none when the
workspace has none or the token can't read them.
*/
func (s *Source) conversationAttributes(ctx context.Context) ([]format.Record, error) {
	var conv list[conversationAttribute]

	err := s.Client.get(ctx, "/conversations/attributes", url.Values{"include_archived": {"true"}}, &conv)
	if err != nil && !isNotFound(err) && !isForbidden(err) {
		return nil, err
	}

	var recs []format.Record

	for _, a := range conv.Data {
		if rec, ok := translateConversationAttribute(a); ok {
			recs = append(recs, rec)
		}
	}

	return recs, nil
}

/*
readCompanies scrolls every company: the list and search endpoints
leave out companies without users.
*/
func (s *Source) readCompanies(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	return s.scrollCompanies(ctx, opts.Cursor, func(list []company, next string) error {
		return emit(engine.Page{Records: companyRecords(list), Cursor: next})
	})
}

/*
scrollWait is how long a scroll left open stays open: Intercom allows
one per workspace, and closes one unused for a minute.
*/
var scrollWait = 65 * time.Second //nolint:gochecknoglobals // shortened in tests

/*
scrollCompanies scrolls every company from the cursor on, a page at a
time. A resumed scroll that expired starts again; records already sent
are skipped by the engine. When another scroll is open, such as one an
interrupted run left, it waits for that one to close.
*/
func (s *Source) scrollCompanies(ctx context.Context, cursor string, fn func([]company, string) error) error {
	waited := 0

	for {
		q := url.Values{}
		if cursor != "" {
			q.Set("scroll_param", cursor)
		}

		var out list[company]

		err := s.Client.get(ctx, "/companies/scroll", q, &out)

		switch {
		case err != nil && cursor != "" && isNotFound(err):
			cursor = ""

			continue
		case isScrollOpen(err) && waited < 3:
			waited++

			if err := sleep(ctx, scrollWait); err != nil {
				return err
			}

			continue
		case err != nil:
			return err
		case len(out.Data) == 0:
			return nil
		}

		if err := fn(out.Data, out.Scroll); err != nil {
			return err
		}

		cursor = out.Scroll
		if cursor == "" {
			return nil
		}
	}
}

/*
isScrollOpen reports whether Intercom refused a scroll because another
is open.
*/
func isScrollOpen(err error) bool {
	var e *APIError

	return errors.As(err, &e) && e.Code == "scroll_exists"
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func companyRecords(list []company) []format.Record {
	recs := make([]format.Record, 0, len(list))
	for _, c := range list {
		recs = append(recs, translateCompany(c))
	}

	return recs
}

/*
contactPage reads one page of contacts: the whole list on a first run,
those updated since on a sync.
*/
func (s *Source) contactPage(ctx context.Context, since time.Time, cursor string) ([]contact, string, error) {
	var out list[contact]

	var err error
	if since.IsZero() {
		q := url.Values{"per_page": {"150"}, "escape_identity": {"false"}}
		if cursor != "" {
			q.Set("starting_after", cursor)
		}

		err = s.Client.get(ctx, "/contacts", q, &out)
	} else {
		err = s.Client.post(ctx, "/contacts/search", changedSince(since.Unix(), cursor), &out)
	}

	if err != nil {
		return nil, "", err
	}

	return out.Data, nextCursor(out.Pages.Next), nil
}

func pagination(cursor string) map[string]any {
	p := map[string]any{"per_page": 150}
	if cursor != "" {
		p["starting_after"] = cursor
	}

	return p
}

func (s *Source) readContacts(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	return s.eachContact(ctx, opts, emit, s.fullContact)
}

/*
readContactNotes reads the notes on every contact, then on every
company. The cursor is prefixed c: once on companies.
*/
func (s *Source) readContactNotes(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	cursor, onCompanies := strings.CutPrefix(opts.Cursor, "c:")

	if !onCompanies {
		contacts := opts
		contacts.Cursor = cursor

		err := s.eachContact(ctx, contacts, func(p engine.Page) error {
			if p.Cursor == "" {
				p.Cursor = "c:"
			}

			return emit(p)
		}, s.contactNotes)
		if err != nil {
			return err
		}

		cursor = ""
	}

	return s.scrollCompanies(ctx, cursor, func(list []company, next string) error {
		var withNotes []company
		for _, c := range list {
			if !c.Notes.empty() {
				withNotes = append(withNotes, c)
			}
		}

		recs, err := parallel(ctx, s.Workers, withNotes, s.companyNotes)
		if err != nil {
			return err
		}

		if next != "" {
			next = "c:" + next
		}

		return emit(engine.Page{Records: recs, Cursor: next})
	})
}

/*
eachContact reads contacts page by page and turns each with fn.
*/
func (s *Source) eachContact(
	ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error,
	fn func(context.Context, contact) ([]format.Record, error),
) error {
	cursor := opts.Cursor

	for {
		page, next, err := s.contactPage(ctx, opts.Since, cursor)
		if err != nil {
			return err
		}

		recs, err := parallel(ctx, s.Workers, page, fn)
		if err != nil {
			return err
		}

		if err := emit(engine.Page{Records: recs, Cursor: next}); err != nil {
			return err
		}

		if next == "" || len(page) == 0 {
			return nil
		}

		cursor = next
	}
}

/*
fullContact reads the tags and companies a contact's own lists leave
out past their first ten.
*/
func (s *Source) fullContact(ctx context.Context, c contact) ([]format.Record, error) {
	tags := refIDs(c.Tags.Data)
	if c.Tags.HasMore {
		var out list[tag]
		if err := s.Client.get(ctx, "/contacts/"+url.PathEscape(string(c.ID))+"/tags", nil, &out); err != nil {
			return nil, err
		}

		tags = nil
		for _, t := range out.Data {
			tags = append(tags, string(t.ID))
		}
	}

	companies := refIDs(c.Companies.Data)
	if c.Companies.HasMore {
		all, err := s.contactCompanies(ctx, c.ID)
		if err != nil {
			return nil, err
		}

		companies = all
	}

	return []format.Record{translateContact(c, tags, companies)}, nil
}

func refIDs(refs []ref) []string {
	var out []string
	for _, r := range refs {
		if r.ID != "" {
			out = append(out, string(r.ID))
		}
	}

	return out
}

func (s *Source) contactCompanies(ctx context.Context, id ID) ([]string, error) {
	var ids []string

	for page := 1; page < 1000; page++ {
		var out list[company]

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"50"}}
		if err := s.Client.get(ctx, "/contacts/"+url.PathEscape(string(id))+"/companies", q, &out); err != nil {
			return nil, err
		}

		for _, c := range out.Data {
			ids = append(ids, string(c.ID))
		}

		if len(out.Data) == 0 || int(out.Pages.TotalPages) <= page {
			break
		}
	}

	return ids, nil
}

/*
contactNotes reads a contact's notes, oldest first. A contact whose
notes list is empty is not asked.
*/
func (s *Source) contactNotes(ctx context.Context, c contact) ([]format.Record, error) {
	if c.Notes.empty() {
		return nil, nil
	}

	return s.notes(ctx, "/contacts/"+url.PathEscape(string(c.ID))+"/notes", func(n note) format.Record {
		return translateNote(n, string(c.ID), "")
	})
}

/*
noteAuthors returns teammate records for the authors of notes who were
removed from Intercom, before the notes themselves.
*/
func (s *Source) noteAuthors(ctx context.Context, list []note) ([]format.Record, error) {
	known, err := s.knownAdmins(ctx)
	if err != nil {
		return nil, err
	}

	var former []Former

	for _, n := range list {
		if n.Author.ID != "" && !known[string(n.Author.ID)] {
			former = append(former, Former{
				ID: string(n.Author.ID), Name: text(n.Author.Name), Email: text(n.Author.Email),
			})
		}
	}

	return s.newFormer(former), nil
}

/*
companyNotes reads a company's notes, oldest first.
*/
func (s *Source) companyNotes(ctx context.Context, c company) ([]format.Record, error) {
	recs, err := s.notes(ctx, "/companies/"+url.PathEscape(string(c.ID))+"/notes", func(n note) format.Record {
		return translateNote(n, "", string(c.ID))
	})
	if isNotFound(err) {
		return nil, nil
	}

	return recs, err
}

/*
notes reads every page of a notes list.
*/
func (s *Source) notes(ctx context.Context, path string, fn func(note) format.Record) ([]format.Record, error) {
	var recs []format.Record
	var all []note

	for page := 1; page < 1000; page++ {
		var out list[note]

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"60"}, "order": {"asc"}}
		if err := s.Client.get(ctx, path, q, &out); err != nil {
			return nil, err
		}

		for _, n := range out.Data {
			recs = append(recs, fn(n))
		}

		all = append(all, out.Data...)

		if len(out.Data) == 0 || int(out.Pages.TotalPages) <= page {
			break
		}
	}

	authors, err := s.noteAuthors(ctx, all)
	if err != nil {
		return nil, err
	}

	return append(authors, recs...), nil
}

func (s *Source) readTicketTypes(ctx context.Context, _ engine.ReadOptions, emit func(engine.Page) error) error {
	var out list[ticketType]
	if err := s.Client.get(ctx, "/ticket_types", nil, &out); err != nil {
		return err
	}

	recs := make([]format.Record, 0, len(out.Data))
	for _, t := range out.Data {
		recs = append(recs, translateTicketType(t))
	}

	return one(emit, recs)
}
