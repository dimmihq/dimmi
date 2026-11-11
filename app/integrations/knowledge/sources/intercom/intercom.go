/*
|--------------------------------------------------------------------------
| Intercom Help Center Source
|--------------------------------------------------------------------------
|
| An Intercom workspace's help center, read through the workspace's
| connected Intercom account. The member ticks, in a tree of its
| collections and articles, what to read. Every article is one document per language:
| the default language under the article's id, each translation under
| the id and its locale, so a translation lands in the help center of
| its language. Its collections become the article's place, its body
| becomes Markdown with its pictures collected, a draft stays a draft,
| and its public address is kept so the import can redirect it. The
| region of the workspace picks the API's host.
|
*/

package intercom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
apiVersion is the Intercom API version the driver speaks, and perPage
the articles one listing page asks for.
*/
const (
	apiVersion = "2.11"
	perPage    = 50
)

/*
host is the API host of an Intercom region, empty for an unknown one.
*/
func host(region string) string {
	switch region {
	case "us":
		return "https://api.intercom.io"
	case "eu":
		return "https://api.eu.intercom.io"
	case "au":
		return "https://api.au.intercom.io"
	}

	return ""
}

/*
schema is the settings form of an Intercom source.
*/
const schema = `{
  "type": "object",
  "required": ["installation"],
  "properties": {
    "installation": {"type": "string", "format": "x-integration", "title": "Intercom workspace",
      "description": "Connect Intercom to read its help center."},
    "region": {"type": "string", "title": "Data region", "enum": ["us", "eu", "au"], "default": "us",
      "x-enum-labels": ["United States", "Europe", "Australia"]},
    "selected": {"type": "array", "items": {"type": "string"}, "format": "x-tree", "title": "Articles",
      "description": "The collections and articles to read. Everything when none are chosen."}
  }
}`

/*
settings are an Intercom source's settings.
*/
type settings struct {
	Token    string   `json:"access_token"`
	Region   string   `json:"region"`
	Selected []string `json:"selected"`
}

/*
Driver reads Intercom help centers over HTTP.
*/
type Driver struct {
	HTTP *httpclient.Client
}

/*
New returns the Intercom driver.
*/
func New(client *httpclient.Client) *Driver {
	return &Driver{HTTP: client}
}

/*
Describe says what the driver is.
*/
func (d *Driver) Describe() sources.Description {
	return sources.Description{
		Type: "intercom_help_center", Name: "Intercom", Logo: "intercom", Auth: sources.AuthOAuth,
		Integration: "intercom",
		Summary:     "Import your Intercom help center with its collections, pictures and translations.",
		Schema:      json.RawMessage(schema), Modes: []sources.Mode{sources.ModeImport, sources.ModeSync},
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

	s.Token = strings.TrimSpace(s.Token)
	if s.Token == "" {
		return s, &sources.ConfigError{Field: "installation", Message: "Connect Intercom first."}
	}

	if host(s.Region) == "" {
		s.Region = "us"
	}

	return s, nil
}

/*
Check reads the app the token belongs to.
*/
func (d *Driver) Check(ctx context.Context, cfg sources.Config) (sources.CheckResult, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.CheckResult{}, err
	}

	var me struct {
		App struct {
			Name string `json:"name"`
		} `json:"app"`
	}

	if err := d.get(ctx, s, "/me", &me); err != nil {
		return sources.CheckResult{}, err
	}

	var list listing
	if err := d.get(ctx, s, "/articles?per_page=1", &list); err != nil {
		return sources.CheckResult{}, err
	}

	return sources.CheckResult{Account: me.App.Name, Estimate: list.TotalCount}, nil
}

/*
article is an Intercom article as the API returns it.
*/
type article struct {
	ID                string                     `json:"id"`
	Title             string                     `json:"title"`
	Description       string                     `json:"description"`
	Body              string                     `json:"body"`
	State             string                     `json:"state"`
	URL               string                     `json:"url"`
	ParentID          json.Number                `json:"parent_id"`
	ParentIDs         []json.Number              `json:"parent_ids"`
	ParentType        string                     `json:"parent_type"`
	DefaultLocale     string                     `json:"default_locale"`
	UpdatedAt         int64                      `json:"updated_at"`
	TranslatedContent map[string]json.RawMessage `json:"translated_content"`
}

/*
translation is one language of an article.
*/
type translation struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Body        string `json:"body"`
	State       string `json:"state"`
	URL         string `json:"url"`
	UpdatedAt   int64  `json:"updated_at"`
}

