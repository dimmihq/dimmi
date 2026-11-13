package ticketissues_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestJiraIssues(t *testing.T) {
	t.Parallel()

	t.Run("a linked issue follows Jira to close the loop, or won't fix", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		d.connect(t, "jira", "jira_oauth_token")
		tracker := d.tracker(t, "Refunds fail", "ann")

		d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
			"url": "https://acme.atlassian.net/browse/PAY-12",
		}).AssertCreated().AssertJSONPath("data.url", "https://acme.atlassian.net/browse/PAY-12")

		d.jiraDelivery(t, "webhook_issue_done.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+tracker).AssertOK().AssertJSONPath("data.state.name", "Close the loop")

		d.jiraDelivery(t, "webhook_issue_wont_do.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+tracker).AssertOK().AssertJSONPath("data.state.name", "Won't fix")
	})

	t.Run("a delivery without the app's token is refused", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		d.TC.Client().Send("POST", "/webhooks/jira", []byte(`{}`)).AssertStatus(401)
	})
}

func TestPagerDutyAndRootly(t *testing.T) {
	t.Parallel()

	t.Run("a tracker linked to a PagerDuty incident follows it and closes the loop", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := d.keyTool(t, "pagerduty")

		d.pagerDutyDelivery(t, installation, "webhook_triggered.json").AssertOK()
		require.Equal(t, 0, count(d.Owner.GetJSON("/api/v1/tickets").AssertOK()))

		tracker := d.tracker(t, "Checkout failing")
		d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
			"installation_id": installation, "key": "PD123",
		}).AssertCreated().AssertJSONPath("data.key", "#501")

		d.pagerDutyDelivery(t, installation, "webhook_acknowledged.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+tracker).AssertOK().AssertJSONPath("data.state.name", "Being worked on")

		d.pagerDutyDelivery(t, installation, "webhook_resolved.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+tracker).AssertOK().AssertJSONPath("data.state.name", "Close the loop")
	})

	t.Run("a new Rootly incident opens nothing in dimmi", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := d.keyTool(t, "rootly")

		d.rootlyDelivery(t, installation, "webhook_incident_created.json").AssertOK()
		require.Equal(t, 0, count(d.Owner.GetJSON("/api/v1/tickets").AssertOK()))
	})
}
