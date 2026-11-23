/*
|--------------------------------------------------------------------------
| GitHub Tools
|--------------------------------------------------------------------------
|
| A routine that keeps the help center in step with the product reads
| the pull requests merged into the repositories it was given, through
| the workspace's GitHub connection. Only those repositories: a request
| for any other is refused, whatever a ticket or a pull request asks.
| Titles, descriptions, labels and diffs are written by outsiders, so
| they come back as untrusted text and taint the run. Lockfiles, images
| and vendored code are left out, and patches are cut short.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
The limits of the GitHub tools.
*/
const (
	pullsListed      = 50
	pullDaysNormal   = 7
	pullDaysMax      = 30
	patchPerFile     = 4 << 10
	patchPerPull     = 32 << 10
	pullBodyInList   = 600
	pullBodyInDetail = 6000
)

/*
GitHub reads pull requests through a workspace's GitHub connection.
*/
type GitHub interface {
	/*
		MergedPulls lists the pull requests merged as the query asks,
		newest first.
	*/
	MergedPulls(ctx context.Context, q PullQuery) ([]Pull, error)

	/*
		PullDetail reads one pull request with its changed files.
	*/
	PullDetail(ctx context.Context, installation ids.UUID, repo string, number int) (PullDetail, error)
}

/*
PullQuery asks for the pull requests merged into a repository's branch
since a moment, at most Limit, through a connection; an empty branch is
the repository's default.
*/
type PullQuery struct {
	Installation ids.UUID
	Repo, Branch string
	Since        time.Time
	Limit        int
}

/*
Pull is one merged pull request.
*/
type Pull struct {
	Number   int
	Title    string
	Body     string
	Author   string
	Labels   []string
	URL      string
	MergedAt time.Time
}

/*
PullDetail is one pull request with the files it changed.
*/
type PullDetail struct {
	Pull
	Files []PullFile
}

/*
PullFile is one file a pull request changed, with its patch.
*/
type PullFile struct {
	Name      string
	Status    string
	Additions int
	Deletions int
	Patch     string
}

/*
githubTools are the tools that read pull requests.
*/
func (e *Engine) githubTools() []tool {
	return []tool{
		{
			spec: contracts.ToolSpec{
				Name: "github_list_merged_pull_requests",
				Description: "List pull requests merged into one of this routine's repositories since a moment, " +
					"newest first, without those carrying an ignored label.",
				Schema: schema(`{"type":"object","required":["repository"],"properties":{` +
					`"repository":{"type":"string","description":"owner/repo, one of this routine's repositories."},` +
					`"merged_since":{"type":"string","description":"Only pull requests merged after this ISO 8601 ` +
					`time; 7 days ago by default, at most 30."}}}`),
			}, handle: e.listMergedPulls, routine: true, group: RoutineGroupGitHub,
			was: map[string]string{"since": "merged_since"},
		},
		{
			spec: contracts.ToolSpec{
				Name:        "github_get_pull_request_diff",
				Description: "Read one pull request's description and the files it changed, with shortened patches.",
				Schema: schema(`{"type":"object","required":["repository","pull_request_number"],"properties":{` +
					`"repository":{"type":"string","description":"owner/repo, one of this routine's repositories."},` +
					`"pull_request_number":{"type":"integer","minimum":1}}}`),
			}, handle: e.readPull, routine: true, group: RoutineGroupGitHub,
			was: map[string]string{"number": "pull_request_number"},
		},
	}
}

/*
repository checks a repository is one the routine may read.
*/
func (s *session) repository(name string) (*GitHubScope, string, error) {
	scope := s.routine.scope.GitHub
	name = strings.TrimSpace(name)
	allowed := func(r string) bool { return strings.EqualFold(r, name) }
	if scope == nil || !slices.ContainsFunc(scope.Repositories, allowed) {
		return nil, "", fmt.Errorf("%w: %q is not one of this routine's repositories", errBadArguments, name)
	}
	return scope, name, nil
}

