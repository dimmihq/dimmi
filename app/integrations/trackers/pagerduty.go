/*
|--------------------------------------------------------------------------
| PagerDuty
|--------------------------------------------------------------------------
|
| PagerDuty is connected with a REST API key. Declaring from dimmi
| triggers an incident on the picked service, which pages whoever is
| on call; PagerDuty asks who triggers it, so the workspace picks one of
| its PagerDuty users to act as. Triggered is unstarted, acknowledged
| has started and resolved is done. Priorities are the severities when
| the account uses them, urgencies otherwise. Webhooks v3 sign the body
| with each subscription's secret.
|
*/

package trackers

import (
	"context"
	"net/url"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
PagerDutyAPI is PagerDuty's REST API.
*/
const PagerDutyAPI = "https://api.pagerduty.com"

/*
PagerDuty is the PagerDuty tool.
*/
type PagerDuty struct {
	c client
}

/*
NewPagerDuty returns the PagerDuty tool calling through the client.
*/
func NewPagerDuty(h *httpclient.Client) *PagerDuty {
	header := map[string]string{"Accept": "application/vnd.pagerduty+json;version=2"}

	return &PagerDuty{c: client{http: h, header: header}}
}

/*
Provider names PagerDuty.
*/
func (*PagerDuty) Provider() string { return "pagerduty" }

/*
Incidents reports true.
*/
func (*PagerDuty) Incidents() bool { return true }

/*
pdIncident is an incident as the API and webhooks carry it.
*/
type pdIncident struct {
	ID       string `json:"id"`
	Number   int64  `json:"number"`
	Incident int64  `json:"incident_number"`
	Title    string `json:"title"`
	HTMLURL  string `json:"html_url"`
	Status   string `json:"status"`
	Urgency  string `json:"urgency"`
	Service  struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
	} `json:"service"`
	Priority *struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
	} `json:"priority"`
}

/*
issue converts the incident.
*/
func (p pdIncident) issue() Issue {
	number := p.Incident
	if number == 0 {
		number = p.Number
	}

	is := Issue{
		ExternalID: p.ID, Key: "#" + formatInt(number), URL: p.HTMLURL, Title: p.Title, Status: p.Status,
		StatusID: p.Status, StatusGroup: PagerDutyGroup(p.Status), TeamID: p.Service.ID, TeamName: p.Service.Summary,
		Severity: p.Urgency, SeverityID: p.Urgency, Rank: urgencyRank(p.Urgency),
	}
	if p.Priority != nil {
		is.Severity, is.SeverityID = p.Priority.Summary, p.Priority.ID
	}

	return is
}

/*
PagerDutyGroup returns the group of a status.
*/
func PagerDutyGroup(status string) string {
	switch status {
	case "acknowledged":
		return GroupStarted
	case "resolved":
		return GroupDone
	}

	return GroupUnstarted
}

/*
urgencyRank ranks an urgency, higher the worse.
*/
func urgencyRank(urgency string) int {
	if urgency == "high" {
		return 2
	}

	return 1
}

/*
call sends one request to the API path, as the user the workspace
picked when the call needs one.
*/
func (t *PagerDuty) call(ctx context.Context, c Call, r request) error {
	r.url = PagerDutyAPI + r.url
	header := map[string]string{"Authorization": "Token token=" + c.Token}

	if from := c.meta("from_email"); from != "" {
		header["From"] = from
	}

	return t.c.do(ctx, header, r)
}

/*
pdNamed is a service, user or priority.
*/
type pdNamed struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Email   string `json:"email"`
	Order   int    `json:"order"`
}

/*
Options returns services, users, severities or statuses.
*/
func (t *PagerDuty) Options(ctx context.Context, c Call, list, _ string) ([]Option, error) {
	var out struct {
		Services   []pdNamed `json:"services"`
		Users      []pdNamed `json:"users"`
		Priorities []pdNamed `json:"priorities"`
	}

	var options []Option

	switch list {
	case ListServices:
		if err := t.call(ctx, c, get("/services?limit=100", &out)); err != nil {
			return nil, err
		}

		for _, s := range out.Services {
			options = append(options, Option{ID: s.ID, Name: s.Name})
		}
	case ListUsers:
		if err := t.call(ctx, c, get("/users?limit=100", &out)); err != nil {
			return nil, err
		}

		for _, u := range out.Users {
			options = append(options, Option{ID: u.Email, Name: u.Name, Detail: u.Email})
		}
	case ListSeverities:
		return t.severities(ctx, c)
	case ListStates:
		for _, s := range []string{"triggered", "acknowledged", "resolved"} {
			options = append(options, Option{ID: s, Name: s, Group: PagerDutyGroup(s)})
		}
	default:
		return nil, ErrUnsupported
	}

	return options, nil
}

