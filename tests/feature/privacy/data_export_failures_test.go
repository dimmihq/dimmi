package privacy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/dataexports"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/tests"
)

/*
pendingExport asks for an export, a spreadsheet of every conversation
unless the body says otherwise, and returns what its build needs,
without building it.
*/
func pendingExport(t *testing.T, d tests.Desk, body testkit.JSON) dataexports.BuildArgs {
	t.Helper()

	if body["kind"] == nil {
		body["kind"], body["format"] = "conversations", "csv"
	}

	if body["kind"] == "conversations" {
		body["selection"] = testkit.JSON{"filter": testkit.JSON{"view": "all", "status": "any"}}
	}

	id := d.Owner.PostJSON(dataExports, body).AssertStatus(202).String("data.id")
	internal, err := d.TC.App.IDs.Decode("dex", id)
	require.NoError(t, err)

	return dataexports.BuildArgs{WorkspaceID: d.TC.Workspace.Workspace.ID, ExportID: internal}
}

func TestDataExportLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("removes the file once the workspace's export period runs out", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Customer(t, "ann", "Order")
		d.Owner.PatchJSON("/api/v1/workspace", testkit.JSON{"retention": testkit.JSON{"export_days": 2}}).AssertOK()
		export := exportAll(t, d, d.Owner, testkit.JSON{"format": "json"})
		show := dataExports + "/" + export.String("data.id")
		job := &jobs.ExpireDataExports{Expire: d.TC.App.Actions.Inbox.ExpireDataExports}

		d.TC.Clock.Travel(47 * time.Hour)
		require.NoError(t, job.Handle(t.Context(), nil))
		d.Owner.GetJSON(show).AssertOK().AssertJSONPath("data.status", "ready")

		d.TC.Clock.Travel(2 * time.Hour)
		require.NoError(t, job.Handle(t.Context(), nil))
		d.Owner.GetJSON(show).AssertOK().AssertJSONPath("data.status", "expired").AssertJSONPath("data.url", nil)
		d.Owner.GetJSON(show + "/download").AssertStatus(409)
		assert.Equal(t, 3, jobs.ExpireDataExportsArgs{}.Options().MaxAttempts)
		assert.Equal(t, 5, dataexports.BuildArgs{}.Options().MaxAttempts)
	})

	t.Run("keeps each step in the audit log", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		export := exportAll(t, d, d.Owner, testkit.JSON{"format": "csv"})
		d.Owner.Send("DELETE", dataExports+"/"+export.String("data.id"), nil).AssertNoContent()

		for _, event := range []string{"data_export.requested", "data_export.ready", "data_export.deleted"} {
			d.TC.Table("audit_logs").AssertCount(1, testkit.Row{"event": event})
		}
	})

	t.Run("leaves an export already built or gone alone", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		args := pendingExport(t, d, testkit.JSON{})
		d.TC.Drain()

		require.NoError(t, d.TC.App.Actions.Inbox.BuildDataExport.Handle(t.Context(), args))
		args.ExportID = args.WorkspaceID
		require.NoError(t, d.TC.App.Actions.Inbox.BuildDataExport.Handle(t.Context(), args))
	})
}

