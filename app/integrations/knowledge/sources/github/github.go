/*
|--------------------------------------------------------------------------
| GitHub Repository Source
|--------------------------------------------------------------------------
|
| Markdown docs kept in a GitHub repository, read through the workspace's
| connected GitHub account at one branch. The repository and the branch
| are picked from what the account can reach, and the folders and files
| to read are ticked in the repository's tree; nothing ticked reads every
| Markdown file. The listing is the branch's file tree, one request
| however large, and a file's blob hash is its version, so only files
| that changed are read again. Front matter gives a file its title and
| can mark it a draft, and folders become collections.
|
*/

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
API is GitHub's REST API.
*/
const API = "https://api.github.com"

/*
repoPattern is an owner/name repository.
*/
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

/*
markdownPaths are the files a repository source reads.
*/
func markdownPaths() []string {
	return []string{"**/*.md", "**/*.mdx", "**/*.markdown"}
}

/*
skipped are the files never read.
*/
func skipped() []string {
	return []string{"node_modules/**", "vendor/**", ".github/**", "**/CHANGELOG.md", "**/LICENSE*"}
}

/*
schema is the settings form of a GitHub source.
*/
const schema = `{
  "type": "object",
  "required": ["installation", "repository"],
  "properties": {
    "installation": {"type": "string", "format": "x-integration", "title": "GitHub account",
      "description": "Connect GitHub to pick a repository."},
    "repository": {"type": "string", "format": "x-choice", "title": "Repository",
      "x-depends": ["installation"], "x-placeholder": "Choose a repository"},
    "branch": {"type": "string", "format": "x-choice", "title": "Branch",
      "x-depends": ["installation", "repository"], "x-placeholder": "Choose a branch"},
    "selected": {"type": "array", "items": {"type": "string"}, "format": "x-tree", "title": "Files",
      "description": "The folders and Markdown files to read. Every Markdown file when none are chosen."}
  }
}`

/*
settings are a GitHub source's settings.
*/
type settings struct {
	Repository string   `json:"repository"`
	Branch     string   `json:"branch"`
	Selected   []string `json:"selected"`
	Token      string   `json:"access_token"`
}

/*
Driver reads GitHub repositories over HTTP.
*/
type Driver struct {
	HTTP *httpclient.Client
}

/*
New returns the GitHub driver.
*/
func New(client *httpclient.Client) *Driver {
	return &Driver{HTTP: client}
}

/*
Describe says what the driver is.
*/
func (d *Driver) Describe() sources.Description {
	return sources.Description{
		Type: "github", Name: "GitHub", Logo: "github", Auth: sources.AuthOAuth, Integration: "github",
		Summary: "Markdown docs from a repository, synced from a branch or imported as articles.",
		Schema:  json.RawMessage(schema), Modes: []sources.Mode{sources.ModeSync, sources.ModeImport},
		Targets:   []sources.Target{sources.TargetAgent, sources.TargetOperator, sources.TargetHelpCenter},
		Schedules: []string{"daily", "hourly", "weekly", "manual"},
	}
}

/*
read decodes and checks the settings.
*/
func read(cfg sources.Config) (settings, error) {
	s, err := account(cfg)
	if err != nil {
		return s, err
	}

	if !repoPattern.MatchString(s.Repository) {
		return s, &sources.ConfigError{Field: "repository", Message: "Choose a repository."}
	}

	return s, nil
}

/*
account decodes the settings and checks only the connected account, as
listing repositories needs nothing else.
*/
func account(cfg sources.Config) (settings, error) {
	var s settings
	if err := cfg.Decode(&s); err != nil {
		return s, err
	}

	s.Repository, s.Branch = strings.TrimSpace(s.Repository), strings.TrimSpace(s.Branch)
	s.Token = strings.TrimSpace(s.Token)
	if s.Token == "" {
		return s, &sources.ConfigError{Field: "installation", Message: "Connect your GitHub account first."}
	}

	return s, nil
}

