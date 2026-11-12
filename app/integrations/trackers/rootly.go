/*
|--------------------------------------------------------------------------
| Rootly
|--------------------------------------------------------------------------
|
| Rootly is connected with an API key and speaks JSON:API. Severities
| name a level (critical, high, medium, low) that becomes their rank;
| statuses run from in triage to closed (Rootly spells canceled with two Ls). Declaring from dimmi opens an
| incident of the picked severity; the impact on customers is kept in
| the incident's labels, and a comment becomes an internal timeline
| event. Deliveries are signed over a timestamp and the body.
|
*/

package trackers

import (
	"context"
	"maps"
	"net/url"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
RootlyAPI is Rootly's API.
*/
const RootlyAPI = "https://api.rootly.com/v1"

/*
Rootly is the Rootly tool.
*/
type Rootly struct {
	c client
}

/*
NewRootly returns the Rootly tool calling through the client.
*/
func NewRootly(h *httpclient.Client) *Rootly {
	return &Rootly{c: client{http: h, header: map[string]string{"Accept": "application/vnd.api+json"}}}
}

/*
Provider names Rootly.
*/
func (*Rootly) Provider() string { return "rootly" }

/*
Incidents reports true.
*/
func (*Rootly) Incidents() bool { return true }

/*
rootlyIncident is an incident's attributes, flat as deliveries carry
them.
*/
type rootlyIncident struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	SequentialID int64  `json:"sequential_id"`
	URL          string `json:"url"`
	ShortURL     string `json:"short_url"`
	Status       string `json:"status"`
	Severity     *struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Severity string `json:"severity"`
	} `json:"severity"`
}

/*
issue converts the incident.
*/
func (r rootlyIncident) issue() Issue {
	link := r.URL
	if link == "" {
		link = r.ShortURL
	}

	is := Issue{
		ExternalID: r.ID, Key: "#" + formatInt(r.SequentialID), URL: link, Title: r.Title,
		Status: strings.ReplaceAll(r.Status, "_", " "), StatusID: r.Status, StatusGroup: RootlyGroup(r.Status),
	}
	if r.Severity != nil {
		is.Severity, is.SeverityID, is.Rank = r.Severity.Name, r.Severity.ID, levelRank(r.Severity.Severity)
	}

	return is
}

/*
RootlyGroup returns the group of a status.
*/
func RootlyGroup(status string) string {
	switch status {
	case "in_triage":
		return GroupBacklog
	case "started", "mitigated":
		return GroupStarted
	case "resolved", "closed":
		return GroupDone
	case "cancel" + "led":
		return GroupCanceled
	}

	return GroupUnstarted
}

/*
levelRank turns a severity level into a rank, higher the worse.
*/
func levelRank(level string) int {
	switch strings.ToLower(level) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}

	return 0
}

/*
jsonAPI is one resource as JSON:API wraps it.
*/
type jsonAPI struct {
	ID         string         `json:"id"`
	Attributes map[string]any `json:"attributes"`
}

/*
flat returns the attributes with the id beside them.
*/
func (j jsonAPI) flat() map[string]any {
	out := map[string]any{"id": j.ID}
	maps.Copy(out, j.Attributes)

	if sev, ok := out["severity"].(map[string]any); ok {
		if data, ok := sev["data"].(map[string]any); ok {
			out["severity"] = jsonAPI{ID: str(data["id"]), Attributes: asMap(data["attributes"])}.flat()
		}
	}

	return out
}

/*
call sends one request to the API path.
*/
func (t *Rootly) call(ctx context.Context, c Call, r request) error {
	r.url = RootlyAPI + r.url

	return t.c.do(ctx, map[string]string{"Authorization": "Bearer " + c.Token}, r)
}

/*
Options returns severities or statuses.
*/
func (t *Rootly) Options(ctx context.Context, c Call, list, _ string) ([]Option, error) {
	switch list {
	case ListSeverities:
		var out struct {
			Data []jsonAPI `json:"data"`
		}

		if err := t.call(ctx, c, get("/severities?page[size]=50", &out)); err != nil {
			return nil, err
		}

		options := make([]Option, len(out.Data))
		for i, s := range out.Data {
			rank := levelRank(str(s.Attributes["severity"]))
			options[i] = Option{ID: s.ID, Name: str(s.Attributes["name"]), Rank: rank}
		}

		return options, nil
	case ListStates:
		var options []Option
		for _, s := range []string{"in_triage", "started", "mitigated", "resolved", "closed", "cancel" + "led"} {
			options = append(options, Option{ID: s, Name: strings.ReplaceAll(s, "_", " "), Group: RootlyGroup(s)})
		}

		return options, nil
	}

	return nil, ErrUnsupported
}