func TestDataExportFailures(t *testing.T) {
	t.Parallel()

	t.Run("fails every endpoint while exports cannot be read or written", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		show := dataExports + "/" + exportAll(t, d, d.Owner, testkit.JSON{"format": "csv"}).String("data.id")
		args := pendingExport(t, d, testkit.JSON{})
		d.TC.BreakTable("data_exports")

		d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "conversations", "format": "csv"}).AssertStatus(500)
		d.Owner.GetJSON(dataExports).AssertStatus(500)
		d.Owner.GetJSON(show).AssertStatus(500)
		d.Owner.GetJSON(show + "/download").AssertStatus(500)
		d.Owner.Send("DELETE", show, nil).AssertStatus(500)
		_, err := d.TC.App.Actions.Inbox.ExpireDataExports.Handle(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, d.TC.App.Actions.Inbox.BuildDataExport.Handle(t.Context(), args), "start export")
	})

	for name, table := range map[string]string{
		"the conversations":   "conversations",
		"the messages":        "messages",
		"the companies":       "companies",
		"the redaction rules": "redaction_rules",
		"the workspace":       "workspace_members",
		"the contacts":        "contacts",
	} {
		t.Run("fails the build when "+name+" cannot be read", func(t *testing.T) {
			t.Parallel()
			d := tests.NewDesk(t)
			path := start(t, d, "Hello", "Hi")
			d.Owner.PostJSON(path+"/notes", testkit.JSON{"body": "Noted"}).AssertCreated()
			_, err := d.TC.Operator().Exec(t.Context(), `
				INSERT INTO companies (workspace_id, name) SELECT id, 'Acme' FROM workspaces;
				UPDATE contacts SET company_id = (SELECT id FROM companies LIMIT 1)`)
			require.NoError(t, err)
			kind, format := "conversations", "json"
			if table == "contacts" {
				kind, format = "contacts", "csv"
			}

			args := pendingExport(t, d, testkit.JSON{"kind": kind, "format": format, "include_notes": false})
			d.TC.BreakTable(table)

			require.Error(t, d.TC.App.Actions.Inbox.BuildDataExport.Handle(t.Context(), args))
		})
	}

	t.Run("fails the build when the disk is unknown and expiry when a file cannot be removed", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		show := dataExports + "/" + exportAll(t, d, d.Owner, testkit.JSON{"format": "csv"}).String("data.id")
		args := pendingExport(t, d, testkit.JSON{})
		misplaced := *d.TC.App.Actions.Inbox.BuildDataExport
		misplaced.DiskName = "nowhere"

		require.ErrorContains(t, misplaced.Handle(t.Context(), args), "open disk nowhere")

		for _, change := range []string{
			`UPDATE data_exports SET path = '../outside.csv' WHERE status = 'ready'`,
			`UPDATE data_exports SET disk = 'nowhere' WHERE status = 'ready'`,
		} {
			_, err := d.TC.Operator().Exec(t.Context(), change)
			require.NoError(t, err)
			d.Owner.GetJSON(show + "/download").AssertStatus(500)
			d.Owner.Send("DELETE", show, nil).AssertStatus(500)
		}

		d.TC.Clock.Travel(8 * 24 * time.Hour)
		_, err := d.TC.App.Actions.Inbox.ExpireDataExports.Handle(t.Context())
		require.Error(t, err)
	})

	t.Run("fails a request while the rows cannot be counted", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Customer(t, "ann", "Order")
		ann := d.Owner.GetJSON("/api/v1/contacts").AssertOK().String("data.0.id")
		d.TC.BreakTable("contacts")
		d.TC.BreakTable("conversations")

		d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "conversations", "format": "csv"}).AssertStatus(500)
		d.Owner.PostJSON(dataExports, testkit.JSON{"kind": "contacts", "format": "csv"}).AssertStatus(500)
		d.Owner.PostJSON(dataExports, testkit.JSON{
			"kind": "contacts", "format": "csv", "selection": testkit.JSON{"ids": []string{ann}},
		}).AssertStatus(500)
	})

	t.Run("fails a build whose selection cannot be read and a delete that cannot be stored", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		people := pendingExport(t, d, testkit.JSON{"kind": "contacts", "format": "csv"})
		threads := pendingExport(t, d, testkit.JSON{})
		_, err := d.TC.Operator().Exec(t.Context(), `UPDATE data_exports SET selection = '[]'`)
		require.NoError(t, err)

		require.ErrorContains(t, d.TC.App.Actions.Inbox.BuildDataExport.Handle(t.Context(), people), "read selection")
		require.ErrorContains(t, d.TC.App.Actions.Inbox.BuildDataExport.Handle(t.Context(), threads), "read selection")

		show := dataExports + "/" + exportAll(t, d, d.Owner, testkit.JSON{"format": "csv"}).String("data.id")
		d.TC.FailWrites("data_exports", "DELETE")
		d.Owner.Send("DELETE", show, nil).AssertStatus(500)
	})

	t.Run("fails an expiry that cannot mark the export expired", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		exportAll(t, d, d.Owner, testkit.JSON{"format": "csv"})
		d.TC.Clock.Travel(8 * 24 * time.Hour)
		d.TC.FailWrites("data_exports", "UPDATE")

		_, err := d.TC.App.Actions.Inbox.ExpireDataExports.Handle(t.Context())
		require.ErrorContains(t, err, "expire export")
	})

	t.Run("fails the build for a requester who left and one who lost the right to notes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		gone := pendingExport(t, d, testkit.JSON{})
		noted := pendingExport(t, d, testkit.JSON{"kind": "conversations", "format": "json", "include_notes": true})
		_, err := d.TC.Operator().Exec(t.Context(),
			`UPDATE data_exports SET requested_by_member_id = NULL WHERE id = $1`, gone.ExportID)
		require.NoError(t, err)
		_, err = d.TC.Operator().Exec(t.Context(), `UPDATE workspace_members SET role = 'member'`)
		require.NoError(t, err)

		require.Error(t, d.TC.App.Actions.Inbox.BuildDataExport.Handle(t.Context(), gone))
		require.Error(t, d.TC.App.Actions.Inbox.BuildDataExport.Handle(t.Context(), noted))
	})
}
