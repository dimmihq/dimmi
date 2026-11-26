package intercom

import (
	"context"
	"net/url"
	"strconv"
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
leave out companies without users. A scroll expires after a minute
idle; a resumed read whose scroll expired starts again, and records
already sent are skipped by the engine.
*/
func (s *Source) readCompanies(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	cursor := opts.Cursor

	for {
		q := url.Values{}
		if cursor != "" {
			q.Set("scroll_param", cursor)
		}

		var out list[company]

		err := s.Client.get(ctx, "/companies/scroll", q, &out)
		if err != nil && cursor != "" && !isForbidden(err) {
			cursor = ""

			continue
		}

		if err != nil || len(out.Data) == 0 {
			return err
		}

		if err := emit(engine.Page{Records: companyRecords(out.Data), Cursor: out.Scroll}); err != nil {
			return err
		}

		cursor = out.Scroll
		if cursor == "" {
			return nil
		}
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
		body := map[string]any{
			"query":      map[string]any{"field": "updated_at", "operator": ">", "value": since.Unix()},
			"pagination": pagination(cursor),
		}

		err = s.Client.post(ctx, "/contacts/search", body, &out)
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

func (s *Source) readContactNotes(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	return s.eachContact(ctx, opts, emit, s.contactNotes)
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

func (s *Source) contactNotes(ctx context.Context, c contact) ([]format.Record, error) {
	var recs []format.Record

	for page := 1; page < 1000; page++ {
		var out list[note]

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"60"}, "order": {"asc"}}
		if err := s.Client.get(ctx, "/contacts/"+url.PathEscape(string(c.ID))+"/notes", q, &out); err != nil {
			return nil, err
		}

		for _, n := range out.Data {
			recs = append(recs, translateNote(n, string(c.ID)))
		}

		if len(out.Data) < 60 || int(out.Pages.TotalPages) <= page {
			break
		}
	}

	return recs, nil
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
