package ticketissues_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
connectIncidentIO connects incident.io with a key and pastes its
signing secret.
*/
func connectIncidentIO(t *testing.T, d desk) string {
	t.Helper()

	installation := d.Owner.PostJSON("/api/v1/integrations/incident_io/key-installations", testkit.JSON{
		"api_key": "inc_live_key_1234",
	}).AssertCreated().String("data.id")

	settings := d.Owner.PatchJSON("/api/v1/integration-installations/"+installation+"/issue-settings", testkit.JSON{
		"signing_secret": incidentSecret,
	}).AssertOK()
	settings.AssertJSONPath("data.has_signing_secret", true).
		AssertJSONPath("data.webhook_url", d.TC.App.Application.Config.App.URL+"/webhooks/incident-io/"+installation)

	return installation
}

/*
incident declares a dimmi incident by hand.
*/
func (d desk) incident(t *testing.T, title string) string {
	t.Helper()

	return d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{"type_id": d.types["Incident"], "title": title}).
		AssertCreated().String("data.id")
}

func TestIncidentTools(t *testing.T) {
	t.Parallel()

	t.Run("a new incident in incident.io opens nothing in dimmi", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := connectIncidentIO(t, d)

		d.incidentDelivery(t, installation, "incident_created.json").AssertOK()
		d.incidentDelivery(t, installation, "incident_update.json").AssertOK()
		d.incidentDelivery(t, installation, "incident_closed.json").AssertOK()

		require.Equal(t, 0, count(d.Owner.GetJSON("/api/v1/tickets?kind=incident").AssertOK()))
		require.Equal(t, 0, count(d.Owner.GetJSON("/api/v1/tickets").AssertOK()))
	})

	t.Run("a linked incident.io incident posts its updates and resolves the dimmi incident", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := connectIncidentIO(t, d)
		incident := d.incident(t, "Sign-in failing")

		d.Owner.PostJSON("/api/v1/tickets/"+incident+"/issues", testkit.JSON{
			"installation_id": installation, "key": "INC-77",
		}).AssertCreated().AssertJSONPath("data.key", "INC-77").AssertJSONPath("data.provider", "incident_io")

		d.incidentDelivery(t, installation, "incident_created.json").AssertOK()
		require.Equal(t, 1, count(d.Owner.GetJSON("/api/v1/tickets?kind=incident").AssertOK()))

		d.incidentDelivery(t, installation, "incident_update.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+incident+"/public-updates").AssertOK().
			AssertJSONPath("data.0.source", "incident_io")

		d.incidentDelivery(t, installation, "incident_closed.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+incident).AssertOK().AssertJSONPath("data.state.group", "resolved")
		d.Owner.GetJSON("/api/v1/tickets/"+incident+"/update-drafts").AssertOK().
			AssertJSONPath("data.0.reason", "incident_resolved").AssertJSONPath("data.0.decided_at", nil)
	})

	t.Run("a tracker linked to an incident.io incident closes the loop when it resolves", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := connectIncidentIO(t, d)
		tracker := d.tracker(t, "Cannot sign in", "ann")

		d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
			"installation_id": installation, "url": "https://app.incident.io/acme/incidents/77",
		}).AssertCreated()

		d.incidentDelivery(t, installation, "incident_closed.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+tracker).AssertOK().AssertJSONPath("data.state.name", "Close the loop")
	})

	t.Run("a delivery signed with another secret is refused", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := connectIncidentIO(t, d)

		d.TC.Client().WithHeader("Webhook-Id", "x").WithHeader("Webhook-Timestamp", "1").
			WithHeader("Webhook-Signature", "v1,AAAA").
			Send("POST", "/webhooks/incident-io/"+installation, []byte(`{}`)).AssertStatus(401)
	})

	t.Run("severities feed the severity picker", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := connectIncidentIO(t, d)

		d.Owner.GetJSON("/api/v1/integration-installations/"+installation+"/issue-options?list=severities").
			AssertOK().AssertJSONPath("data.2.name", "Critical").AssertJSONPath("data.2.rank", float64(3))
	})
}

func TestGitHubDeployCheck(t *testing.T) {
	t.Parallel()

	t.Run("a deployment of the watched repository marks the done fix shipped", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := d.connect(t, "github", "gho_"+"k123456789012345678901234567890123456")
		d.Owner.PatchJSON("/api/v1/integration-installations/"+installation+"/issue-settings", testkit.JSON{
			"deploy_repository": "acme/app", "deploy_environment": "production",
		}).AssertOK()

		tracker := d.tracker(t, "Invoices show the wrong tax", "cat")
		d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
			"url": "https://github.com/acme/app/issues/88",
		}).AssertCreated().AssertJSONPath("data.key", "acme/app#88")

		d.githubDelivery(t, installation, "webhook_issue_closed.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+tracker).AssertOK().AssertJSONPath("data.state.name", "Close the loop")
		d.Owner.GetJSON("/api/v1/tickets/"+tracker+"/issues").AssertOK().AssertJSONPath("data.0.deployed_at", nil)

		d.TC.Clock.Travel(1)
		d.githubDelivery(t, installation, "webhook_deployment_status.json").AssertOK()
		issues := d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/issues").AssertOK()
		require.NotEmpty(t, issues.String("data.0.deployed_at"))
	})
}
