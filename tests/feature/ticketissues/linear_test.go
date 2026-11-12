package ticketissues_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

/*
linkLinear connects Linear and creates ENG-412 on a new tracker that
reaches two customers, returning the tracker.
*/
func linkLinear(t *testing.T, d desk) string {
	t.Helper()

	installation := d.connect(t, "linear", "lin_oauth_token")
	tracker := d.tracker(t, "CSV export crashes", "ann", "ben")

	d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
		"installation_id": installation, "team_id": "team-eng", "title": "CSV export crashes",
	}).AssertCreated().AssertJSONPath("data.key", "ENG-412")

	return tracker
}

func TestLinearIssues(t *testing.T) {
	t.Parallel()

	t.Run("the pickers read teams and states from Linear", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		installation := d.connect(t, "linear", "lin_oauth_token")

		teams := d.Owner.GetJSON("/api/v1/integration-installations/" + installation + "/issue-teams").AssertOK()
		teams.AssertJSONPath("data.0.id", "team-eng").AssertJSONPath("data.0.name", "Engineering")

		states := d.Owner.GetJSON("/api/v1/integration-installations/" + installation +
			"/issue-options?list=states&parent=team-eng").AssertOK()
		states.AssertJSONPath("data.1.name", "In Progress").AssertJSONPath("data.1.group", "started")
	})

	t.Run("a new issue carries the customer impact and a link back", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		tracker := linkLinear(t, d)

		var create string
		for _, q := range d.linearSent() {
			if strings.Contains(q, "issueCreate") {
				create = q
			}
		}

		assert.Contains(t, create, "Customer impact: 2 customers")
		assert.Contains(t, create, "/tickets/"+tracker)

		list := d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/issues").AssertOK()
		list.AssertJSONPath("data.0.provider", "linear").AssertJSONPath("data.0.status_group", "backlog")
		d.Owner.GetJSON("/api/v1/tickets/"+tracker).AssertOK().AssertJSONPath("data.issues.0.key", "ENG-412")
	})

	t.Run("an issue is linked by its pasted URL", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		d.connect(t, "linear", "lin_oauth_token")
		tracker := d.tracker(t, "CSV export crashes")

		d.Owner.PostJSON("/api/v1/tickets/"+tracker+"/issues", testkit.JSON{
			"url": "https://linear.app/acme/issue/ENG-412/csv-export-crashes",
		}).AssertCreated().AssertJSONPath("data.title", "CSV export crashes")
	})

	t.Run("a signed delivery moves the tracker to close the loop without messaging anyone", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		tracker := linkLinear(t, d)
		changes := tests.Record[appevents.TicketIssueUpdated](d.TC)

		d.linearDelivery(t, "webhook_issue_started.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+tracker).AssertOK().AssertJSONPath("data.state.name", "Being worked on")

		d.linearDelivery(t, "webhook_issue_done.json").AssertOK()
		d.Owner.GetJSON("/api/v1/tickets/"+tracker).AssertOK().AssertJSONPath("data.state.name", "Close the loop").
			AssertJSONPath("data.close_loop", true)

		require.Equal(t, 0, count(d.Owner.GetJSON("/api/v1/tickets/"+tracker+"/update-drafts").AssertOK()))
		require.Equal(t, 0, count(d.Owner.GetJSON("/api/v1/tickets/"+tracker+"/updates").AssertOK()))

		seen := changes.All()
		require.NotEmpty(t, seen)
		assert.Equal(t, "done", seen[len(seen)-1].Changes["status_group"].To)
		assert.Equal(t, "started", seen[len(seen)-1].Changes["status_group"].From)
	})

	t.Run("an unsigned delivery is refused", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		d.TC.Client().WithHeader("Linear-Signature", "00").
			Send("POST", "/webhooks/linear", []byte(`{"type":"Issue","webhookTimestamp":1}`)).AssertStatus(401)
	})

	t.Run("comments become notes and @customers drafts a message", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		tracker := linkLinear(t, d)

		d.linearDelivery(t, "webhook_comment.json").AssertOK()

		events := d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/events").AssertOK()
		assert.Contains(t, string(events.Body), "Priya commented on Linear ENG-412")

		drafts := d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/update-drafts").AssertOK()
		drafts.AssertJSONPath("data.0.reason", "comment").AssertJSONPath("data.0.status", "ready").
			AssertJSONPath("data.0.recipients", float64(2)).
			AssertJSONPath("data.0.body", "The export works again for files of any size.")

		d.Owner.PatchJSON("/api/v1/ticket-update-drafts/"+drafts.String("data.0.id"), testkit.JSON{"status": "sent"}).
			AssertOK().AssertJSONPath("data.status", "sent")
		d.TC.Drain()
		d.Owner.GetJSON("/api/v1/tickets/"+tracker+"/updates").AssertOK().AssertJSONPath("data.0.delivered", float64(2))
	})

	t.Run("the re-check warns about a team the app cannot see", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		tracker := linkLinear(t, d)

		d.mu.Lock()
		*d.hidden = true
		d.mu.Unlock()

		d.TC.Clock.Travel(16 * time.Minute)
		require.NoError(t, d.TC.App.Actions.TicketIssues.Sweep(t.Context()))

		list := d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/issues").AssertOK()
		assert.Contains(t, list.String("data.0.warning"), "no access to Engineering")
	})

	t.Run("unlinking keeps the issue and removes the link", func(t *testing.T) {
		t.Parallel()

		d := newDesk(t)
		tracker := linkLinear(t, d)
		link := d.Owner.GetJSON("/api/v1/tickets/" + tracker + "/issues").AssertOK().String("data.0.id")

		d.Owner.Delete("/api/v1/ticket-issues/" + link).AssertNoContent()
		require.Equal(t, 0, count(d.Owner.GetJSON("/api/v1/tickets/"+tracker+"/issues").AssertOK()))
	})
}
