package intercom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/engine"
	"github.com/dimmihq/dimmi/cmd/dimmi-cli/format"
)

/*
Source reads an Intercom workspace. Workflows lists the workflows to
export by their ruleset id, the number at the end of a workflow's URL,
since Intercom has no way to list them. Workers is how many records are
read in parallel within a page.
*/
type Source struct {
	Client    *Client
	Region    string
	Workflows []string
	Workers   int

	mu          sync.Mutex
	account     *engine.Account
	known       map[string]bool
	former      map[string]Former
	centers     map[string]string
	defaultHC   string
	defaultLang string
}

/*
New returns a source over the client.
*/
func New(c *Client, region string, workflows []string) *Source {
	return &Source{Client: c, Region: region, Workflows: workflows, Workers: 8}
}

func (s *Source) Name() string { return "intercom" }

/*
Kinds lists what is read, in import order.
*/
func (s *Source) Kinds() []format.Kind {
	kinds := []format.Kind{
		format.KindTeammate, format.KindTeam, format.KindTag, format.KindAttributeDefinition, format.KindCompany,
		format.KindContact, format.KindContactNote, format.KindTicketType, format.KindHelpCenter, format.KindCollection,
		format.KindArticle, format.KindSnippet, format.KindSavedReply, format.KindSchedule, format.KindConversation,
		format.KindTicket,
	}

	if len(s.Workflows) > 0 {
		kinds = append(kinds, format.KindWorkflow)
	}

	return kinds
}

/*
Account reads the workspace the token belongs to.
*/
func (s *Source) Account(ctx context.Context) (engine.Account, error) {
	s.mu.Lock()
	if s.account != nil {
		a := *s.account
		s.mu.Unlock()

		return a, nil
	}
	s.mu.Unlock()

	var m me
	if err := s.Client.get(ctx, "/me", nil, &m); err != nil {
		return engine.Account{}, err
	}

	a := engine.Account{ID: m.App.IDCode, Name: text(m.App.Name), Region: s.Region}
	if a.ID == "" {
		return a, errors.New("Intercom didn't say which workspace this token belongs to")
	}

	s.mu.Lock()
	s.account = &a
	s.mu.Unlock()

	return a, nil
}

/*
Open downloads a file.
*/
func (s *Source) Open(ctx context.Context, f *format.File) (io.ReadCloser, error) {
	return s.Client.Open(ctx, f.Source)
}

/*
Read reads one kind page by page.
*/
func (s *Source) Read(ctx context.Context, kind format.Kind, opts engine.ReadOptions, emit func(engine.Page) error) error {
	switch kind {
	case format.KindTeammate:
		return s.readTeammates(ctx, emit)
	case format.KindTeam:
		return s.readTeams(ctx, emit)
	case format.KindTag:
		return s.readTags(ctx, emit)
	case format.KindAttributeDefinition:
		return s.readAttributes(ctx, emit)
	case format.KindCompany:
		return s.readCompanies(ctx, opts, emit)
	case format.KindContact:
		return s.readContacts(ctx, opts, emit, false)
	case format.KindContactNote:
		return s.readContacts(ctx, opts, emit, true)
	case format.KindTicketType:
		return s.readTicketTypes(ctx, emit)
	case format.KindHelpCenter:
		return s.readHelpCenters(ctx, emit)
	case format.KindCollection:
		return s.readCollections(ctx, opts, emit)
	case format.KindArticle:
		return s.readArticles(ctx, opts, emit)
	case format.KindSnippet:
		return s.readSnippets(ctx, opts, emit)
	case format.KindSavedReply:
		return s.readMacros(ctx, opts, emit)
	case format.KindSchedule:
		return s.readSchedules(ctx, emit)
	case format.KindConversation:
		return s.readConversations(ctx, opts, emit)
	case format.KindTicket:
		return s.readTickets(ctx, opts, emit)
	case format.KindWorkflow:
		return s.readWorkflows(ctx, opts, emit)
	}

	return fmt.Errorf("Intercom has no %s", kind)
}

func one(emit func(engine.Page) error, recs []format.Record) error {
	return emit(engine.Page{Records: recs})
}

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

