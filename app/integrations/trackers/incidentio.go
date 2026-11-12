/*
|--------------------------------------------------------------------------
| incident.io
|--------------------------------------------------------------------------
|
| incident.io is connected with an API key and delivers through Svix,
| signed with the endpoint's own secret. Severities carry a rank, higher
| the worse, which the mirror threshold compares; statuses fall into
| categories that map to the five groups. Declaring from dimmi opens a
| public incident; the impact on customers is written to the numeric
| custom field the workspace picked, when it picked one.
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
IncidentIOAPI is incident.io's API.
*/
const IncidentIOAPI = "https://api.incident.io"

/*
ListFields is the list of an incident tool's custom fields.
*/
const ListFields = "fields"

/*
incidentRef matches the number of an incident URL.
*/
var incidentRef = regexp.MustCompile(`/incidents/([\w-]+)`)

/*
IncidentIO is the incident.io tool.
*/
type IncidentIO struct {
	c client
}

/*
NewIncidentIO returns the incident.io tool calling through the client.
*/
func NewIncidentIO(h *httpclient.Client) *IncidentIO {
	return &IncidentIO{c: client{http: h}}
}

/*
Provider names incident.io.
*/
func (*IncidentIO) Provider() string { return "incident_io" }

/*
Incidents reports true.
*/
func (*IncidentIO) Incidents() bool { return true }

/*
ioIncident is an incident as the API returns it.
*/
type ioIncident struct {
	ID        string `json:"id"`
	Reference string `json:"reference"`
	Name      string `json:"name"`
	Permalink string `json:"permalink"`
	Status    struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
	} `json:"incident_status"`
	Severity *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Rank int    `json:"rank"`
	} `json:"severity"`
}

/*
issue converts the incident.
*/
func (i ioIncident) issue() Issue {
	is := Issue{
		ExternalID: i.ID, Key: i.Reference, URL: i.Permalink, Title: i.Name, Status: i.Status.Name,
		StatusID: i.Status.ID, StatusGroup: IncidentIOGroup(i.Status.Category),
	}
	if i.Severity != nil {
		is.Severity, is.SeverityID, is.Rank = i.Severity.Name, i.Severity.ID, i.Severity.Rank
	}

	return is
}

/*
IncidentIOGroup returns the group of a status category.
*/
func IncidentIOGroup(category string) string {
	switch category {
	case "triage":
		return GroupBacklog
	case "live", "paused":
		return GroupStarted
	case "learning", "closed":
		return GroupDone
	case "declined", "merged", "canceled":
		return GroupCanceled
	}

	return GroupUnstarted
}

/*
call sends one request to the API path.
*/
func (t *IncidentIO) call(ctx context.Context, c Call, r request) error {
	r.url = IncidentIOAPI + r.url

	return t.c.do(ctx, map[string]string{"Authorization": "Bearer " + c.Token}, r)
}

/*
ioLists are the lists incident.io answers, as one shape.
*/
type ioLists struct {
	Severities []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Rank int    `json:"rank"`
	} `json:"severities"`
	Statuses []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
	} `json:"incident_statuses"`
	Fields []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		FieldType string `json:"field_type"`
	} `json:"custom_fields"`
}

/*
ioListPath is where each list is read.
*/
func ioListPath(list string) string {
	switch list {
	case ListSeverities:
		return "/v1/severities"
	case ListStates:
		return "/v1/incident_statuses"
	case ListFields:
		return "/v2/custom_fields"
	}

	return ""
}

/*
Options returns severities, statuses or numeric custom fields.
*/
func (t *IncidentIO) Options(ctx context.Context, c Call, list, _ string) ([]Option, error) {
	path := ioListPath(list)
	if path == "" {
		return nil, ErrUnsupported
	}

	var out ioLists
	if err := t.call(ctx, c, get(path, &out)); err != nil {
		return nil, err
	}

	var options []Option

	for _, s := range out.Severities {
		options = append(options, Option{ID: s.ID, Name: s.Name, Rank: s.Rank})
	}

	for _, s := range out.Statuses {
		options = append(options, Option{ID: s.ID, Name: s.Name, Group: IncidentIOGroup(s.Category)})
	}

	for _, f := range out.Fields {
		if f.FieldType == "numeric" {
			options = append(options, Option{ID: f.ID, Name: f.Name})
		}
	}

	return options, nil
}

