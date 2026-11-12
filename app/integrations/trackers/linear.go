/*
|--------------------------------------------------------------------------
| Linear
|--------------------------------------------------------------------------
|
| Linear is one GraphQL API. Teams, their projects and workflow states
| feed the pickers; an issue is read by its id or its identifier, such
| as ENG-412, which is also what a pasted URL carries. A new issue gets
| the customer impact in its description and one customer request per
| company, and the impact stays current as an attachment that links back
| to the tracker: Linear keeps one attachment per URL, so writing it
| again updates it. A workflow state's type is its group; triage counts
| as backlog.
|
| An issue Linear will not show to a token that saw it before sits in a
| team the app was removed from, which the sync reports as ErrHidden.
|
*/

package trackers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
LinearAPI is Linear's GraphQL endpoint.
*/
const LinearAPI = "https://api.linear.app/graphql"

/*
linearIssueFields are the fields read of every issue.
*/
const linearIssueFields = `id identifier title url state { id name type } team { id name }`

/*
linearKey matches an identifier in text or a URL, such as ENG-412.
*/
var linearKey = regexp.MustCompile(`\b([A-Z][A-Z0-9]{0,9}-\d+)\b`)

/*
Linear is the Linear tracker.
*/
type Linear struct {
	c client
}

/*
NewLinear returns the Linear tracker calling through the client.
*/
func NewLinear(h *httpclient.Client) *Linear {
	return &Linear{c: client{http: h}}
}

/*
Provider names Linear.
*/
func (*Linear) Provider() string { return "linear" }

/*
Incidents reports false: Linear holds issues.
*/
func (*Linear) Incidents() bool { return false }

