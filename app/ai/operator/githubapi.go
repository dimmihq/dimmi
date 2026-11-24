/*
|--------------------------------------------------------------------------
| GitHub API
|--------------------------------------------------------------------------
|
| The pull requests a routine reads come from GitHub's REST API, with a
| token of the workspace's GitHub connection asked for on every call and
| never kept. A failed call says what GitHub answered, which the model
| reads as a refusal and reports.
|
*/

package operator

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
GitHubTokens hands out a connection's token.
*/
type GitHubTokens interface {
	/*
		Bot returns the token acting as the app on the connection.
	*/
	Bot(ctx context.Context, installationID ids.UUID) (string, error)
}

/*
GitHubAPI reads pull requests over GitHub's REST API. Base is the API's
address, GitHub's own when empty.
*/
type GitHubAPI struct {
	HTTP   *httpclient.Client
	Tokens GitHubTokens
	Base   string
}

/*
githubAPI is GitHub's own API address.
*/
const githubAPI = "https://api.github.com"

/*
githubPull is a pull request as GitHub returns it.
*/
type githubPull struct {
	Number   int        `json:"number"`
	Title    string     `json:"title"`
	Body     string     `json:"body"`
	HTMLURL  string     `json:"html_url"`
	MergedAt *time.Time `json:"merged_at"`
	User     struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

/*
pull reads the parts a tool shows.
*/
func (p githubPull) pull() Pull {
	out := Pull{Number: p.Number, Title: p.Title, Body: p.Body, Author: p.User.Login, URL: p.HTMLURL}
	if p.MergedAt != nil {
		out.MergedAt = *p.MergedAt
	}
	for _, l := range p.Labels {
		out.Labels = append(out.Labels, l.Name)
	}
	return out
}

/*
MergedPulls lists the pull requests merged into a branch since a moment.
GitHub lists closed pull requests by when they last changed, so the
listing stops at the first one that changed before the moment.
*/
func (g *GitHubAPI) MergedPulls(ctx context.Context, in PullQuery) ([]Pull, error) {
	query := url.Values{
		"state": {"closed"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {"50"},
	}
	if in.Branch != "" {
		query.Set("base", in.Branch)
	}
	var rows []struct {
		githubPull
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := g.get(ctx, in.Installation, "/repos/"+repoPath(in.Repo)+"/pulls?"+query.Encode(), &rows); err != nil {
		return nil, err
	}
	var out []Pull
	for _, r := range rows {
		if r.UpdatedAt.Before(in.Since) {
			break
		}
		if r.MergedAt == nil || r.MergedAt.Before(in.Since) {
			continue
		}
		out = append(out, r.pull())
		if len(out) == in.Limit {
			break
		}
	}
	return out, nil
}

/*
PullDetail reads one pull request and the files it changed.
*/
func (g *GitHubAPI) PullDetail(ctx context.Context, installation ids.UUID, repo string, number int,
) (PullDetail, error) {
	var row githubPull
	base := fmt.Sprintf("/repos/%s/pulls/%d", repoPath(repo), number)
	if err := g.get(ctx, installation, base, &row); err != nil {
		return PullDetail{}, err
	}
	var files []struct {
		Filename  string `json:"filename"`
		Status    string `json:"status"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Patch     string `json:"patch"`
	}
	if err := g.get(ctx, installation, base+"/files?per_page=100", &files); err != nil {
		return PullDetail{}, err
	}
	out := PullDetail{Pull: row.pull()}
	for _, f := range files {
		out.Files = append(out.Files, PullFile{
			Name: f.Filename, Status: f.Status, Additions: f.Additions, Deletions: f.Deletions, Patch: f.Patch,
		})
	}
	return out, nil
}

/*
get calls GitHub with the connection's token and decodes the answer.
*/
func (g *GitHubAPI) get(ctx context.Context, installation ids.UUID, path string, out any) error {
	token, err := g.Tokens.Bot(ctx, installation)
	if err != nil {
		return fmt.Errorf("the GitHub connection is not available: %w", err)
	}
	base := g.Base
	if base == "" {
		base = githubAPI
	}
	resp, err := g.HTTP.New().WithHeader("X-GitHub-Api-Version", "2022-11-28").WithToken(token).
		Accept("application/vnd.github+json").Get(ctx, base+path)
	if err != nil {
		return fmt.Errorf("github: %w", err)
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return fmt.Errorf("github answered %d", resp.Status)
	}
	return resp.JSON(out)
}

/*
repoPath escapes an owner/repo for a URL path.
*/
func repoPath(repo string) string {
	owner, name, _ := strings.Cut(repo, "/")
	return url.PathEscape(owner) + "/" + url.PathEscape(name)
}