/*
severities returns the account's priorities, or the two urgencies when
it uses none.
*/
func (t *PagerDuty) severities(ctx context.Context, c Call) ([]Option, error) {
	var out struct {
		Priorities []pdNamed `json:"priorities"`
	}

	_ = t.call(ctx, c, get("/priorities", &out))

	if len(out.Priorities) == 0 {
		return []Option{{ID: "high", Name: "High urgency", Rank: 2}, {ID: "low", Name: "Low urgency", Rank: 1}}, nil
	}

	options := make([]Option, len(out.Priorities))
	for i, p := range out.Priorities {
		options[i] = Option{ID: p.ID, Name: p.Name, Rank: p.Order}
	}

	return options, nil
}

/*
Search finds open incidents whose title has the words.
*/
func (t *PagerDuty) Search(ctx context.Context, c Call, q string) ([]Issue, error) {
	var out struct {
		Incidents []pdIncident `json:"incidents"`
	}

	path := "/incidents?limit=50&statuses[]=triggered&statuses[]=acknowledged"
	if err := t.call(ctx, c, get(path, &out)); err != nil {
		return nil, err
	}

	var issues []Issue

	for _, i := range out.Incidents {
		if q == "" || strings.Contains(strings.ToLower(i.Title), strings.ToLower(q)) {
			issues = append(issues, i.issue())
		}
	}

	return issues, nil
}

/*
Read returns the incident an id or URL names.
*/
func (t *PagerDuty) Read(ctx context.Context, c Call, ref string) (Issue, error) {
	id := strings.TrimSpace(ref)
	if m := incidentRef.FindStringSubmatch(ref); m != nil {
		id = m[1]
	}

	var out struct {
		Incident pdIncident `json:"incident"`
	}

	err := t.call(ctx, c, get("/incidents/"+url.PathEscape(id), &out))

	return out.Incident.issue(), err
}

/*
Create triggers the incident on the service.
*/
func (t *PagerDuty) Create(ctx context.Context, c Call, in CreateInput) (Issue, error) {
	incident := map[string]any{
		"type": "incident", "title": in.Title,
		"service": map[string]any{"id": in.Service, "type": "service_reference"},
		"body":    map[string]any{"type": "incident_body", "details": Description(in.Description, in.Impact)},
	}

	switch in.Severity {
	case "", "high", "low":
		incident["urgency"] = map[bool]string{true: "high", false: in.Severity}[in.Severity == ""]
	default:
		incident["priority"] = map[string]any{"id": in.Severity, "type": "priority_reference"}
	}

	var out struct {
		Incident pdIncident `json:"incident"`
	}

	err := t.call(ctx, c, post("/incidents", map[string]any{"incident": incident}, &out))

	return out.Incident.issue(), err
}

/*
Comment adds a note to the incident.
*/
func (t *PagerDuty) Comment(ctx context.Context, c Call, is Issue, body string) error {
	return t.call(ctx, c, request{
		method: fhttp.MethodPost, url: "/incidents/" + url.PathEscape(is.ExternalID) + "/notes",
		body: map[string]any{"note": map[string]any{"content": body}},
	})
}

/*
Impact is not written to PagerDuty, which pages on urgency alone.
*/
func (*PagerDuty) Impact(context.Context, Call, Issue, Impact) error {
	return nil
}

/*
Verify checks X-PagerDuty-Signature.
*/
func (*PagerDuty) Verify(s Signed, secret string, _ time.Time) error {
	return verifyPagerDuty(s, secret)
}

/*
Parse reads a webhook v3 delivery: an incident that changed, or a
status update published on one.
*/
func (*PagerDuty) Parse(body []byte) ([]Event, error) {
	var d struct {
		Event struct {
			Type string         `json:"event_type"`
			Data map[string]any `json:"data"`
		} `json:"event"`
	}

	if err := decode(body, &d); err != nil {
		return nil, err
	}

	data := d.Event.Data

	switch d.Event.Type {
	case "incident.status_update_published", "incident.annotated":
		text := str(data["message"])
		if text == "" {
			text = str(data["content"])
		}

		kind := EventUpdate
		if d.Event.Type == "incident.annotated" {
			kind = EventComment
		}

		return []Event{{Kind: kind, ExternalID: str(asMap(data["incident"])["id"]), Body: text}}, nil
	}

	if !strings.HasPrefix(d.Event.Type, "incident.") || str(data["type"]) != "incident" {
		return nil, nil
	}

	var p pdIncident
	if err := remarshal(data, &p); err != nil {
		return nil, err
	}

	is := p.issue()

	event := Event{Kind: EventIssue, ExternalID: p.ID, Issue: &is}
	if d.Event.Type == "incident.triggered" {
		event.Kind = EventIncident
	}

	return []Event{event}, nil
}