/*
Search lists recent incidents whose name has the words.
*/
func (t *IncidentIO) Search(ctx context.Context, c Call, q string) ([]Issue, error) {
	var out struct {
		Incidents []ioIncident `json:"incidents"`
	}

	if err := t.call(ctx, c, get("/v2/incidents?page_size=50", &out)); err != nil {
		return nil, err
	}

	var issues []Issue

	for _, i := range out.Incidents {
		if q == "" || strings.Contains(strings.ToLower(i.Name+" "+i.Reference), strings.ToLower(q)) {
			issues = append(issues, i.issue())
		}
	}

	return issues, nil
}

/*
Read returns the incident an id, number or URL names.
*/
func (t *IncidentIO) Read(ctx context.Context, c Call, ref string) (Issue, error) {
	id := strings.TrimPrefix(strings.TrimSpace(ref), "INC-")
	if m := incidentRef.FindStringSubmatch(ref); m != nil {
		id = m[1]
	}

	var out struct {
		Incident ioIncident `json:"incident"`
	}

	err := t.call(ctx, c, get("/v2/incidents/"+url.PathEscape(id), &out))

	return out.Incident.issue(), err
}

/*
Create declares a public incident of the severity.
*/
func (t *IncidentIO) Create(ctx context.Context, c Call, in CreateInput) (Issue, error) {
	body := map[string]any{
		"idempotency_key": in.Impact.Link + "#" + in.Title, "name": in.Title, "visibility": "public",
		"summary": Description(in.Description, in.Impact), "mode": "standard",
	}
	if in.Severity != "" {
		body["severity_id"] = in.Severity
	}

	var out struct {
		Incident ioIncident `json:"incident"`
	}

	if err := t.call(ctx, c, post("/v2/incidents", body, &out)); err != nil {
		return Issue{}, err
	}

	is := out.Incident.issue()

	return is, t.Impact(ctx, c, is, in.Impact)
}

/*
Comment is not offered: incident.io keeps its timeline to itself.
*/
func (*IncidentIO) Comment(context.Context, Call, Issue, string) error {
	return ErrUnsupported
}

/*
Impact writes the customers reached to the picked numeric field.
*/
func (t *IncidentIO) Impact(ctx context.Context, c Call, is Issue, impact Impact) error {
	field := c.meta("impact_field")
	if field == "" {
		return nil
	}

	value := map[string]any{"value_numeric": formatInt(impact.Customers)}
	entry := map[string]any{"custom_field_id": field, "values": []any{value}}

	return t.call(ctx, c, request{
		method: fhttp.MethodPost, url: "/v2/incidents/" + url.PathEscape(is.ExternalID) + "/actions/edit",
		body: map[string]any{
			"notify_incident_channel": false, "incident": map[string]any{"custom_field_entries": []any{entry}},
		},
	})
}

/*
Verify checks the Svix signature.
*/
func (*IncidentIO) Verify(s Signed, secret string, now time.Time) error {
	return verifySvix(s, secret, now)
}

/*
Parse reads an incident.io delivery: a new or changed incident, or an
update posted on one.
*/
func (*IncidentIO) Parse(body []byte) ([]Event, error) {
	var raw map[string]any
	if err := decode(body, &raw); err != nil {
		return nil, err
	}

	kind, _ := raw["event_type"].(string)

	payload, ok := raw[kind].(map[string]any)
	if !ok {
		return nil, nil
	}

	if strings.Contains(kind, "incident_update_") {
		return []Event{ioUpdate(payload)}, nil
	}

	if inner, ok := payload["incident"].(map[string]any); ok {
		payload = inner
	}

	var i ioIncident
	if err := remarshal(payload, &i); err != nil || i.ID == "" {
		return nil, err
	}

	is := i.issue()

	event := Event{Kind: EventIssue, ExternalID: i.ID, Issue: &is}
	if strings.Contains(kind, "incident_created") {
		event.Kind = EventIncident
	}

	return []Event{event}, nil
}

/*
ioUpdate reads an update posted on an incident.
*/
func ioUpdate(payload map[string]any) Event {
	id, _ := payload["incident_id"].(string)
	message, _ := payload["message"].(string)

	author := ""
	if updater, ok := payload["updater"].(map[string]any); ok {
		if user, ok := updater["user"].(map[string]any); ok {
			author, _ = user["name"].(string)
		}
	}

	return Event{Kind: EventUpdate, ExternalID: id, Body: message, Author: author}
}
