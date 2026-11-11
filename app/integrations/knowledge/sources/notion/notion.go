/*
|--------------------------------------------------------------------------
| Notion Source
|--------------------------------------------------------------------------
|
| A Notion workspace's pages, read through the workspace's connected
| Notion account. Notion shows the connection only the pages the person
| shared while connecting; the member then ticks, in a tree of those
| pages, the ones to read, each with the pages under it. A page's last edit time is its version, so an unchanged page is
| never read twice. Its blocks become Markdown, and the pages above it
| become its collections, ready for an import.
|
*/

package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
API is where Notion's API lives, and the version this driver speaks.
*/
const (
	API        = "https://api.notion.com/v1"
	apiVersion = "2022-06-28"
	pageSize   = 100
	maxPages   = 5000
)

/*
schema is the settings form of a Notion source.
*/
const schema = `{
  "type": "object",
  "required": ["installation"],
  "properties": {
    "installation": {"type": "string", "format": "x-integration", "title": "Notion workspace",
      "description": "Connect Notion and pick the pages dimmi may read."},
    "selected": {"type": "array", "items": {"type": "string"}, "format": "x-tree", "title": "Pages",
      "description": "The pages to read, with the pages under them. Everything shared when none are chosen."}
  }
}`

/*
settings are a Notion source's settings.
*/
type settings struct {
	Token    string   `json:"access_token"`
	Selected []string `json:"selected"`
}

/*
Driver reads Notion workspaces over HTTP.
*/
type Driver struct {
	HTTP *httpclient.Client
}

/*
New returns the Notion driver.
*/
func New(client *httpclient.Client) *Driver {
	return &Driver{HTTP: client}
}

/*
Describe says what the driver is.
*/
func (d *Driver) Describe() sources.Description {
	return sources.Description{
		Type: "notion", Name: "Notion", Logo: "notion", Auth: sources.AuthOAuth, Integration: "notion",
		Summary: "Pages from your Notion workspace, kept in sync or imported as articles.",
		Schema:  json.RawMessage(schema), Modes: []sources.Mode{sources.ModeSync, sources.ModeImport},
		Targets:   []sources.Target{sources.TargetAgent, sources.TargetOperator, sources.TargetHelpCenter},
		Schedules: []string{"daily", "weekly", "manual"},
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
		return s, &sources.ConfigError{Field: "installation", Message: "Connect Notion first."}
	}

	for i, id := range s.Selected {
		s.Selected[i] = PageID(id)
	}

	return s, nil
}

/*
Check reads the connection's own bot user.
*/
func (d *Driver) Check(ctx context.Context, cfg sources.Config) (sources.CheckResult, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.CheckResult{}, err
	}

	var me struct {
		Bot struct {
			WorkspaceName string `json:"workspace_name"`
		} `json:"bot"`
	}

	if err := d.call(ctx, s, request{method: "GET", path: "/users/me", out: &me}); err != nil {
		return sources.CheckResult{}, err
	}

	return sources.CheckResult{Account: me.Bot.WorkspaceName}, nil
}

/*
List returns every chosen page in one listing.
*/
func (d *Driver) List(ctx context.Context, cfg sources.Config, _ string) (sources.Page, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Page{}, err
	}

	all, err := d.pages(ctx, s)
	if err != nil {
		return sources.Page{}, err
	}

	parents := parentsOf(all)
	result := sources.Page{}

	for _, p := range all {
		if !sources.Chosen(s.Selected, lineage(parents, PageID(p.ID))...) {
			continue
		}

		result.Refs = append(result.Refs, sources.Ref{
			ExternalID: p.ID, URL: p.URL, Title: p.title(), Path: p.title(),
			Version: p.LastEdited.UTC().Format(time.RFC3339), UpdatedAt: p.LastEdited,
		})
	}

	return result, nil
}

/*
pages reads every page shared with the connection, up to the most a
source holds.
*/
func (d *Driver) pages(ctx context.Context, s settings) ([]page, error) {
	var all []page

	cursor := ""

	for len(all) < maxPages {
		filter := map[string]string{"property": "object", "value": "page"}
		body := map[string]any{"filter": filter, "page_size": pageSize}
		if cursor != "" {
			body["start_cursor"] = cursor
		}

		var out struct {
			Results    []page `json:"results"`
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		}

		if err := d.call(ctx, s, request{method: "POST", path: "/search", body: body, out: &out}); err != nil {
			return nil, err
		}

		for _, p := range out.Results {
			if !p.Archived && !p.InTrash {
				all = append(all, p)
			}
		}

		if !out.HasMore {
			break
		}

		cursor = out.NextCursor
	}

	return all, nil
}

