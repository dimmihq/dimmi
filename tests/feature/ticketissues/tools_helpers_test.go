package ticketissues_test

import (
	"strconv"
	"testing"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/integrations/trackers"
	jirafixtures "github.com/dimmihq/dimmi/tests/fixtures/jira"
	pagerdutyfixtures "github.com/dimmihq/dimmi/tests/fixtures/pagerduty"
	rootlyfixtures "github.com/dimmihq/dimmi/tests/fixtures/rootly"
)

/*
jiraSite is the faked Jira site's REST address.
*/
const jiraSite = "https://api.atlassian.com/ex/jira/cloud-1/rest/api/3"

/*
serveJira answers site acme, issue PAY-12 and the calls linking it.
*/
func (d desk) serveJira(t *testing.T) {
	t.Helper()

	d.fake.Stub("GET https://api.atlassian.com/oauth/token/accessible-resources",
		reply(fixture(t, jirafixtures.Files, "accessible_resources.json")))
	d.fake.Stub("GET "+jiraSite+"/issue/PAY-12?fields=summary,status,project,resolution",
		reply(fixture(t, jirafixtures.Files, "issue.json")))
	d.fake.Stub("POST "+jiraSite+"/webhook", reply(fixture(t, jirafixtures.Files, "webhook_registered.json")))
	d.fake.Stub("POST "+jiraSite+"/issue/10001/remotelink", reply([]byte(`{"id": 1}`)))
	d.fake.Stub("PUT "+jiraSite+"/issue/10001/properties/dimmi.impact", reply(nil))
}

/*
serveKeyedTools answers the checks of a Rootly and a PagerDuty key.
*/
func (d desk) serveKeyedTools(t *testing.T) {
	t.Helper()

	d.fake.Stub("GET https://api.rootly.com/v1/users/me", reply(fixture(t, rootlyfixtures.Files, "users_me.json")))
	d.fake.Stub("GET https://api.pagerduty.com/abilities", reply(fixture(t, pagerdutyfixtures.Files, "abilities.json")))
}

/*
jiraDelivery signs a recorded Jira delivery with the app's secret.
*/
func (d desk) jiraDelivery(t *testing.T, name string) *testkit.Response {
	t.Helper()

	token := trackers.SignJWT(jiraSecret, map[string]any{"iss": "jira", "exp": d.TC.Clock.Now().Unix() + 60})

	return d.TC.Client().WithHeader("Authorization", "Bearer "+token).
		Send(fhttp.MethodPost, "/webhooks/jira", fixture(t, jirafixtures.Files, name))
}

/*
keyTool connects a tool with an API key, pastes its signing secret and
turns the incident mirror on from major incidents.
*/
func (d desk) keyTool(t *testing.T, provider string) string {
	t.Helper()

	installation := d.Owner.PostJSON("/api/v1/integrations/"+provider+"/key-installations", testkit.JSON{
		"api_key": provider + "_key_0001",
	}).AssertCreated().String("data.id")

	d.Owner.PatchJSON("/api/v1/integration-installations/"+installation+"/issue-settings", testkit.JSON{
		"signing_secret": toolSecret,
	}).AssertOK()
	d.Owner.PatchJSON("/api/v1/ticket-settings", testkit.JSON{"incident_mirror": true}).AssertOK()

	return installation
}

/*
pagerDutyDelivery signs a recorded PagerDuty delivery.
*/
func (d desk) pagerDutyDelivery(t *testing.T, installation, name string) *testkit.Response {
	t.Helper()

	body := fixture(t, pagerdutyfixtures.Files, name)

	return d.TC.Client().WithHeader("X-PagerDuty-Signature", "v1="+trackers.Sign(toolSecret, body)).
		Send(fhttp.MethodPost, "/webhooks/pagerduty/"+installation, body)
}

/*
rootlyDelivery signs a recorded Rootly delivery stamped now.
*/
func (d desk) rootlyDelivery(t *testing.T, installation, name string) *testkit.Response {
	t.Helper()

	body := fixture(t, rootlyfixtures.Files, name)
	ts := strconv.FormatInt(d.TC.Clock.Now().Unix(), 10)

	return d.TC.Client().WithHeader("X-Rootly-Signature", trackers.RootlySignature(toolSecret, ts, body)).
		Send(fhttp.MethodPost, "/webhooks/rootly/"+installation, body)
}
