/*
|--------------------------------------------------------------------------
| Zendesk Guide Source
|--------------------------------------------------------------------------
|
| A Zendesk help center, read through the workspace's connected Zendesk
| account, whose subdomain is the account it was connected as; Zendesk
| is retiring API tokens, so the driver speaks OAuth only. Each article
| is one document per language: the source language under the
| article's id, each translation under the id and its locale.
| Its section and category become its collections, its body becomes
| Markdown with its pictures collected, a draft stays a draft, and an
| article limited to signed-in users is kept from customers. Listing
| walks every locale the help center has, with Zendesk's cursor pages.
|
*/

package zendesk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
pageSize is how many articles one listing page asks for.
*/
const pageSize = 100

/*
subdomainPattern is a Zendesk subdomain.
*/
var subdomainPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

/*
schema is the settings form of a Zendesk source.
*/
const schema = `{
  "type": "object",
  "required": ["installation"],
  "properties": {
    "installation": {"type": "string", "format": "x-integration", "title": "Zendesk account",
      "description": "Connect your Zendesk with its subdomain, the part before .zendesk.com.",
      "x-account": "subdomain"},
    "selected": {"type": "array", "items": {"type": "string"}, "format": "x-tree", "title": "Articles",
      "description": "The categories, sections and articles to read. Everything when none are chosen."}
  }
}`

/*
settings are a Zendesk source's settings.
*/
type settings struct {
	Subdomain string   `json:"account"`
	Token     string   `json:"access_token"`
	Selected  []string `json:"selected"`
}

/*
base is the help center API's root.
*/
func (s settings) base() string {
	return "https://" + s.Subdomain + ".zendesk.com/api/v2/help_center"
}

/*
Driver reads Zendesk help centers over HTTP.
*/
type Driver struct {
	HTTP *httpclient.Client
}

/*
New returns the Zendesk driver.
*/
func New(client *httpclient.Client) *Driver {
	return &Driver{HTTP: client}
}

/*
Describe says what the driver is.
*/
func (d *Driver) Describe() sources.Description {
	return sources.Description{
		Type: "zendesk_help_center", Name: "Zendesk", Logo: "zendesk", Auth: sources.AuthOAuth, Integration: "zendesk",
		Summary: "Import your Zendesk Guide articles with their sections, pictures and translations.",
		Schema:  json.RawMessage(schema), Modes: []sources.Mode{sources.ModeImport, sources.ModeSync},
		Targets:   []sources.Target{sources.TargetHelpCenter, sources.TargetAgent, sources.TargetOperator},
		Schedules: []string{"manual", "daily", "weekly"},
	}
}

/*
read decodes and checks the settings.
*/
func read(cfg sources.Config) (settings, error) {
	var s settings
	if err := cfg.Decode(&s); err != nil {
		return s, err
	}

	s.Subdomain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s.Subdomain)), ".zendesk.com")
	s.Subdomain = strings.TrimPrefix(strings.TrimPrefix(s.Subdomain, "https://"), "http://")

	if !subdomainPattern.MatchString(s.Subdomain) {
		return s, &sources.ConfigError{Field: "installation", Message: "Connect your Zendesk account first."}
	}

	if s.Token = strings.TrimSpace(s.Token); s.Token == "" {
		return s, &sources.ConfigError{Field: "installation", Message: "Connect your Zendesk account first."}
	}

	return s, nil
}

/*
Check reads the help center's languages.
*/
func (d *Driver) Check(ctx context.Context, cfg sources.Config) (sources.CheckResult, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.CheckResult{}, err
	}

	if _, err := d.locales(ctx, s); err != nil {
		return sources.CheckResult{}, err
	}

	var list struct {
		Count int `json:"count"`
	}

	if err := d.get(ctx, s, s.base()+"/articles.json?per_page=1", &list); err != nil {
		return sources.CheckResult{}, err
	}

	return sources.CheckResult{Account: s.Subdomain + ".zendesk.com", Estimate: list.Count}, nil
}

/*
locales lists the help center's languages, the default first.
*/
func (d *Driver) locales(ctx context.Context, s settings) ([]string, error) {
	var out struct {
		Locales       []string `json:"locales"`
		DefaultLocale string   `json:"default_locale"`
	}

	if err := d.get(ctx, s, s.base()+"/locales.json", &out); err != nil {
		return nil, err
	}

	ordered := []string{out.DefaultLocale}
	for _, l := range out.Locales {
		if l != out.DefaultLocale {
			ordered = append(ordered, l)
		}
	}

	return ordered, nil
}

