/*
|--------------------------------------------------------------------------
| Jira Cloud
|--------------------------------------------------------------------------
|
| Jira Cloud is reached through Atlassian's API gateway with the site's
| cloud id, which the install kept from the accessible resources. A
| project plays the part of a team and its issue types that of a
| project; statuses fall into Jira's own three categories, and a done
| issue resolved as won't do or duplicate counts as canceled. The impact
| is written as a remote link back to the tracker, which Jira updates in
| place by its global id, and as an issue property JQL can sort by.
|
*/

package trackers

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
JiraGateway is Atlassian's API gateway for Jira Cloud sites.
*/
const JiraGateway = "https://api.atlassian.com/ex/jira/"

/*
jiraKey matches an issue key in text or a URL, such as PAY-12.
*/
var jiraKey = regexp.MustCompile(`\b([A-Z][A-Z0-9_]{0,9}-\d+)\b`)

/*
Jira is the Jira Cloud tracker.
*/
type Jira struct {
	c client
}

/*
NewJira returns the Jira tracker calling through the client.
*/
func NewJira(h *httpclient.Client) *Jira {
	return &Jira{c: client{http: h}}
}

/*
Provider names Jira.
*/
func (*Jira) Provider() string { return "jira" }

/*
Incidents reports false: Jira holds issues.
*/
func (*Jira) Incidents() bool { return false }

/*
jiraIssue is an issue as the REST API returns it.
*/
type jiraIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary string     `json:"summary"`
		Status  jiraStatus `json:"status"`
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
		Resolution *struct {
			Name string `json:"name"`
		} `json:"resolution"`
	} `json:"fields"`
}

/*
jiraStatus is a status with its category.
*/
type jiraStatus struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category struct {
		Key string `json:"key"`
	} `json:"statusCategory"`
}

/*
issue converts the REST issue, linking it on the site.
*/
func (j jiraIssue) issue(site string) Issue {
	group := JiraGroup(j.Fields.Status.Category.Key)
	if r := j.Fields.Resolution; group == GroupDone && r != nil && unresolvedName(r.Name) {
		group = GroupCanceled
	}

	link := ""
	if site != "" {
		link = strings.TrimRight(site, "/") + "/browse/" + j.Key
	}

	return Issue{
		ExternalID: j.ID, Key: j.Key, URL: link, Title: j.Fields.Summary, Status: j.Fields.Status.Name,
		StatusID: j.Fields.Status.ID, StatusGroup: group, TeamID: j.Fields.Project.ID, TeamName: j.Fields.Project.Name,
	}
}

/*
JiraGroup returns the group of a status category.
*/
func JiraGroup(category string) string {
	switch category {
	case "indeterminate":
		return GroupStarted
	case "done":
		return GroupDone
	default:
		return GroupUnstarted
	}
}

/*
unresolvedName reports a resolution that means the work will not be
done.
*/
func unresolvedName(name string) bool {
	lower := strings.ToLower(name)

	return strings.Contains(lower, "won") || strings.Contains(lower, "duplicate") ||
		strings.Contains(lower, "cannot") || strings.Contains(lower, "declined")
}

/*
api returns the site's REST address of the path.
*/
func (*Jira) api(c Call, path string) string {
	return JiraGateway + url.PathEscape(c.meta("cloud_id")) + "/rest/api/3" + path
}

/*
call sends one request to the site.
*/
func (j *Jira) call(ctx context.Context, c Call, r request) error {
	r.url = j.api(c, r.url)

	return j.c.do(ctx, map[string]string{"Authorization": "Bearer " + c.Token}, r)
}