/*
Fetch reads one page with its blocks.
*/
func (d *Driver) Fetch(ctx context.Context, cfg sources.Config, ref sources.Ref) (sources.Document, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Document{}, err
	}

	p, err := d.page(ctx, s, ref.ExternalID)
	if err != nil {
		return sources.Document{}, err
	}

	if p.Archived || p.InTrash {
		return sources.Document{}, sources.ErrNotFound
	}

	collections, err := d.ancestors(ctx, s, p)
	if err != nil {
		return sources.Document{}, err
	}

	md, assets, err := d.blocks(ctx, s, p.ID, 0)
	if err != nil {
		return sources.Document{}, err
	}

	if strings.TrimSpace(md) == "" {
		return sources.Document{}, sources.Excluded("empty")
	}

	doc := sources.Document{
		Title: p.title(), Markdown: shiftHeadings(sources.Tidy(md)), URL: p.URL, Path: pathOf(collections, p.title()),
		Collections: collections, Assets: assets, Visibility: sources.VisibilityInternal, UpdatedAt: p.LastEdited,
		Version: p.LastEdited.UTC().Format(time.RFC3339),
	}

	if p.PublicURL != "" {
		doc.URL, doc.Visibility = p.PublicURL, sources.VisibilityPublic
	}

	return doc, nil
}

/*
ancestors walks up from the page to the workspace, returning the pages
above it as collections, outermost first.
*/
func (d *Driver) ancestors(ctx context.Context, s settings, p page) ([]sources.Collection, error) {
	var out []sources.Collection

	parent := p.Parent

	for depth := 0; depth < 6 && parent.Type == "page_id"; depth++ {
		up, err := d.page(ctx, s, parent.PageID)
		if err != nil {
			if errorsIsNotFound(err) {
				break
			}

			return nil, err
		}

		out = append([]sources.Collection{{ExternalID: up.ID, Name: up.title(), URL: up.URL}}, out...)
		parent = up.Parent
	}

	return out, nil
}

/*
page reads one page.
*/
func (d *Driver) page(ctx context.Context, s settings, id string) (page, error) {
	var p page
	if err := d.call(ctx, s, request{method: "GET", path: "/pages/" + url.PathEscape(id), out: &p}); err != nil {
		return p, err
	}

	return p, nil
}

/*
request is one call to Notion: its method, path, body and where its
answer is decoded.
*/
type request struct {
	method string
	path   string
	body   any
	out    any
}

/*
call sends one request to Notion and decodes the answer.
*/
func (d *Driver) call(ctx context.Context, s settings, r request) error {
	method, path, body, out := r.method, r.path, r.body, r.out

	req := d.HTTP.New().WithToken(s.Token).WithHeader("Notion-Version", apiVersion).Accept("application/json")

	var (
		resp *httpclient.Response
		err  error
	)

	if method == "POST" {
		resp, err = req.WithHeader("Content-Type", "application/json").Post(ctx, API+path, body)
	} else {
		resp, err = req.Get(ctx, API+path)
	}

	if err != nil {
		return fmt.Errorf("notion %s: %w", path, err)
	}

	if err := sources.FromResponse(resp, "notion "+path); err != nil {
		return err
	}

	return resp.JSON(out)
}

/*
pathOf is where a page sits, as patterns see it: its ancestors' titles
and its own, joined by slashes.
*/
func pathOf(collections []sources.Collection, title string) string {
	parts := make([]string, 0, len(collections)+1)
	for _, c := range collections {
		parts = append(parts, c.Name)
	}

	return strings.Join(append(parts, title), "/")
}

/*
PageID reads a page id out of an id or a page link, without dashes,
lowercase.
*/
func PageID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	if u, err := url.Parse(s); err == nil && u.Host != "" {
		s = u.Path
	}

	s = strings.ReplaceAll(strings.ToLower(s), "-", "")
	if len(s) >= 32 {
		s = s[len(s)-32:]
	}

	return s
}