/*
list reads a list or one incident.
*/
func (t *Rootly) list(ctx context.Context, c Call, path string) ([]Issue, error) {
	var many struct {
		Data []jsonAPI `json:"data"`
	}

	if err := t.call(ctx, c, get(path, &many)); err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(many.Data))

	for _, d := range many.Data {
		var r rootlyIncident
		if err := remarshal(d.flat(), &r); err == nil {
			issues = append(issues, r.issue())
		}
	}

	return issues, nil
}

/*
Search finds incidents by their words.
*/
func (t *Rootly) Search(ctx context.Context, c Call, q string) ([]Issue, error) {
	return t.list(ctx, c, "/incidents?page[size]=20&filter[search]="+url.QueryEscape(q))
}

/*
Read returns the incident an id or URL names.
*/
func (t *Rootly) Read(ctx context.Context, c Call, ref string) (Issue, error) {
	id := strings.TrimSpace(ref)
	if m := incidentRef.FindStringSubmatch(ref); m != nil {
		id = m[1]
	}

	var one struct {
		Data jsonAPI `json:"data"`
	}

	if err := t.call(ctx, c, get("/incidents/"+url.PathEscape(id), &one)); err != nil {
		return Issue{}, err
	}

	var r rootlyIncident
	err := remarshal(one.Data.flat(), &r)

	return r.issue(), err
}

/*
Create declares the incident and writes its impact.
*/
func (t *Rootly) Create(ctx context.Context, c Call, in CreateInput) (Issue, error) {
	attrs := map[string]any{"title": in.Title, "summary": Description(in.Description, in.Impact), "kind": "normal"}
	if in.Severity != "" {
		attrs["severity_id"] = in.Severity
	}

	var one struct {
		Data jsonAPI `json:"data"`
	}

	err := t.call(ctx, c, request{method: fhttp.MethodPost, url: "/incidents", out: &one, body: map[string]any{
		"data": map[string]any{"type": "incidents", "attributes": attrs},
	}})
	if err != nil {
		return Issue{}, err
	}

	var r rootlyIncident
	if err := remarshal(one.Data.flat(), &r); err != nil {
		return Issue{}, err
	}

	is := r.issue()

	return is, t.Impact(ctx, c, is, in.Impact)
}

/*
Comment adds an internal event to the incident's timeline.
*/
func (t *Rootly) Comment(ctx context.Context, c Call, is Issue, body string) error {
	return t.call(ctx, c, request{
		method: fhttp.MethodPost, url: "/incidents/" + url.PathEscape(is.ExternalID) + "/events",
		body: map[string]any{"data": map[string]any{
			"type": "incident_events", "attributes": map[string]any{"event": body, "visibility": "internal"},
		}},
	})
}

/*
Impact keeps the customers, companies and revenue in the labels.
*/
func (t *Rootly) Impact(ctx context.Context, c Call, is Issue, impact Impact) error {
	labels := map[string]any{
		"dimmi_customers": formatInt(impact.Customers), "dimmi_companies": formatInt(impact.Companies),
		"dimmi_revenue": Money(impact.Revenue),
	}

	return t.call(ctx, c, request{
		method: fhttp.MethodPut, url: "/incidents/" + url.PathEscape(is.ExternalID),
		body: map[string]any{"data": map[string]any{
			"type": "incidents", "attributes": map[string]any{"labels": labels},
		}},
	})
}

/*
Verify checks X-Rootly-Signature.
*/
func (*Rootly) Verify(s Signed, secret string, now time.Time) error {
	return verifyRootly(s, secret, now)
}

/*
Parse reads a Rootly delivery: a new or changed incident, or a status
page event posted on one.
*/
func (*Rootly) Parse(body []byte) ([]Event, error) {
	var d struct {
		Event struct {
			Type string `json:"type"`
		} `json:"event"`
		Data map[string]any `json:"data"`
	}

	if err := decode(body, &d); err != nil {
		return nil, err
	}

	if strings.HasPrefix(d.Event.Type, "incident_status_page_event") {
		return []Event{{Kind: EventUpdate, ExternalID: str(d.Data["incident_id"]), Body: str(d.Data["event"])}}, nil
	}

	if !strings.HasPrefix(d.Event.Type, "incident.") {
		return nil, nil
	}

	var r rootlyIncident
	if err := remarshal(d.Data, &r); err != nil || r.ID == "" {
		return nil, err
	}

	is := r.issue()

	event := Event{Kind: EventIssue, ExternalID: r.ID, Issue: &is}
	if d.Event.Type == "incident.created" {
		event.Kind = EventIncident
	}

	return []Event{event}, nil
}