/*
Check reads the repository and settles its branch.
*/
func (d *Driver) Check(ctx context.Context, cfg sources.Config) (sources.CheckResult, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.CheckResult{}, err
	}

	var repo struct {
		FullName string `json:"full_name"`
	}

	if err := d.get(ctx, s, "/repos/"+s.Repository, &repo); err != nil {
		return sources.CheckResult{}, err
	}

	return sources.CheckResult{Account: repo.FullName}, nil
}

/*
branch is the branch the settings name, else the default one.
*/
func (d *Driver) branch(ctx context.Context, s settings) (string, error) {
	if s.Branch != "" {
		return s.Branch, nil
	}

	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}

	if err := d.get(ctx, s, "/repos/"+s.Repository, &repo); err != nil {
		return "", err
	}

	return repo.DefaultBranch, nil
}

/*
List returns every chosen Markdown file of the branch in one page.
*/
func (d *Driver) List(ctx context.Context, cfg sources.Config, _ string) (sources.Page, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Page{}, err
	}

	branch, files, err := d.files(ctx, s)
	if err != nil {
		return sources.Page{}, err
	}

	out := sources.Page{}
	for _, f := range files {
		if !sources.Chosen(s.Selected, lineage(f.Path)...) {
			continue
		}

		out.Refs = append(out.Refs, sources.Ref{
			ExternalID: f.Path, Path: f.Path, Version: f.SHA, Title: sources.Humanize(f.Path),
			URL: "https://github.com/" + s.Repository + "/blob/" + branch + "/" + f.Path,
		})
	}

	return out, nil
}

/*
file is one entry of a branch's tree.
*/
type file struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

/*
files reads the branch's tree and keeps its Markdown files, returning
the branch it read.
*/
func (d *Driver) files(ctx context.Context, s settings) (string, []file, error) {
	branch, err := d.branch(ctx, s)
	if err != nil {
		return "", nil, err
	}

	var tree struct {
		Tree []file `json:"tree"`
	}

	path := "/repos/" + s.Repository + "/git/trees/" + url.PathEscape(branch) + "?recursive=1"
	if err := d.get(ctx, s, path, &tree); err != nil {
		return "", nil, err
	}

	var out []file
	for _, f := range tree.Tree {
		if f.Type == "blob" && sources.Allowed(markdownPaths(), skipped(), f.Path) {
			out = append(out, f)
		}
	}

	return branch, out, nil
}

/*
Fetch reads one file at the branch.
*/
func (d *Driver) Fetch(ctx context.Context, cfg sources.Config, ref sources.Ref) (sources.Document, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Document{}, err
	}

	branch, err := d.branch(ctx, s)
	if err != nil {
		return sources.Document{}, err
	}

	path := "/repos/" + s.Repository + "/contents/" + escapePath(ref.Path) + "?ref=" + url.QueryEscape(branch)

	resp, err := d.request(s).Accept("application/vnd.github.raw+json").Get(ctx, API+path)
	if err != nil {
		return sources.Document{}, fmt.Errorf("github %s: %w", ref.Path, err)
	}

	if err := sources.FromResponse(resp, "github "+ref.Path); err != nil {
		return sources.Document{}, err
	}

	doc := sources.MarkdownFile(ref.Path, resp.Body)
	doc.Collections = sources.Folders(ref.Path)
	doc.URL, doc.Visibility = ref.URL, sources.VisibilityInternal

	return doc, nil
}

/*
escapePath escapes each segment of a file path.
*/
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}

	return strings.Join(parts, "/")
}

/*
request starts a GitHub request with the connected account's token.
*/
func (d *Driver) request(s settings) *httpclient.Request {
	return d.HTTP.New().WithHeader("X-GitHub-Api-Version", "2022-11-28").WithToken(s.Token)
}

/*
get sends one request to GitHub and decodes the answer.
*/
func (d *Driver) get(ctx context.Context, s settings, path string, out any) error {
	resp, err := d.request(s).Accept("application/vnd.github+json").Get(ctx, API+path)
	if err != nil {
		return fmt.Errorf("github %s: %w", path, err)
	}

	if err := sources.FromResponse(resp, "github "+path); err != nil {
		return err
	}

	return resp.JSON(out)
}