/*
language is one language of an article: its content, its locale and
whether it is a translation.
*/
type language struct {
	content    translation
	locale     string
	translated bool
}

/*
listing is one page of articles.
*/
type listing struct {
	Data       []article `json:"data"`
	TotalCount int       `json:"total_count"`
	Pages      struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"pages"`
}

/*
List returns one page of articles, one document per language.
*/
func (d *Driver) List(ctx context.Context, cfg sources.Config, cursor string) (sources.Page, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Page{}, err
	}

	page := 1
	if n, err := strconv.Atoi(cursor); err == nil && n > 1 {
		page = n
	}

	var list listing
	if err := d.get(ctx, s, fmt.Sprintf("/articles?page=%d&per_page=%d", page, perPage), &list); err != nil {
		return sources.Page{}, err
	}

	out := sources.Page{}
	for _, a := range list.Data {
		if sources.Chosen(s.Selected, lineageOf(a)...) {
			out.Refs = append(out.Refs, refsOf(a)...)
		}
	}

	if list.Pages.Page < list.Pages.TotalPages {
		out.Next = strconv.Itoa(page + 1)
	}

	return out, nil
}

/*
refsOf names an article's languages as documents.
*/
func refsOf(a article) []sources.Ref {
	refs := []sources.Ref{{
		ExternalID: a.ID, URL: a.URL, Title: a.Title, Path: a.Title,
		Version: strconv.FormatInt(a.UpdatedAt, 10), UpdatedAt: time.Unix(a.UpdatedAt, 0),
	}}

	for locale, raw := range a.TranslatedContent {
		var t translation
		if locale == "type" || locale == a.DefaultLocale || json.Unmarshal(raw, &t) != nil || t.Title == "" {
			continue
		}

		refs = append(refs, sources.Ref{
			ExternalID: a.ID + ":" + locale, URL: t.URL, Title: t.Title, Path: t.Title,
			Version: strconv.FormatInt(t.UpdatedAt, 10), UpdatedAt: time.Unix(t.UpdatedAt, 0),
		})
	}

	return refs
}

/*
Fetch reads one language of one article.
*/
func (d *Driver) Fetch(ctx context.Context, cfg sources.Config, ref sources.Ref) (sources.Document, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Document{}, err
	}

	id, locale, translated := strings.Cut(ref.ExternalID, ":")

	var a article
	if err := d.get(ctx, s, "/articles/"+url.PathEscape(id), &a); err != nil {
		return sources.Document{}, err
	}

	t := translation{
		Title: a.Title, Description: a.Description, Body: a.Body, State: a.State, URL: a.URL, UpdatedAt: a.UpdatedAt,
	}
	if !translated {
		locale = a.DefaultLocale
	}

	if translated {
		raw, ok := a.TranslatedContent[locale]
		if !ok || json.Unmarshal(raw, &t) != nil || t.Title == "" {
			return sources.Document{}, sources.ErrNotFound
		}
	}

	return d.document(ctx, s, a, language{content: t, locale: locale, translated: translated})
}

/*
document turns one language of an article into a document.
*/
func (d *Driver) document(ctx context.Context, s settings, a article, l language) (sources.Document, error) {
	t, locale := l.content, l.locale

	md, assets, err := sources.HTMLToMarkdown(t.Body)
	if err != nil {
		return sources.Document{}, fmt.Errorf("convert article %s: %w", a.ID, err)
	}

	collections, err := d.collections(ctx, s, a, locale)
	if err != nil {
		return sources.Document{}, err
	}

	doc := sources.Document{
		Title: t.Title, Description: t.Description, Markdown: md, URL: t.URL, Locale: locale, Assets: assets,
		Collections: collections, Draft: t.State != "published", Visibility: sources.VisibilityPublic,
		UpdatedAt: time.Unix(t.UpdatedAt, 0),
	}

	if l.translated {
		doc.TranslationOf = a.ID
	}

	return doc, nil
}

/*
get sends one request to Intercom and decodes the answer.
*/
func (d *Driver) get(ctx context.Context, s settings, path string, out any) error {
	resp, err := d.HTTP.New().WithToken(s.Token).WithHeader("Intercom-Version", apiVersion).
		Accept("application/json").Get(ctx, host(s.Region)+path)
	if err != nil {
		return fmt.Errorf("intercom %s: %w", path, err)
	}

	if err := sources.FromResponse(resp, "intercom "+path); err != nil {
		return err
	}

	return resp.JSON(out)
}