func (s *Source) readTeammates(ctx context.Context, emit func(engine.Page) error) error {
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

func (s *Source) readTeams(ctx context.Context, emit func(engine.Page) error) error {
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

func (s *Source) readTags(ctx context.Context, emit func(engine.Page) error) error {
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

func (s *Source) readAttributes(ctx context.Context, emit func(engine.Page) error) error {
	var recs []format.Record

	for _, model := range []string{"contact", "company"} {
		var out list[dataAttribute]
		if err := s.Client.get(ctx, "/data_attributes", url.Values{"model": {model}, "include_archived": {"true"}}, &out); err != nil {
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

	var conv list[conversationAttribute]

	err := s.Client.get(ctx, "/conversations/attributes", url.Values{"include_archived": {"true"}}, &conv)
	if err != nil && !isNotFound(err) && !isForbidden(err) {
		return err
	}

	for _, a := range conv.Data {
		if rec, ok := translateConversationAttribute(a); ok {
			recs = append(recs, rec)
		}
	}

	return one(emit, recs)
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

		if err != nil {
			return err
		}

		if len(out.Data) == 0 {
			return nil
		}

		recs := make([]format.Record, 0, len(out.Data))
		for _, c := range out.Data {
			recs = append(recs, translateCompany(c))
		}

		if err := emit(engine.Page{Records: recs, Cursor: out.Scroll}); err != nil {
			return err
		}

		cursor = out.Scroll
		if cursor == "" {
			return nil
		}
	}
}

/*
contactPage reads one page of contacts: the whole list on a first run,
those updated since on a sync.
*/
func (s *Source) contactPage(ctx context.Context, since time.Time, cursor string) ([]contact, string, error) {
	var out list[contact]

	if since.IsZero() {
		q := url.Values{"per_page": {"150"}, "escape_identity": {"false"}}
		if cursor != "" {
			q.Set("starting_after", cursor)
		}

		if err := s.Client.get(ctx, "/contacts", q, &out); err != nil {
			return nil, "", err
		}
	} else {
		body := map[string]any{
			"query":      map[string]any{"field": "updated_at", "operator": ">", "value": since.Unix()},
			"pagination": pagination(cursor),
		}

		if err := s.Client.post(ctx, "/contacts/search", body, &out); err != nil {
			return nil, "", err
		}
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

func (s *Source) readContacts(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error, notes bool) error {
	cursor := opts.Cursor

	for {
		page, next, err := s.contactPage(ctx, opts.Since, cursor)
		if err != nil {
			return err
		}

		recs, err := parallel(ctx, s.Workers, page, func(ctx context.Context, c contact) ([]format.Record, error) {
			if notes {
				return s.contactNotes(ctx, c)
			}

			return s.fullContact(ctx, c)
		})
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

		if len(out.Data) == 0 || out.Pages.TotalPages <= page {
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

		if len(out.Data) < 60 || out.Pages.TotalPages <= page {
			break
		}
	}

	return recs, nil
}

func (s *Source) readTicketTypes(ctx context.Context, emit func(engine.Page) error) error {
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

/*
helpCenters reads the help centers once, with the collection each
belongs to, so articles can be placed in theirs.
*/
func (s *Source) helpCenters(ctx context.Context) ([]helpCenter, error) {
	var out list[helpCenter]
	if err := s.Client.get(ctx, "/help_center/help_centers", nil, &out); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, h := range out.Data {
		if h.Default || s.defaultHC == "" {
			s.defaultHC = string(h.ID)
		}
	}

	return out.Data, nil
}

func (s *Source) readHelpCenters(ctx context.Context, emit func(engine.Page) error) error {
	centers, err := s.helpCenters(ctx)
	if err != nil {
		return err
	}

	lang, err := s.defaultLocale(ctx)
	if err != nil {
		return err
	}

	recs := make([]format.Record, 0, len(centers))
	for _, h := range centers {
		recs = append(recs, translateHelpCenter(h, lang))
	}

	return one(emit, recs)
}

/*
defaultLocale is the workspace's help center language: the default
locale its collections and articles carry.
*/
func (s *Source) defaultLocale(ctx context.Context) (string, error) {
	s.mu.Lock()
	lang := s.defaultLang
	s.mu.Unlock()

	if lang != "" {
		return lang, nil
	}

	var out list[collection]
	if err := s.Client.get(ctx, "/help_center/collections", url.Values{"per_page": {"1"}}, &out); err != nil {
		return "", err
	}

	lang = "en"
	if len(out.Data) > 0 && out.Data[0].DefaultLocale != "" {
		lang = out.Data[0].DefaultLocale
	}

	s.mu.Lock()
	s.defaultLang = lang
	s.mu.Unlock()

	return lang, nil
}

func pageCursor(cursor string) int {
	n, err := strconv.Atoi(cursor)
	if err != nil || n < 1 {
		return 1
	}

	return n
}

func (s *Source) readCollections(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	if _, err := s.helpCenters(ctx); err != nil {
		return err
	}

	for page := pageCursor(opts.Cursor); ; page++ {
		var out list[collection]

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"250"}}
		if err := s.Client.get(ctx, "/help_center/collections", q, &out); err != nil {
			return err
		}

		recs := make([]format.Record, 0, len(out.Data))
		for _, c := range out.Data {
			hc := string(c.HelpCenterID)
			if hc == "" || hc == "0" {
				hc = s.defaultHC
			}

			recs = append(recs, translateCollection(c, hc))
		}

		last := len(out.Data) == 0 || page >= out.Pages.TotalPages
		next := ""

		if !last {
			next = strconv.Itoa(page + 1)
		}

		if err := emit(engine.Page{Records: recs, Cursor: next}); err != nil {
			return err
		}

		if last {
			return nil
		}
	}
}

/*
collectionCenters maps every collection to its help center, read once.
*/
func (s *Source) collectionCenters(ctx context.Context) (map[string]string, error) {
	s.mu.Lock()
	centers := s.centers
	s.mu.Unlock()

	if centers != nil {
		return centers, nil
	}

	if _, err := s.helpCenters(ctx); err != nil {
		return nil, err
	}

	centers = map[string]string{}

	for page := 1; page < 1000; page++ {
		var out list[collection]

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"250"}}
		if err := s.Client.get(ctx, "/help_center/collections", q, &out); err != nil {
			return nil, err
		}

		for _, c := range out.Data {
			centers[string(c.ID)] = string(c.HelpCenterID)
		}

		if len(out.Data) == 0 || page >= out.Pages.TotalPages {
			break
		}
	}

	s.mu.Lock()
	s.centers = centers
	s.mu.Unlock()

	return centers, nil
}

/*
readArticles reads public articles, then internal ones. The cursor is
the page, prefixed i: once on internal articles.
*/
func (s *Source) readArticles(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	centers, err := s.collectionCenters(ctx)
	if err != nil {
		return err
	}

	lang, err := s.defaultLocale(ctx)
	if err != nil {
		return err
	}

	cursor, internal := strings.CutPrefix(opts.Cursor, "i:")

	for page := pageCursor(cursor); !internal; page++ {
		var out list[article]

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"250"}}
		if err := s.Client.get(ctx, "/articles", q, &out); err != nil {
			return err
		}

		recs := make([]format.Record, 0, len(out.Data))
		for _, a := range out.Data {
			hc := centers[string(firstID(a.ParentID, firstIDOf(a.ParentIDs)))]
			if hc == "" || hc == "0" {
				hc = s.defaultHC
			}

			recs = append(recs, translateArticle(a, hc, lang))
		}

		last := len(out.Data) == 0 || page >= out.Pages.TotalPages
		next := strconv.Itoa(page + 1)

		if last {
			next = "i:1"
		}

		if err := emit(engine.Page{Records: recs, Cursor: next}); err != nil {
			return err
		}

		if last {
			internal, cursor = true, "1"
		}
	}

	for page := pageCursor(cursor); ; page++ {
		var out list[internalArticle]

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"50"}}

		err := s.Client.get(ctx, "/internal_articles", q, &out)
		if isNotFound(err) || isForbidden(err) {
			return nil
		}

		if err != nil {
			return err
		}

		recs := make([]format.Record, 0, len(out.Data))
		for _, a := range out.Data {
			recs = append(recs, translateInternalArticle(a, s.defaultHC))
		}

		last := len(out.Data) == 0 || page >= out.Pages.TotalPages
		next := ""

		if !last {
			next = "i:" + strconv.Itoa(page+1)
		}

		if err := emit(engine.Page{Records: recs, Cursor: next}); err != nil {
			return err
		}

		if last {
			return nil
		}
	}
}

