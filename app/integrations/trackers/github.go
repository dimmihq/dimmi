/*
|--------------------------------------------------------------------------
| GitHub Issues
|--------------------------------------------------------------------------
|
| GitHub Issues through the GitHub account a workspace connected: a
| repository plays the part of a team, and an issue is named owner/repo
| and its number, which is also what its URL carries. An open issue with
| someone assigned has started; a closed one is done, or canceled when
| closed as not planned. GitHub has no fields to write the impact to, so
| it lives in one comment dimmi keeps editing, found again by a marker.
|
| The repositories dimmi links get a webhook for issues, comments and
| deployment statuses, signed with the install's GitHub webhook secret.
| A successful deployment is how dimmi knows a fix reached production.
|
*/

package trackers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
GitHubAPI is GitHub's REST API.
*/
const GitHubAPI = "https://api.github.com"

/*
impactMarker marks the comment dimmi keeps the impact in.
*/
const impactMarker = "<!-- dimmi-impact -->"

/*
githubRef matches owner/repo#123 or an issue URL.
*/
var githubRef = regexp.MustCompile(`([\w.-]+)/([\w.-]+)(?:#|/issues/)(\d+)`)

/*
GitHub is the GitHub Issues tracker.
*/
type GitHub struct {
	c client
}

/*
NewGitHub returns the GitHub tracker calling through the client.
*/
func NewGitHub(h *httpclient.Client) *GitHub {
	return &GitHub{c: client{http: h, header: map[string]string{"X-GitHub-Api-Version": "2022-11-28"}}}
}

/*
Provider names GitHub.
*/
func (*GitHub) Provider() string { return "github" }

/*
Incidents reports false: GitHub holds issues.
*/
func (*GitHub) Incidents() bool { return false }

/*
githubIssue is an issue as the REST API returns it.
*/
type githubIssue struct {
	Number      int64  `json:"number"`
	Title       string `json:"title"`
	HTMLURL     string `json:"html_url"`
	State       string `json:"state"`
	StateReason string `json:"state_reason"`
	Assignees   []any  `json:"assignees"`
	PullRequest any    `json:"pull_request"`
}

/*
issue converts the issue of the repository.
*/
func (g githubIssue) issue(repo string) Issue {
	group, status := GroupUnstarted, "Open"

	switch {
	case g.State == "closed" && g.StateReason == "not_planned":
		group, status = GroupCanceled, "Closed as not planned"
	case g.State == "closed":
		group, status = GroupDone, "Closed"
	case len(g.Assignees) > 0:
		group, status = GroupStarted, "Open, assigned"
	}

	key := repo + "#" + strconv.FormatInt(g.Number, 10)

	return Issue{
		ExternalID: key, Key: key, URL: g.HTMLURL, Title: g.Title, Status: status, StatusID: group,
		StatusGroup: group, TeamID: repo, TeamName: repo,
	}
}

/*
call sends one request to the API path.
*/
func (g *GitHub) call(ctx context.Context, c Call, r request) error {
	r.url = GitHubAPI + r.url

	return g.c.do(ctx, map[string]string{"Authorization": "Bearer " + c.Token}, r)
}

/*
Options returns repositories, a repository's environments, or the
three states an issue can be in.
*/
func (g *GitHub) Options(ctx context.Context, c Call, list, parent string) ([]Option, error) {
	switch list {
	case ListTeams:
		var out []struct {
			FullName string `json:"full_name"`
			Private  bool   `json:"private"`
		}

		if err := g.call(ctx, c, get("/user/repos?per_page=100&sort=updated", &out)); err != nil {
			return nil, err
		}

		options := make([]Option, len(out))
		for i, r := range out {
			options[i] = Option{ID: r.FullName, Name: r.FullName}
		}

		return options, nil
	case ListEnvironments:
		var out struct {
			Environments []struct {
				Name string `json:"name"`
			} `json:"environments"`
		}

		if err := g.call(ctx, c, get("/repos/"+parent+"/environments", &out)); err != nil {
			return nil, err
		}

		options := make([]Option, len(out.Environments))
		for i, e := range out.Environments {
			options[i] = Option{ID: e.Name, Name: e.Name}
		}

		return options, nil
	case ListStates:
		return []Option{
			{ID: GroupUnstarted, Name: "Open", Group: GroupUnstarted},
			{ID: GroupStarted, Name: "Open, assigned", Group: GroupStarted},
			{ID: GroupDone, Name: "Closed", Group: GroupDone},
			{ID: GroupCanceled, Name: "Closed as not planned", Group: GroupCanceled},
		}, nil
	}

	return nil, ErrUnsupported
}

