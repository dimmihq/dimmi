package ticketissues_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	rootlyfixtures "github.com/dimmihq/dimmi/tests/fixtures/rootly"
)

/*
rootlyIncident is the Rootly incident the fixtures record.
*/
const rootlyIncident = "https://api.rootly.com/v1/incidents/fe952192-b309-438e-a112-4c0fdd21ec36"

/*
serveRootly answers Rootly's services, a search, incident #13 and
changes to it.
*/
func (d desk) serveRootly(t *testing.T) {
	t.Helper()

	d.fake.Stub("GET https://api.rootly.com/v1/services*", reply(fixture(t, rootlyfixtures.Files, "services.json")))
	d.fake.Stub("GET https://api.rootly.com/v1/incidents?*", reply(fixture(t, rootlyfixtures.Files, "incidents.json")))
	d.fake.Stub("GET "+rootlyIncident, reply(fixture(t, rootlyfixtures.Files, "incident.json")))
	d.fake.Stub("PUT "+rootlyIncident, reply(fixture(t, rootlyfixtures.Files, "incident.json")))
	d.fake.Stub("POST https://api.rootly.com/v1/incidents", reply(fixture(t, rootlyfixtures.Files, "incident.json")))
}

/*
sentTo returns the bodies of the requests sent with the method to the
address, oldest first.
*/
func (d desk) sentTo(method, address string) []httpclienttest.SentRequest {
	var out []httpclienttest.SentRequest

	for _, r := range d.fake.Sent() {
		if r.Method == method && r.URL.String() == address {
			out = append(out, r)
		}
	}

	return out
}