func firstIDOf(ids IDs) ID {
	if len(ids) == 0 {
		return ""
	}

	return ids[0]
}

func (s *Source) readSnippets(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	if _, err := s.helpCenters(ctx); err != nil && !isForbidden(err) {
		return err
	}

	for page := pageCursor(opts.Cursor); ; page++ {
		var out list[snippet]

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"50"}}

		err := s.Client.get(ctx, "/content_snippets", q, &out)
		if isNotFound(err) {
			return nil
		}

		if err != nil {
			return err
		}

		recs := make([]format.Record, 0, len(out.Data))
		for _, sn := range out.Data {
			recs = append(recs, translateSnippet(sn, s.defaultHC))
		}

		last := len(out.Data) < 50 || (out.Pages.TotalPages > 0 && page >= out.Pages.TotalPages)
		next := ""

		if !last {
			next = strconv.Itoa(page + 1)
		}

		if err := emit(engine.Page{Records: recs, Cursor: next}); err != nil {
			return err
		}

		if last {
			return nil
		}
	}
}

func (s *Source) readMacros(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error {
	cursor := opts.Cursor

	for {
		q := url.Values{"per_page": {"100"}}
		if cursor != "" {
			q.Set("starting_after", cursor)
		}

		if !opts.Since.IsZero() {
			q.Set("updated_since", strconv.FormatInt(opts.Since.Unix(), 10))
		}

		var out list[macro]
		if err := s.Client.get(ctx, "/macros", q, &out); err != nil {
			return err
		}

		recs := make([]format.Record, 0, len(out.Data))
		for _, m := range out.Data {
			recs = append(recs, translateMacro(m))
		}

		next := nextCursor(out.Pages.Next)
		if err := emit(engine.Page{Records: recs, Cursor: next}); err != nil {
			return err
		}

		if next == "" || len(out.Data) == 0 {
			return nil
		}

		cursor = next
	}
}

func (s *Source) readSchedules(ctx context.Context, emit func(engine.Page) error) error {
	var out list[schedule]
	if err := s.Client.get(ctx, "/office_hours_schedules", nil, &out); err != nil {
		return err
	}

	var recs []format.Record

	for i, sch := range out.Data {
		var ex list[scheduleException]
		if err := s.Client.get(ctx, "/office_hours_schedules/"+url.PathEscape(string(sch.ID))+"/office_hours_exceptions", nil, &ex); err != nil && !isNotFound(err) {
			return err
		}

		recs = append(recs, translateSchedule(sch, ex.Data, i == 0))
	}

	return one(emit, recs)
}

/*
search reads one page of a search for what changed after since: every
conversation or ticket on a first run.
*/
func (s *Source) search(ctx context.Context, path string, since time.Time, cursor string, out any) error {
	value := int64(0)
	if !since.IsZero() {
		value = since.Unix()
	}

	body := map[string]any{
		"query":      map[string]any{"field": "updated_at", "operator": ">", "value": value},
		"pagination": pagination(cursor),
	}

	return s.Client.post(ctx, path, body, out)
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

		if err := s.search(ctx, "/conversations/search", opts.Since, cursor, &out); err != nil {
			return err
		}

		recs, err := parallel(ctx, s.Workers, out.Conversations, func(ctx context.Context, c conversation) ([]format.Record, error) {
			full, parts, err := s.conversation(ctx, c.ID)
			if isNotFound(err) {
				return nil, nil
			}

			if err != nil {
				return nil, err
			}

			rec, former := translateConversation(full, parts, known)

			return append(s.newFormer(former), rec), nil
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
		recs = append(recs, translateFormer(f))
	}

	return recs
}

/*
conversation reads a conversation and every one of its parts. Without
page, Intercom returns only the latest 500 parts; with it, parts come
oldest first, 500 a page, until a page is empty.
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

		if len(c.Parts.Parts) < 500 {
			break
		}
	}

	return first, parts, nil
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

		if err := s.search(ctx, "/tickets/search", opts.Since, cursor, &out); err != nil {
			return err
		}

		recs, err := parallel(ctx, s.Workers, out.Tickets, func(ctx context.Context, t ticket) ([]format.Record, error) {
			var full ticket

			err := s.Client.get(ctx, "/tickets/"+url.PathEscape(string(t.ID)), nil, &full)
			if isNotFound(err) {
				return nil, nil
			}

			if err != nil {
				return nil, err
			}

			rec, former := translateTicket(full, known)

			return append(s.newFormer(former), rec), nil
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
parallel runs fn over items with n at a time, keeping their order.
*/
func parallel[T any](ctx context.Context, n int, items []T, fn func(context.Context, T) ([]format.Record, error)) ([]format.Record, error) {
	if n <= 0 {
		n = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([][]format.Record, len(items))
	errs := make([]error, len(items))
	sem := make(chan struct{}, n)

	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}

		go func(i int, item T) {
			defer wg.Done()
			defer func() { <-sem }()

			results[i], errs[i] = fn(ctx, item)
			if errs[i] != nil {
				cancel()
			}
		}(i, item)
	}

	wg.Wait()

	var out []format.Record

	for i := range items {
		if errs[i] != nil && !errors.Is(errs[i], context.Canceled) {
			return nil, errs[i]
		}

		out = append(out, results[i]...)
	}

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}