/*
Options returns projects, a project's issue types or its statuses.
*/
func (j *Jira) Options(ctx context.Context, c Call, list, parent string) ([]Option, error) {
	switch list {
	case ListTeams:
		var out struct {
			Values []struct {
				ID   string `json:"id"`
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"values"`
		}

		if err := j.call(ctx, c, get("/project/search?maxResults=100", &out)); err != nil {
			return nil, err
		}

		options := make([]Option, len(out.Values))
		for i, p := range out.Values {
			options[i] = Option{ID: p.ID, Name: p.Name, Detail: p.Key}
		}

		return options, nil
	case ListProjects:
		return j.issueTypes(ctx, c, parent)
	case ListStates:
		return j.statuses(ctx, c, parent)
	}

	return nil, ErrUnsupported
}

/*
issueTypes lists the issue types a project files.
*/
func (j *Jira) issueTypes(ctx context.Context, c Call, project string) ([]Option, error) {
	var out struct {
		IssueTypes []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Subtask bool   `json:"subtask"`
		} `json:"issueTypes"`
	}

	if err := j.call(ctx, c, get("/project/"+url.PathEscape(project), &out)); err != nil {
		return nil, err
	}

	var options []Option

	for _, t := range out.IssueTypes {
		if !t.Subtask {
			options = append(options, Option{ID: t.ID, Name: t.Name})
		}
	}

	return options, nil
}

/*
statuses lists every status a project's issue types use, once each.
*/
func (j *Jira) statuses(ctx context.Context, c Call, project string) ([]Option, error) {
	var out []struct {
		Statuses []jiraStatus `json:"statuses"`
	}

	err := j.call(ctx, c, get("/project/"+url.PathEscape(project)+"/statuses", &out))
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}

	var options []Option

	for _, t := range out {
		for _, s := range t.Statuses {
			if !seen[s.ID] {
				seen[s.ID] = true
				options = append(options, Option{ID: s.ID, Name: s.Name, Group: JiraGroup(s.Category.Key)})
			}
		}
	}

	return options, nil
}

/*
Search runs JQL: the key when the words hold one, a text search
otherwise.
*/
func (j *Jira) Search(ctx context.Context, c Call, q string) ([]Issue, error) {
	jql := `text ~ "` + strings.ReplaceAll(strings.ReplaceAll(q, `\`, `\\`), `"`, `\"`) + `" ORDER BY updated DESC`
	if key := jiraKey.FindString(q); key != "" {
		jql = `key = "` + key + `"`
	}

	var out struct {
		Issues []jiraIssue `json:"issues"`
	}

	err := j.call(ctx, c, request{method: fhttp.MethodPost, url: "/search/jql", out: &out, body: map[string]any{
		"jql": jql, "maxResults": 20, "fields": []string{"summary", "status", "project", "resolution"},
	}})
	if err != nil {
		return nil, err
	}

	issues := make([]Issue, len(out.Issues))
	for i, is := range out.Issues {
		issues[i] = is.issue(c.meta("site_url"))
	}

	return issues, nil
}

/*
Read returns the issue an id, key or URL names.
*/
func (j *Jira) Read(ctx context.Context, c Call, ref string) (Issue, error) {
	id := strings.TrimSpace(ref)
	if key := jiraKey.FindString(id); key != "" {
		id = key
	}

	var out jiraIssue

	err := j.call(ctx, c, request{
		method: fhttp.MethodGet, out: &out,
		url: "/issue/" + url.PathEscape(id) + "?fields=summary,status,project,resolution",
	})

	return out.issue(c.meta("site_url")), err
}

/*
Create files the issue in the project as the issue type, then writes
its impact.
*/
func (j *Jira) Create(ctx context.Context, c Call, in CreateInput) (Issue, error) {
	fields := map[string]any{
		"project": map[string]any{"id": in.Team}, "summary": in.Title,
		"description": adf(Description(in.Description, in.Impact)),
	}
	if in.Project != "" {
		fields["issuetype"] = map[string]any{"id": in.Project}
	}

	var made struct {
		ID string `json:"id"`
	}

	if err := j.call(ctx, c, post("/issue", map[string]any{"fields": fields}, &made)); err != nil {
		return Issue{}, err
	}

	is, err := j.Read(ctx, c, made.ID)
	if err != nil {
		return Issue{}, err
	}

	return is, j.Impact(ctx, c, is, in.Impact)
}

/*
Comment posts a comment on the issue.
*/
func (j *Jira) Comment(ctx context.Context, c Call, is Issue, body string) error {
	return j.call(ctx, c, request{
		method: fhttp.MethodPost, url: "/issue/" + url.PathEscape(is.ExternalID) + "/comment",
		body: map[string]any{"body": adf(body)},
	})
}

/*
Impact writes the remote link back to the tracker and the property
JQL ranks by.
*/
func (j *Jira) Impact(ctx context.Context, c Call, is Issue, impact Impact) error {
	if impact.Link == "" {
		return nil
	}

	path := "/issue/" + url.PathEscape(is.ExternalID)

	err := j.call(ctx, c, request{method: fhttp.MethodPost, url: path + "/remotelink", body: map[string]any{
		"globalId": impact.Link,
		"object":   map[string]any{"url": impact.Link, "title": "dimmi tracker", "summary": ImpactLine(impact)},
	}})
	if err != nil {
		return err
	}

	return j.call(ctx, c, request{method: fhttp.MethodPut, url: path + "/properties/dimmi.impact", body: map[string]any{
		"customers": impact.Customers, "companies": impact.Companies, "conversations": impact.Conversations,
		"revenue": impact.Revenue,
	}})
}

/*
Verify checks the delivery's token was signed with the app's secret.
*/
func (*Jira) Verify(s Signed, secret string, now time.Time) error {
	return verifyJWT(s, secret, now)
}