/*
labelsOf reads the labels a Rootly update sent.
*/
func labelsOf(t *testing.T, r httpclienttest.SentRequest) map[string]any {
	t.Helper()

	var body struct {
		Data struct {
			Attributes struct {
				Labels map[string]any `json:"labels"`
			} `json:"attributes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(r.Body, &body))

	return body.Data.Attributes.Labels
}

func TestRootlyIncidents(t *testing.T) {
	t.Parallel()

	t.Run("declaring opens an incident on the picked service with the impact in its labels", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		d.serveRootly(t)
		installation := d.keyTool(t, "rootly")
		d.Owner.PatchJSON("/api/v1/integration-installations/"+installation+"/issue-settings", testkit.JSON{
			"service_id": "368b88bc-e307-4f63-b8c7-2a267e8dd091",
		}).AssertOK()

		d.Owner.GetJSON("/api/v1/integration-installations/"+installation+"/issue-options?list=services").
			AssertOK().AssertJSONPath("data.0.name", "Transcription & Summary Pipeline")

		tracker := d.tracker(t, "Webhook signature never matches", "ann")
		d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
			"installation_id": installation, "severity_id": "462e5b20-06a0-44a3-974e-fcbad9f02b50",
		}).AssertCreated().AssertJSONPath("data.key", "#13").AssertJSONPath("data.status", "Started")

		created := d.sentTo(fhttp.MethodPost, "https://api.rootly.com/v1/incidents")
		require.Len(t, created, 1)
		assert.Equal(t, "application/vnd.api+json", created[0].Header.Get("Content-Type"))
		assert.Contains(t, string(created[0].Body), `"service_ids":["368b88bc-e307-4f63-b8c7-2a267e8dd091"]`)

		updates := d.sentTo(fhttp.MethodPut, rootlyIncident)
		require.NotEmpty(t, updates)
		labels := labelsOf(t, updates[len(updates)-1])
		assert.Equal(t, "payments", labels["team"])
		assert.Equal(t, "1", labels["dimmi_customers"])
		assert.Contains(t, labels["dimmi_ticket"], "/tickets/"+tracker)
	})

	t.Run("a linked incident moves the ticket, and a note on its timeline becomes a note", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		d.serveRootly(t)
		installation := d.keyTool(t, "rootly")
		conv := d.Customer(t, "ann", "Webhook signature never matches").String("data.id")
		ticket := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": d.types["Tracker"], "title": "Webhook signature never matches", "conversation_id": conv,
		}).AssertCreated().String("data.id")

		d.Owner.PostJSON("/api/v1/tickets/"+ticket+"/issues", testkit.JSON{
			"installation_id": installation, "url": "https://rootly.com/account/incidents/fe952192-b309-438e-a112-4c0fdd21ec36",
		}).AssertCreated()

		d.rootlyDelivery(t, installation, "webhook_incident_mitigated.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+ticket+"/issues").AssertOK().AssertJSONPath("data.0.status", "Mitigated")
		d.Owner.GetJSON("/api/v1/tickets/"+ticket).AssertOK().AssertJSONPath("data.state.name", "Being worked on")

		d.rootlyDelivery(t, installation, "webhook_incident_event_trail.json").AssertOK()
		d.rootlyDelivery(t, installation, "webhook_incident_event_created_note.json").AssertOK()
		d.rootlyDelivery(t, installation, "webhook_incident_event_note.json").AssertOK()

		events := string(d.Owner.GetJSON("/api/v1/tickets/" + ticket + "/events").AssertOK().Body)
		assert.Contains(t, events, "Comment on Rootly #13: [dimmi dev test] Root cause")
		assert.NotContains(t, events, "has been set")
		assert.NotContains(t, events, "created this incident")

		messages := string(d.Owner.GetJSON("/api/v1/conversations/" + conv + "/messages").AssertOK().Body)
		assert.Contains(t, messages, `"name":"Rootly"`)
	})

	t.Run("search matches words in the title and unlinking takes dimmi's labels off", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		d.serveRootly(t)
		installation := d.keyTool(t, "rootly")

		found := d.Owner.GetJSON("/api/v1/integration-installations/" + installation + "/issues?q=meeting").AssertOK()
		require.Equal(t, 1, count(found))
		found.AssertJSONPath("data.0.key", "#12")

		tracker := d.tracker(t, "Webhook signature never matches", "ann")
		link := d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
			"installation_id": installation, "key": "fe952192-b309-438e-a112-4c0fdd21ec36",
		}).AssertCreated().String("data.id")

		d.Owner.Delete("/api/v1/ticket-issues/" + link + "?remove_from_tool=true").AssertNoContent()

		updates := d.sentTo(fhttp.MethodPut, rootlyIncident)
		require.NotEmpty(t, updates)
		labels := labelsOf(t, updates[len(updates)-1])
		assert.Equal(t, map[string]any{"team": "payments"}, labels)
	})
}

func TestIncidentIOImpact(t *testing.T) {
	t.Parallel()

	const field = "01FIELD"

	edits := func(d desk) []string {
		var out []string
		for _, r := range d.sentTo(fhttp.MethodPost, "https://api.incident.io/v2/incidents/inc-77/actions/edit") {
			out = append(out, string(r.Body))
		}

		return out
	}

	link := func(t *testing.T, d desk, installation, title string) string {
		t.Helper()

		tracker := d.tracker(t, title, "ann")

		return d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
			"installation_id": installation, "url": "https://app.incident.io/acme/incidents/77",
		}).AssertCreated().String("data.id")
	}

	t.Run("an update shared with a status change becomes a note", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := connectIncidentIO(t, d)
		tracker := d.tracker(t, "Cannot sign in", "ann")
		d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
			"installation_id": installation, "url": "https://app.incident.io/acme/incidents/77",
		}).AssertCreated()

		d.incidentDelivery(t, installation, "incident_status_updated.json").AssertOK()

		d.Owner.GetJSON("/api/v1/tickets/"+tracker+"/issues").AssertOK().AssertJSONPath("data.0.status", "Monitoring")
		events := string(d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/events").AssertOK().Body)
		assert.Contains(t, events, "Update on incident.io INC-77: We rolled back the bad deploy")
	})

	t.Run("unlinking clears the customer count, unless another ticket still links it", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		d.fake.Stub("POST https://api.incident.io/v2/incidents/inc-77/actions/edit", reply([]byte(`{}`)))
		installation := connectIncidentIO(t, d)
		d.Owner.PatchJSON("/api/v1/integration-installations/"+installation+"/issue-settings", testkit.JSON{
			"impact_field_id": field,
		}).AssertOK()

		first := link(t, d, installation, "Cannot sign in")
		second := link(t, d, installation, "Sign-in loops")
		written := len(edits(d))

		d.Owner.Delete("/api/v1/ticket-issues/" + first + "?remove_from_tool=true").AssertNoContent()
		sent := edits(d)
		require.Greater(t, len(sent), written)
		assert.Contains(t, sent[len(sent)-1], `"value_numeric":"1"`)

		d.Owner.Delete("/api/v1/ticket-issues/" + second + "?remove_from_tool=true").AssertNoContent()
		sent = edits(d)
		last := sent[len(sent)-1]
		assert.True(t, strings.Contains(last, `"values":[]`), last)
	})
}
