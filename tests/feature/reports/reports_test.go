package reports_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
desk is a workspace with conversations to report on.
*/
type desk struct {
	tests.Desk
	t *testing.T
}

func open(t *testing.T) desk {
	t.Helper()

	return desk{Desk: tests.NewDesk(t), t: t}
}

/*
refresh works out every changed conversation's facts at once.
*/
func (d desk) refresh() {
	d.t.Helper()
	far := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

	for range 10 {
		n, err := d.TC.App.Actions.ReportMaintenance.Drain(d.t.Context(), far)
		require.NoError(d.t, err)

		if n == 0 {
			return
		}
	}
}

/*
customer starts a conversation and returns its path.
*/
func (d desk) customer(name string) string {
	d.t.Helper()

	return "/api/v1/conversations/" + d.Customer(d.t, name, "Help").String("data.id")
}

/*
metric returns a report number's value.
*/
func metric(t *testing.T, res *testkit.Response, key string) any {
	t.Helper()

	var body struct {
		Data struct {
			Metrics []struct {
				Key    string   `json:"key"`
				Value  *float64 `json:"value"`
				Sample int64    `json:"sample"`
			} `json:"metrics"`
		} `json:"data"`
	}
	res.Decode(&body)

	for _, m := range body.Data.Metrics {
		if m.Key == key {
			if m.Value == nil {
				return nil
			}

			return *m.Value
		}
	}

	t.Fatalf("no metric %s in %s", key, res.Body)

	return nil
}

func TestReportFacts(t *testing.T) {
	t.Parallel()

	t.Run("a reply, a close, a reopen and a second close count once each", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := d.customer("ana")

		d.TC.Clock.Travel(10 * time.Minute)
		d.Owner.PostJSON(path+"/replies", testkit.JSON{"body": "On it"}).AssertCreated()
		d.Owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK()
		d.TC.Clock.Travel(time.Hour)
		d.Owner.PatchJSON(path, testkit.JSON{"status": "open"}).AssertOK()
		d.Owner.PostJSON(path+"/replies", testkit.JSON{"body": "Fixed now"}).AssertCreated()
		d.Owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK()
		d.refresh()

		res := d.Owner.GetJSON("/api/v1/reports/overview").AssertOK()
		assert.InDelta(t, 1.0, metric(t, res, "new_conversations"), 0)
		assert.InDelta(t, 1.0, metric(t, res, "closed"), 0)
		assert.InDelta(t, 600.0, metric(t, res, "first_response"), 1)
		assert.InDelta(t, 0.0, metric(t, res, "first_contact_resolution"), 0)

		res = d.Owner.GetJSON("/api/v1/reports/resolution").AssertOK()
		assert.InDelta(t, 100.0, metric(t, res, "reopen_rate"), 0)
		assert.InDelta(t, 4200.0, metric(t, res, "resolution"), 1)
		assert.InDelta(t, 600.0, metric(t, res, "first_close"), 1)

		drill := d.Owner.GetJSON("/api/v1/reports/resolution/conversations?metric=reopen_rate").AssertOK()
		drill.AssertJSONPath("total", float64(1)).AssertJSONPath("data.0.status", "closed")

		d.Owner.GetJSON("/api/v1/reports/resolution/conversations?metric=nope").
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})

	t.Run("an AI hand-off and its return are logged", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := d.customer("bo")
		id := d.Owner.GetJSON(path).AssertOK().String("data.id")

		_, err := d.TC.Operator().Exec(t.Context(), `UPDATE conversations SET ai_state = 'handed_off',
			ai_handed_off_at = now(), ai_handoff_reason = 'asked_for_human' WHERE workspace_id = $1`,
			d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)
		_, err = d.TC.Operator().Exec(t.Context(),
			`UPDATE conversations SET ai_state = 'active' WHERE workspace_id = $1`, d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)

		kinds := kindsOf(t, d, id)
		assert.Contains(t, kinds, "ai_handed_off:live")
		assert.Contains(t, kinds, "ai_resumed:live")
	})

	t.Run("an import is logged at its own times and tagged as imported", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		start := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
		id := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.InboxID(), "author": "contact", "subject": "Old", "body": "Still waiting",
			"contact": testkit.JSON{"email": "old@example.com"}, "created_at": start.Format(time.RFC3339),
			"status": "closed", "closed_at": start.Add(3 * time.Hour).Format(time.RFC3339),
		}).AssertCreated().String("data.id")
		d.refresh()

		assert.Equal(t, []string{"created:import", "closed:import"}, kindsOf(t, d, id))

		res := d.Owner.GetJSON("/api/v1/reports/resolution?from=2026-09-01&to=2026-09-01").AssertOK()
		assert.InDelta(t, 1.0, metric(t, res, "closed"), 0)
		assert.InDelta(t, 3*3600.0, metric(t, res, "resolution"), 1)
	})

	t.Run("a merged-away conversation leaves the reports", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		first := d.customer("cy")
		target := d.Owner.GetJSON(first).AssertOK().String("data.id")
		d.Owner.PatchJSON(first, testkit.JSON{"status": "closed"}).AssertOK()
		source := d.Owner.GetJSON(d.customer("cy")).AssertOK().String("data.id")
		require.NotEqual(t, target, source)
		sourceNumber := numberOf(t, d, source)
		d.refresh()

		res := d.Owner.GetJSON("/api/v1/reports/overview").AssertOK()
		assert.InDelta(t, 2.0, metric(t, res, "new_conversations"), 0)

		d.Owner.PostJSON("/api/v1/conversations/"+target+"/merges", testkit.JSON{"source_ids": []string{source}}).
			AssertOK()
		d.refresh()

		res = d.Owner.GetJSON("/api/v1/reports/overview").AssertOK()
		assert.InDelta(t, 1.0, metric(t, res, "new_conversations"), 0)
		assert.Contains(t, kindsByNumber(t, d, sourceNumber), "merged:live")
	})
}