/*
linearIssue is an issue as GraphQL returns it.
*/
type linearIssue struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	State      struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"state"`
	Team struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
}

/*
issue converts the GraphQL issue.
*/
func (l linearIssue) issue() Issue {
	return Issue{
		ExternalID: l.ID, Key: l.Identifier, URL: l.URL, Title: l.Title, Status: l.State.Name,
		StatusID: l.State.ID, StatusGroup: LinearGroup(l.State.Type), TeamID: l.Team.ID, TeamName: l.Team.Name,
	}
}

/*
LinearGroup returns the group of a workflow state type.
*/
func LinearGroup(stateType string) string {
	switch stateType {
	case "triage", "backlog":
		return GroupBacklog
	case "started":
		return GroupStarted
	case "completed":
		return GroupDone
	case "canceled":
		return GroupCanceled
	default:
		return GroupUnstarted
	}
}

/*
graphQLError is one error GraphQL reports.
*/
type graphQLError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
		Type string `json:"type"`
	} `json:"extensions"`
}

/*
gql is a GraphQL document and its variables.
*/
type gql struct {
	doc  string
	vars map[string]any
}

/*
query runs a GraphQL document with its variables into out.
*/
func (l *Linear) query(ctx context.Context, c Call, g gql, out any) error {
	var resp struct {
		Data   any            `json:"data"`
		Errors []graphQLError `json:"errors"`
	}

	resp.Data = out

	err := l.c.do(ctx, map[string]string{"Authorization": "Bearer " + c.Token}, request{
		method: fhttp.MethodPost, url: LinearAPI, body: map[string]any{"query": g.doc, "variables": g.vars}, out: &resp,
	})
	if err != nil {
		return err
	}

	if len(resp.Errors) == 0 {
		return nil
	}

	e := resp.Errors[0]
	lower := strings.ToLower(e.Message + " " + e.Extensions.Code + " " + e.Extensions.Type)

	switch {
	case strings.Contains(lower, "not found"):
		return fmt.Errorf("linear: %w: %s", ErrNotFound, e.Message)
	case strings.Contains(lower, "authentication") || strings.Contains(lower, "forbidden"):
		return fmt.Errorf("linear: %w: %s", ErrAuth, e.Message)
	default:
		return fmt.Errorf("linear: %w: %s", ErrProvider, e.Message)
	}
}

/*
linearNodes is a connection's nodes of options.
*/
type linearNodes struct {
	Nodes []struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Key      string  `json:"key"`
		Type     string  `json:"type"`
		Position float64 `json:"position"`
	} `json:"nodes"`
}

/*
Options returns teams, a team's projects or a team's workflow states.
*/
func (l *Linear) Options(ctx context.Context, c Call, list, parent string) ([]Option, error) {
	var out struct {
		Teams linearNodes `json:"teams"`
		Team  struct {
			Projects linearNodes `json:"projects"`
			States   linearNodes `json:"states"`
		} `json:"team"`
	}

	var nodes *linearNodes

	switch list {
	case ListTeams:
		nodes = &out.Teams
		if err := l.query(ctx, c, gql{`query { teams(first: 100) { nodes { id name key } } }`, nil}, &out); err != nil {
			return nil, err
		}
	case ListProjects, ListStates:
		field := "projects(first: 100) { nodes { id name } }"
		nodes = &out.Team.Projects

		if list == ListStates {
			field, nodes = "states { nodes { id name type position } }", &out.Team.States
		}

		doc := `query($id: String!) { team(id: $id) { ` + field + ` } }`
		if err := l.query(ctx, c, gql{doc, map[string]any{"id": parent}}, &out); err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnsupported
	}

	options := make([]Option, 0, len(nodes.Nodes))
	for _, n := range nodes.Nodes {
		o := Option{ID: n.ID, Name: n.Name, Detail: n.Key}
		if n.Type != "" {
			o.Group, o.Detail = LinearGroup(n.Type), n.Type
		}

		options = append(options, o)
	}

	return options, nil
}

/*
Search finds issues by the words, or the issue a key or URL names.
*/
func (l *Linear) Search(ctx context.Context, c Call, q string) ([]Issue, error) {
	if key := linearKey.FindString(q); key != "" {
		is, err := l.Read(ctx, c, key)
		if err == nil {
			return []Issue{is}, nil
		}
	}

	var out struct {
		Search struct {
			Nodes []linearIssue `json:"nodes"`
		} `json:"searchIssues"`
	}

	doc := `query($q: String!) { searchIssues(term: $q, first: 20) { nodes { ` + linearIssueFields + ` } } }`
	if err := l.query(ctx, c, gql{doc, map[string]any{"q": q}}, &out); err != nil {
		return nil, err
	}

	issues := make([]Issue, len(out.Search.Nodes))
	for i, n := range out.Search.Nodes {
		issues[i] = n.issue()
	}

	return issues, nil
}

/*
Read returns the issue an id, identifier or URL names.
*/
func (l *Linear) Read(ctx context.Context, c Call, ref string) (Issue, error) {
	id := strings.TrimSpace(ref)
	if key := linearKey.FindString(id); key != "" {
		id = key
	}

	var out struct {
		Issue *linearIssue `json:"issue"`
	}

	doc := `query($id: String!) { issue(id: $id) { ` + linearIssueFields + ` } }`
	if err := l.query(ctx, c, gql{doc, map[string]any{"id": id}}, &out); err != nil {
		return Issue{}, err
	}

	if out.Issue == nil {
		return Issue{}, ErrNotFound
	}

	return out.Issue.issue(), nil
}

/*
Create files the issue in the team and project, then adds a customer
request for each company it reaches, best effort: a workspace without
Linear's customer requests still gets the issue.
*/
func (l *Linear) Create(ctx context.Context, c Call, in CreateInput) (Issue, error) {
	input := map[string]any{"teamId": in.Team, "title": in.Title, "description": Description(in.Description, in.Impact)}
	if in.Project != "" {
		input["projectId"] = in.Project
	}

	var out struct {
		Create struct {
			Success bool        `json:"success"`
			Issue   linearIssue `json:"issue"`
		} `json:"issueCreate"`
	}

	doc := `mutation($input: IssueCreateInput!) { issueCreate(input: $input) { success issue { ` +
		linearIssueFields + ` } } }`
	if err := l.query(ctx, c, gql{doc, map[string]any{"input": input}}, &out); err != nil {
		return Issue{}, err
	}

	is := out.Create.Issue.issue()

	for _, company := range in.Impact.CompanyNames {
		_ = l.customerRequest(ctx, c, is, company)
	}

	return is, l.Impact(ctx, c, is, in.Impact)
}

/*
customerRequest records that the company asked for the issue.
*/
func (l *Linear) customerRequest(ctx context.Context, c Call, is Issue, company string) error {
	var out struct {
		Upsert struct {
			Customer struct {
				ID string `json:"id"`
			} `json:"customer"`
		} `json:"customerUpsert"`
	}

	doc := `mutation($input: CustomerUpsertInput!) { customerUpsert(input: $input) { customer { id } } }`
	if err := l.query(ctx, c, gql{doc, map[string]any{"input": map[string]any{"name": company}}}, &out); err != nil {
		return err
	}

	need := map[string]any{
		"customerId": out.Upsert.Customer.ID, "issueId": is.ExternalID, "body": "Reported to support",
	}
	doc = `mutation($input: CustomerNeedCreateInput!) { customerNeedCreate(input: $input) { success } }`

	return l.query(ctx, c, gql{doc, map[string]any{"input": need}}, nil)
}

/*
Comment posts a comment on the issue.
*/
func (l *Linear) Comment(ctx context.Context, c Call, is Issue, body string) error {
	doc := `mutation($input: CommentCreateInput!) { commentCreate(input: $input) { success } }`

	input := map[string]any{"issueId": is.ExternalID, "body": body}

	return l.query(ctx, c, gql{doc, map[string]any{"input": input}}, nil)
}

/*
Impact writes the impact as the attachment linking back to the
tracker.
*/
func (l *Linear) Impact(ctx context.Context, c Call, is Issue, impact Impact) error {
	if impact.Link == "" {
		return nil
	}

	input := map[string]any{
		"issueId": is.ExternalID, "url": impact.Link,
		"title":    fmt.Sprintf("dimmi: %s", plural(impact.Customers, "customer")),
		"subtitle": ImpactLine(impact),
	}
	doc := `mutation($input: AttachmentCreateInput!) { attachmentCreate(input: $input) { success } }`

	return l.query(ctx, c, gql{doc, map[string]any{"input": input}}, nil)
}

/*
Verify checks Linear-Signature over the body and that the delivery's
own timestamp is recent.
*/
func (*Linear) Verify(s Signed, secret string, now time.Time) error {
	if err := verifyHex(s, "Linear-Signature", "", secret); err != nil {
		return err
	}

	var stamp struct {
		At int64 `json:"webhookTimestamp"`
	}

	if decode(s.Body, &stamp) != nil || stamp.At == 0 {
		return ErrBadSignature
	}

	if age := now.Sub(time.UnixMilli(stamp.At)); age > replayWindow || age < -replayWindow {
		return ErrBadSignature
	}

	return nil
}