/*
Search finds issues by the words, or the issue a reference names.
*/
func (g *GitHub) Search(ctx context.Context, c Call, q string) ([]Issue, error) {
	if githubRef.MatchString(q) {
		is, err := g.Read(ctx, c, q)
		if err == nil {
			return []Issue{is}, nil
		}
	}

	var out struct {
		Items []struct {
			githubIssue

			RepositoryURL string `json:"repository_url"`
		} `json:"items"`
	}

	address := "/search/issues?per_page=20&q=" + url.QueryEscape(q+" is:issue")
	if err := g.call(ctx, c, get(address, &out)); err != nil {
		return nil, err
	}

	issues := make([]Issue, len(out.Items))
	for i, it := range out.Items {
		issues[i] = it.issue(strings.TrimPrefix(it.RepositoryURL, GitHubAPI+"/repos/"))
	}

	return issues, nil
}

/*
Read returns the issue owner/repo#123 or its URL names.
*/
func (g *GitHub) Read(ctx context.Context, c Call, ref string) (Issue, error) {
	m := githubRef.FindStringSubmatch(ref)
	if m == nil {
		return Issue{}, ErrNotFound
	}

	repo := m[1] + "/" + m[2]

	var out githubIssue
	if err := g.call(ctx, c, get("/repos/"+repo+"/issues/"+m[3], &out)); err != nil {
		return Issue{}, err
	}

	return out.issue(repo), nil
}

/*
Create opens the issue in the repository and comments its impact.
*/
func (g *GitHub) Create(ctx context.Context, c Call, in CreateInput) (Issue, error) {
	var out githubIssue

	err := g.call(ctx, c, request{
		method: fhttp.MethodPost, url: "/repos/" + in.Team + "/issues", out: &out,
		body: map[string]any{"title": in.Title, "body": Description(in.Description, in.Impact)},
	})
	if err != nil {
		return Issue{}, err
	}

	return out.issue(in.Team), nil
}

/*
Comment posts a comment on the issue.
*/
func (g *GitHub) Comment(ctx context.Context, c Call, is Issue, body string) error {
	repo, number, ok := splitRef(is.ExternalID)
	if !ok {
		return ErrNotFound
	}

	return g.call(ctx, c, request{
		method: fhttp.MethodPost, url: "/repos/" + repo + "/issues/" + number + "/comments",
		body: map[string]any{"body": body},
	})
}

/*
Impact edits the impact comment, or posts it the first time.
*/
func (g *GitHub) Impact(ctx context.Context, c Call, is Issue, impact Impact) error {
	repo, number, ok := splitRef(is.ExternalID)
	if !ok || impact.Link == "" {
		return nil
	}

	body := impactMarker + "\n" + ImpactLine(impact) + "\n\nTracked in dimmi: " + impact.Link

	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}

	path := "/repos/" + repo + "/issues/" + number + "/comments?per_page=100"
	if err := g.call(ctx, c, get(path, &comments)); err != nil {
		return err
	}

	for _, existing := range comments {
		if strings.HasPrefix(existing.Body, impactMarker) {
			return g.call(ctx, c, request{
				method: fhttp.MethodPatch, url: "/repos/" + repo + "/issues/comments/" + formatInt(existing.ID),
				body: map[string]any{"body": body},
			})
		}
	}

	return g.Comment(ctx, c, is, body)
}

/*
Hook is a repository webhook: the repository, where it delivers and
the secret it signs with.
*/
type Hook struct {
	Repository string
	URL        string
	Secret     string
}

/*
Subscribe adds the webhook to the repository, a hook already there
counting as done.
*/
func (g *GitHub) Subscribe(ctx context.Context, c Call, h Hook) error {
	err := g.call(ctx, c, post("/repos/"+h.Repository+"/hooks", map[string]any{
		"name": "web", "active": true, "events": []string{"issues", "issue_comment", "deployment_status"},
		"config": map[string]any{"url": h.URL, "content_type": "json", "secret": h.Secret},
	}, nil))
	if errors.Is(err, ErrProvider) && strings.Contains(err.Error(), "422") {
		return nil
	}

	if err != nil {
		return fmt.Errorf("github hook: %w", err)
	}

	return nil
}

/*
Verify checks X-Hub-Signature-256 over the body.
*/
func (*GitHub) Verify(s Signed, secret string, _ time.Time) error {
	return verifyHex(s, "X-Hub-Signature-256", "sha256=", secret)
}

/*
splitRef splits owner/repo#123 into the repository and the number.
*/
func splitRef(ref string) (string, string, bool) {
	repo, number, ok := strings.Cut(ref, "#")

	return repo, number, ok && repo != "" && number != ""
}