func TestReportBackfill(t *testing.T) {
	t.Parallel()

	t.Run("history from before the log is rebuilt from the timeline", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := d.customer("gus")
		id := d.Owner.GetJSON(path).AssertOK().String("data.id")
		for _, step := range []func(){
			func() { d.Owner.PostJSON(path+"/replies", testkit.JSON{"body": "Done"}).AssertCreated() },
			func() { d.Owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK() },
			func() { d.Owner.PatchJSON(path, testkit.JSON{"status": "open"}).AssertOK() },
			func() { d.Owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK() },
		} {
			d.TC.Clock.Travel(time.Minute)
			step()
		}

		ws := d.TC.Workspace.Workspace.ID
		for _, stmt := range []string{
			`DELETE FROM conversation_events WHERE workspace_id = $1`,
			`DELETE FROM conversation_facts WHERE workspace_id = $1`,
			`DELETE FROM report_pending WHERE workspace_id = $1`,
			`INSERT INTO report_states (workspace_id, log_started_at) VALUES ($1, now() + interval '1 day')
			 ON CONFLICT (workspace_id) DO UPDATE SET log_started_at = now() + interval '1 day', backfilled_at = NULL`,
		} {
			_, err := d.TC.Operator().Exec(t.Context(), stmt, ws)
			require.NoError(t, err)
		}

		require.NoError(t, d.TC.App.Actions.ReportMaintenance.RebuildNow(t.Context(), ws))

		assert.Equal(t, []string{
			"created:backfill", "assigned:backfill", "team_assigned:backfill", "closed:backfill",
			"reopened:backfill", "closed:backfill",
		}, kindsOf(t, d, id), "the reply assigned the conversation to its author")

		res := d.Owner.GetJSON("/api/v1/reports/resolution").AssertOK()
		assert.InDelta(t, 1.0, metric(t, res, "closed"), 0)
		assert.InDelta(t, 100.0, metric(t, res, "reopen_rate"), 0)
	})
}

func TestReportAccess(t *testing.T) {
	t.Parallel()

	t.Run("a teammate who sees only their own conversations counts only those", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.customer("dee")
		d.customer("eve")
		d.refresh()

		member, m := d.Join(t, enums.RoleMember)
		_, err := d.TC.Operator().Exec(t.Context(),
			`UPDATE workspace_members SET conversation_access = 'assigned' WHERE id = $1`, m.ID)
		require.NoError(t, err)

		res := member.GetJSON("/api/v1/reports/overview").AssertOK()
		assert.InDelta(t, 0.0, metric(t, res, "new_conversations"), 0)

		res = d.Owner.GetJSON("/api/v1/reports/overview?compare=true").AssertOK()
		assert.InDelta(t, 2.0, metric(t, res, "new_conversations"), 0)
		assert.NotEmpty(t, res.String("data.previous_from"))
	})

	t.Run("lists the reports and refuses an unknown one", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.Owner.GetJSON("/api/v1/reports").AssertOK().AssertJSONPath("data.0.key", "overview")
		d.Owner.GetJSON("/api/v1/reports/nope").AssertNotFound()
		d.Owner.GetJSON("/api/v1/reports/overview?from=2026-11-10&to=2026-11-01").
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})

	t.Run("every report answers", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.customer("fay")
		d.refresh()

		for _, key := range []string{
			"overview", "responsiveness", "resolution", "team", "live", "satisfaction", "ai", "help_center",
		} {
			d.Owner.GetJSON("/api/v1/reports/"+key+"?compare=true&business_hours=true").AssertOK().
				AssertJSONPath("data.key", key)
		}
	})
}