/*
listMergedPulls lists the merged pull requests of a repository.
*/
func (e *Engine) listMergedPulls(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Repository string `json:"repository"`
		Since      string `json:"merged_since"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	scope, repo, err := s.repository(in.Repository)
	if err != nil {
		return result{}, err
	}
	now := e.Clock.Now()
	since := now.Add(-pullDaysNormal * 24 * time.Hour)
	if in.Since != "" {
		if since, err = time.Parse(time.RFC3339, in.Since); err != nil {
			return result{}, fmt.Errorf("%w: since is an ISO 8601 time", errBadArguments)
		}
	}
	since = later(since, now.Add(-pullDaysMax*24*time.Hour))
	pulls, err := e.Services.GitHub.MergedPulls(ctx, PullQuery{
		Installation: scope.InstallationID, Repo: repo, Branch: scope.BaseBranch, Since: since, Limit: pullsListed,
	})
	if err != nil {
		return result{}, err
	}
	s.tainted = true
	var b strings.Builder
	shown := 0
	for _, p := range pulls {
		if ignored(p.Labels, scope.IgnoreLabels) {
			continue
		}
		shown++
		fmt.Fprintf(&b, "- #%d · merged %s · by %s · %s\n  title: %s\n", p.Number,
			p.MergedAt.UTC().Format(time.RFC3339), p.Author, p.URL, untrusted(p.Title))
		if len(p.Labels) > 0 {
			fmt.Fprintf(&b, "  labels: %s\n", untrusted(strings.Join(p.Labels, ", ")))
		}
		if body := strings.TrimSpace(p.Body); body != "" {
			fmt.Fprintf(&b, "  body: %s\n", untrusted(truncate(body, pullBodyInList)))
		}
	}
	if shown == 0 {
		fmt.Fprintf(&b, "No pull requests were merged into %s since %s.", repo, since.UTC().Format(time.RFC3339))
	}
	return result{text: b.String(), step: Step{
		Tool:    "github_list_merged_pull_requests",
		Summary: fmt.Sprintf("Listed %d merged pull requests of %s", shown, repo),
	}}, nil
}

/*
readPull reads one pull request and its changed files.
*/
func (e *Engine) readPull(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Repository string `json:"repository"`
		Number     int    `json:"pull_request_number"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	scope, repo, err := s.repository(in.Repository)
	if err != nil {
		return result{}, err
	}
	if in.Number <= 0 {
		return result{}, fmt.Errorf("%w: give the pull request's number", errBadArguments)
	}
	pull, err := e.Services.GitHub.PullDetail(ctx, scope.InstallationID, repo, in.Number)
	if err != nil {
		return result{}, err
	}
	s.tainted = true
	var b strings.Builder
	fmt.Fprintf(&b, "Pull request %s#%d · %s · merged %s\ntitle: %s\n", repo, pull.Number, pull.URL,
		pull.MergedAt.UTC().Format(time.RFC3339), untrusted(pull.Title))
	if body := strings.TrimSpace(pull.Body); body != "" {
		fmt.Fprintf(&b, "description: %s\n", untrusted(truncate(body, pullBodyInDetail)))
	}
	b.WriteString("\nChanged files:\n")
	budget := patchPerPull
	for _, f := range pull.Files {
		fmt.Fprintf(&b, "- %s (%s, +%d -%d)", f.Name, f.Status, f.Additions, f.Deletions)
		if skippedFile(f.Name) || f.Patch == "" || budget <= 0 {
			b.WriteString("\n")
			continue
		}
		patch := truncateBytes(f.Patch, min(patchPerFile, budget))
		budget -= len(patch)
		fmt.Fprintf(&b, "\n%s\n", untrusted(patch))
	}
	return result{text: b.String(), step: Step{
		Tool: "github_get_pull_request_diff", Summary: fmt.Sprintf("Read %s#%d", repo, pull.Number),
	}}, nil
}

/*
ignored reports whether a pull request carries an ignored label.
*/
func ignored(labels, ignore []string) bool {
	for _, l := range labels {
		if slices.ContainsFunc(ignore, func(i string) bool { return strings.EqualFold(i, l) }) {
			return true
		}
	}
	return false
}

/*
skippedFile reports whether a file's patch says nothing about behavior:
lockfiles, images and vendored code.
*/
func skippedFile(name string) bool {
	base := strings.ToLower(path.Base(name))
	switch base {
	case "pnpm-lock.yaml", "package-lock.json", "yarn.lock", "go.sum", "cargo.lock", "composer.lock",
		"poetry.lock", "gemfile.lock":
		return true
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico", ".pdf", ".woff", ".woff2", ".snap":
		return true
	}
	return strings.HasPrefix(name, "vendor/") || strings.Contains(name, "/vendor/") ||
		strings.Contains(name, "node_modules/")
}

/*
truncateBytes keeps at most n bytes of a text, on a rune boundary.
*/
func truncateBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	cut := n
	for cut > 0 && !utf8Start(text[cut]) {
		cut--
	}
	return text[:cut] + "\n… (cut short)"
}

/*
utf8Start reports whether a byte starts a rune.
*/
func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

/*
later is the later of two moments.
*/
func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