/*
article is a Zendesk article in one language.
*/
type article struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	HTMLURL       string    `json:"html_url"`
	Locale        string    `json:"locale"`
	SourceLocale  string    `json:"source_locale"`
	SectionID     int64     `json:"section_id"`
	Draft         bool      `json:"draft"`
	UserSegmentID *int64    `json:"user_segment_id"`
	UpdatedAt     time.Time `json:"updated_at"`
}

/*
cursor is where a listing stands: the locale being walked and Zendesk's
own cursor inside it.
*/
type cursor struct {
	Locale int    `json:"l"`
	After  string `json:"a"`
}

/*
List returns one page of one locale's articles.
*/
func (d *Driver) List(ctx context.Context, cfg sources.Config, raw string) (sources.Page, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Page{}, err
	}

	locales, err := d.locales(ctx, s)
	if err != nil {
		return sources.Page{}, err
	}

	var at cursor
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &at)
	}

	if at.Locale >= len(locales) {
		return sources.Page{}, nil
	}

	page, err := d.articles(ctx, s, locales[at.Locale], at.After)
	if err != nil {
		return sources.Page{}, err
	}

	chosen, err := d.chooser(ctx, s)
	if err != nil {
		return sources.Page{}, err
	}

	out := sources.Page{}
	for _, a := range page.Articles {
		if chosen(a) {
			out.Refs = append(out.Refs, refOf(a))
		}
	}

	next := cursor{Locale: at.Locale + 1}
	if page.Meta.HasMore {
		next = cursor{Locale: at.Locale, After: page.Meta.AfterCursor}
	}

	if next.Locale < len(locales) {
		encoded, _ := json.Marshal(next)
		out.Next = string(encoded)
	}

	return out, nil
}

/*
articlePage is one page of a locale's articles and Zendesk's cursor of
the next.
*/
type articlePage struct {
	Articles []article `json:"articles"`
	Meta     struct {
		HasMore     bool   `json:"has_more"`
		AfterCursor string `json:"after_cursor"`
	} `json:"meta"`
}

/*
articles reads one page of a locale's articles.
*/
func (d *Driver) articles(ctx context.Context, s settings, locale, after string) (articlePage, error) {
	address := fmt.Sprintf("%s/%s/articles.json?page[size]=%d", s.base(), url.PathEscape(locale), pageSize)
	if after != "" {
		address += "&page[after]=" + url.QueryEscape(after)
	}

	var page articlePage
	if err := d.get(ctx, s, address, &page); err != nil {
		return page, err
	}

	return page, nil
}

/*
refOf names one language of an article as a document.
*/
func refOf(a article) sources.Ref {
	id := fmt.Sprint(a.ID)
	if a.Locale != a.SourceLocale {
		id += ":" + a.Locale
	}

	return sources.Ref{
		ExternalID: id, URL: a.HTMLURL, Title: a.Title, Path: a.Title,
		Version: a.UpdatedAt.UTC().Format(time.RFC3339), UpdatedAt: a.UpdatedAt,
	}
}

/*
Fetch reads one language of one article with its section and category.
*/
func (d *Driver) Fetch(ctx context.Context, cfg sources.Config, ref sources.Ref) (sources.Document, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Document{}, err
	}

	id, locale, translated := strings.Cut(ref.ExternalID, ":")

	path := s.base() + "/articles/" + url.PathEscape(id) + ".json"
	if translated {
		path = s.base() + "/" + url.PathEscape(locale) + "/articles/" + url.PathEscape(id) + ".json"
	}

	var one struct {
		Article article `json:"article"`
	}

	if err := d.get(ctx, s, path, &one); err != nil {
		return sources.Document{}, err
	}

	a := one.Article

	md, assets, err := sources.HTMLToMarkdown(a.Body)
	if err != nil {
		return sources.Document{}, fmt.Errorf("convert article %s: %w", id, err)
	}

	collections, err := d.collections(ctx, s, a)
	if err != nil {
		return sources.Document{}, err
	}

	doc := sources.Document{
		Title: a.Title, Markdown: md, URL: a.HTMLURL, Locale: a.Locale, Assets: assets, Collections: collections,
		Draft: a.Draft, Visibility: sources.VisibilityPublic, UpdatedAt: a.UpdatedAt,
	}

	if a.UserSegmentID != nil {
		doc.Visibility = sources.VisibilityInternal
	}

	if translated {
		doc.TranslationOf = id
	}

	return doc, nil
}

/*
get sends one request to Zendesk and decodes the answer.
*/
func (d *Driver) get(ctx context.Context, s settings, address string, out any) error {
	resp, err := d.HTTP.New().WithToken(s.Token).Accept("application/json").Get(ctx, address)
	if err != nil {
		return fmt.Errorf("zendesk: %w", err)
	}

	if err := sources.FromResponse(resp, "zendesk"); err != nil {
		return err
	}

	return resp.JSON(out)
}