func TestReportFactsExport(t *testing.T) {
	t.Parallel()

	t.Run("every conversation's facts leave as one spreadsheet", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := d.customer("hal")
		d.TC.Clock.Travel(5 * time.Minute)
		d.Owner.PostJSON(path+"/replies", testkit.JSON{"body": "Here"}).AssertCreated()
		d.refresh()

		id := d.Owner.PostJSON("/api/v1/data-exports", testkit.JSON{
			"kind": "conversation_facts", "format": "csv", "selection": testkit.JSON{"filter": testkit.JSON{}},
		}).AssertStatus(fhttp.StatusAccepted).String("data.id")
		d.TC.Drain()

		export := d.Owner.GetJSON("/api/v1/data-exports/"+id).AssertOK().AssertJSONPath("data.status", "ready").
			AssertJSONPath("data.row_count", float64(1))
		file := string(d.Owner.GetJSON(export.String("data.url")).AssertOK().Body)

		assert.True(t, strings.HasPrefix(file, "conversation_id,number,inbox_id,channel,status"), file)
		assert.Contains(t, file, ",300,", "the first response took five minutes")

		d.Owner.PostJSON("/api/v1/data-exports", testkit.JSON{"kind": "conversation_facts", "format": "json"}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})
}

func TestReportDigest(t *testing.T) {
	t.Parallel()

	t.Run("a teammate subscribes, changes and stops their digest", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.Owner.GetJSON("/api/v1/report-digest").AssertOK().AssertJSONPath("data.frequency", nil)
		d.Owner.PatchJSON("/api/v1/report-digest", testkit.JSON{"business_hours": true}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
		d.Owner.PatchJSON("/api/v1/report-digest", testkit.JSON{"frequency": "weekly"}).AssertOK().
			AssertJSONPath("data.frequency", "weekly")
		d.Owner.PatchJSON("/api/v1/report-digest", testkit.JSON{"business_hours": true}).AssertOK().
			AssertJSONPath("data.frequency", "weekly").AssertJSONPath("data.business_hours", true)
		d.Owner.PatchJSON("/api/v1/report-digest", testkit.JSON{"frequency": "daily"}).AssertOK().
			AssertJSONPath("data.frequency", "daily").AssertJSONPath("data.business_hours", true)
		d.Owner.Delete("/api/v1/report-digest").AssertNoContent()
		d.Owner.GetJSON("/api/v1/report-digest").AssertOK().AssertJSONPath("data.frequency", nil)
	})

	t.Run("a due digest goes out and moves on", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.Owner.PatchJSON("/api/v1/report-digest", testkit.JSON{"frequency": "daily"}).AssertOK()

		d.TC.Clock.Travel(25 * time.Hour)
		require.NoError(t, d.TC.App.Actions.ReportDigests.SendDigests(t.Context()))

		res := d.Owner.GetJSON("/api/v1/report-digest").AssertOK()
		assert.NotEmpty(t, res.String("data.last_sent_at"))
	})
}

/*
kindsOf returns the logged kinds and sources of the conversation named
by its public id.
*/
func kindsOf(t *testing.T, d desk, public string) []string {
	t.Helper()

	return kindsByNumber(t, d, numberOf(t, d, public))
}

/*
numberOf returns the number of the conversation named by its public id.
*/
func numberOf(t *testing.T, d desk, public string) int64 {
	t.Helper()

	var body struct {
		Data struct {
			Number int64 `json:"number"`
		} `json:"data"`
	}
	d.Owner.GetJSON("/api/v1/conversations/" + public).AssertOK().Decode(&body)

	return body.Data.Number
}

/*
kindsByNumber returns the logged kinds and sources of the conversation
with the number.
*/
func kindsByNumber(t *testing.T, d desk, number int64) []string {
	t.Helper()

	rows, err := d.TC.Operator().Query(t.Context(), `
		SELECT e.kind || ':' || e.source FROM conversation_events e
		JOIN conversations c ON c.id = e.conversation_id
		WHERE c.workspace_id = $1 AND c.number = $2
		ORDER BY e.occurred_at, e.id`, d.TC.Workspace.Workspace.ID, number)
	require.NoError(t, err)
	defer rows.Close()

	var out []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		out = append(out, s)
	}

	return out
}
