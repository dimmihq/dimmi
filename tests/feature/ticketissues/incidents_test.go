package ticketissues_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
connectIncidentIO connects incident.io with a key, pastes its signing
secret and turns the mirror on from major incidents.
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

	d.Owner.PatchJSON("/api/v1/ticket-settings", testkit.JSON{
		"incident_mirror": true, "incident_mirror_severity": "major",
	}).AssertOK()

	return installation
}

func TestIncidentTools(t *testing.T) {
	t.Parallel()

	t.Run("a major incident in incident.io is mirrored and followed to resolved", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := connectIncidentIO(t, d)

		d.incidentDelivery(t, installation, "incident_created.json").AssertOK()

		list := d.Owner.GetJSON("/api/v1/tickets?kind=incident").AssertOK()
		require.Equal(t, 1, count(list))
		incident := list.String("data.0.id")
		d.Owner.GetJSON("/api/v1/tickets/"+incident+"/issues").AssertOK().
			AssertJSONPath("data.0.key", "INC-77").AssertJSONPath("data.0.provider", "incident_io")

		d.incidentDelivery(t, installation, "incident_update.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+incident+"/public-updates").AssertOK().
			AssertJSONPath("data.0.source", "incident_io")

		d.incidentDelivery(t, installation, "incident_closed.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+incident).AssertOK().AssertJSONPath("data.state.group", "resolved")
		d.Owner.GetJSON("/api/v1/tickets/"+incident+"/update-drafts").AssertOK().
			AssertJSONPath("data.0.reason", "incident_resolved")
	})

	t.Run("an incident below the threshold is not mirrored", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := connectIncidentIO(t, d)

		d.incidentDelivery(t, installation, "incident_minor.json").AssertOK()
		require.Equal(t, 0, count(d.Owner.GetJSON("/api/v1/tickets?kind=incident").AssertOK()))
	})

	t.Run("a delivery signed with another secret is refused", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := connectIncidentIO(t, d)

		d.TC.Client().WithHeader("Webhook-Id", "x").WithHeader("Webhook-Timestamp", "1").
			WithHeader("Webhook-Signature", "v1,AAAA").
			Send("POST", "/webhooks/incident-io/"+installation, []byte(`{}`)).AssertStatus(401)
	})

	t.Run("severities feed the threshold picker", func(t *testing.T) {
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
