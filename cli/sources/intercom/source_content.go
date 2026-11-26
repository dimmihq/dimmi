package intercom

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

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

func (s *Source) readHelpCenters(ctx context.Context, _ engine.ReadOptions, emit func(engine.Page) error) error {
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
			recs = append(recs, translateCollection(c, s.centerOr(string(c.HelpCenterID))))
		}

		last := len(out.Data) == 0 || page >= int(out.Pages.TotalPages)
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
centerOr is the help center hc names, or the default one when it names
none.
*/
func (s *Source) centerOr(hc string) string {
	if hc == "" || hc == "0" {
		return s.defaultHC
	}

	return hc
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

		if len(out.Data) == 0 || page >= int(out.Pages.TotalPages) {
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

	if !internal {
		from := articlePages{from: pageCursor(cursor), centers: centers, lang: lang}
		if err := s.readPublicArticles(ctx, from, emit); err != nil {
			return err
		}

		cursor = "1"
	}

	return s.readInternalArticles(ctx, pageCursor(cursor), emit)
}

/*
articlePages is where a read of public articles starts, with the help
center of every collection and the workspace's language.
*/
type articlePages struct {
	from    int
	centers map[string]string
	lang    string
}

/*
readPublicArticles reads public articles from the page on. Its last page
carries the cursor of the first page of internal articles.
*/
func (s *Source) readPublicArticles(ctx context.Context, pages articlePages, emit func(engine.Page) error) error {
	for page := pages.from; ; page++ {
		var out list[article]

		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"250"}}
		if err := s.Client.get(ctx, "/articles", q, &out); err != nil {
			return err
		}

		recs := make([]format.Record, 0, len(out.Data))
		for _, a := range out.Data {
			hc := s.centerOr(pages.centers[string(firstID(a.ParentID, firstIDOf(a.ParentIDs)))])
			recs = append(recs, translateArticle(a, hc, pages.lang))
		}

		last := len(out.Data) == 0 || page >= int(out.Pages.TotalPages)

		next := strconv.Itoa(page + 1)
		if last {
			next = "i:1"
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
readInternalArticles reads internal articles from the page on, none when
the workspace has none or the token can't read them.
*/
func (s *Source) readInternalArticles(ctx context.Context, from int, emit func(engine.Page) error) error {
	for page := from; ; page++ {
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

		last := len(out.Data) == 0 || page >= int(out.Pages.TotalPages)
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

/*
lastSnippetPage reports whether a page of n snippets is the last: a
short page, or the last of the pages Intercom counted.
*/
func lastSnippetPage(n, page, totalPages int) bool {
	return n < 50 || (totalPages > 0 && page >= totalPages)
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

		last := lastSnippetPage(len(out.Data), page, int(max(out.TotalPages, out.Pages.TotalPages)))
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

func (s *Source) readSchedules(ctx context.Context, _ engine.ReadOptions, emit func(engine.Page) error) error {
	var out list[schedule]
	if err := s.Client.get(ctx, "/office_hours_schedules", nil, &out); err != nil {
		return err
	}

	var recs []format.Record

	for i, sch := range out.Data {
		var ex list[scheduleException]
		path := "/office_hours_schedules/" + url.PathEscape(string(sch.ID)) + "/office_hours_exceptions"
		if err := s.Client.get(ctx, path, nil, &ex); err != nil && !isNotFound(err) {
			return err
		}

		recs = append(recs, translateSchedule(sch, ex.Data, i == 0))
	}

	return one(emit, recs)
}
