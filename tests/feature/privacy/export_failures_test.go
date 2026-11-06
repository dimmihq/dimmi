package privacy_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/tests"
)

/*
exported asks for an export of a new customer, builds it, and returns
the export's path.
*/
func exported(t *testing.T, d tests.Desk) string {
	t.Helper()

	_, contact := customer(t, d)
	export := d.Owner.PostJSON("/api/v1/contacts/"+contact+"/exports", nil).AssertStatus(202).String("data.id")
	d.TC.Drain()

	return "/api/v1/contacts/" + contact + "/exports/" + export
}

/*
startExport asks for an export of the contact and returns its id.
*/
func startExport(d tests.Desk, contact string) string {
	return d.Owner.PostJSON("/api/v1/contacts/"+contact+"/exports", nil).AssertStatus(202).String("data.id")
}

/*
build runs the export's build now and returns its error.
*/
func build(t *testing.T, d tests.Desk, export string) error {
	t.Helper()

	id, err := d.TC.App.IDs.Decode("exp", export)
	require.NoError(t, err)

	return d.TC.App.Actions.Inbox.BuildContactExport.Handle(t.Context(), contacts.BuildExportArgs{
		WorkspaceID: d.TC.Workspace.Workspace.ID, ExportID: id,
	})
}

func TestContactExportFailures(t *testing.T) {
	t.Parallel()

	t.Run("fails exports while they cannot be read or written", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		show := exported(t, d)
		_, contact := customer(t, d)
		d.TC.BreakTable("contact_exports")

		d.Owner.PostJSON("/api/v1/contacts/"+contact+"/exports", nil).AssertStatus(500)
		d.Owner.GetJSON(show).AssertStatus(500)
		d.Owner.GetJSON(show + "/download").AssertStatus(500)
		_, err := d.TC.App.Actions.Inbox.ExpireContactExports.Handle(t.Context())
		require.Error(t, err)
		require.ErrorContains(t, build(t, d, show[strings.LastIndex(show, "/")+1:]), "start export")
	})

	t.Run("fails an expiry that cannot mark the export expired", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		exported(t, d)
		d.TC.Clock.Travel(25 * time.Hour)
		d.TC.FailWrites("contact_exports", "UPDATE")

		_, err := d.TC.App.Actions.Inbox.ExpireContactExports.Handle(t.Context())
		require.ErrorContains(t, err, "expire export")
	})

	for name, table := range map[string]string{
		"the contact":       "contacts",
		"the identities":    "contact_identities",
		"the conversations": "conversations",
		"the messages":      "messages",
		"the files":         "attachments",
	} {
		t.Run("fails the build when "+name+" cannot be read", func(t *testing.T) {
			t.Parallel()
			d := tests.NewDesk(t)
			path, contact := customer(t, d)
			storeAttachment(t, d.TC, path)
			exports := "/api/v1/contacts/" + contact + "/exports"
			export := d.Owner.PostJSON(exports, nil).AssertStatus(202).String("data.id")
			d.TC.BreakTable(table)

			require.Error(t, build(t, d, export))
		})
	}

	t.Run("fails the build when a file cannot be read from its disk", func(t *testing.T) {
		t.Parallel()

		for _, change := range []string{
			`UPDATE attachments SET disk = 'nowhere'`, `UPDATE attachments SET path = 'gone.txt'`,
		} {
			d := tests.NewDesk(t)
			path, contact := customer(t, d)
			storeAttachment(t, d.TC, path)
			_, err := d.TC.Operator().Exec(t.Context(), change)
			require.NoError(t, err)
			export := startExport(d, contact)

			require.Error(t, build(t, d, export))
			d.Owner.GetJSON("/api/v1/contacts/"+contact+"/exports/"+export).AssertOK().
				AssertJSONPath("data.status", "failed")
		}
	})

	t.Run("fails the build when the zip's disk is unknown", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		export := startExport(d, contact)
		id, err := d.TC.App.IDs.Decode("exp", export)
		require.NoError(t, err)
		misplaced := *d.TC.App.Actions.Inbox.BuildContactExport
		misplaced.DiskName = "nowhere"

		args := contacts.BuildExportArgs{WorkspaceID: d.TC.Workspace.Workspace.ID, ExportID: id}
		err = misplaced.Handle(t.Context(), args)
		require.ErrorContains(t, err, "open disk nowhere")
	})

	t.Run("fails a download whose zip is gone and an expiry that cannot delete it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		show := exported(t, d)
		d.TC.Clock.Travel(25 * time.Hour)
		expire := d.TC.App.Actions.Inbox.ExpireContactExports

		for _, change := range []string{
			`UPDATE contact_exports SET disk = 'nowhere'`, `UPDATE contact_exports SET path = '../outside.zip'`,
		} {
			_, err := d.TC.Operator().Exec(t.Context(), change)
			require.NoError(t, err)
			d.Owner.GetJSON(show + "/download").AssertStatus(500)
			_, err = expire.Handle(t.Context())
			require.Error(t, err)
		}
	})

	t.Run("expires through its job and keeps going while batches come back", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		show := exported(t, d)
		d.TC.Clock.Travel(25 * time.Hour)
		job := &jobs.ExpireContactExports{Expire: d.TC.App.Actions.Inbox.ExpireContactExports}

		assert.Equal(t, 3, jobs.ExpireContactExportsArgs{}.Options().MaxAttempts)
		require.NoError(t, job.Handle(t.Context(), nil))
		d.Owner.GetJSON(show).AssertOK().AssertJSONPath("data.status", "expired")
	})

	t.Run("builds an export whose requester left, naming automatic messages as the system", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path, contact := customer(t, d)
		conversation, err := d.TC.App.IDs.Decode("conv", path[len("/api/v1/conversations/"):])
		require.NoError(t, err)
		_, err = d.TC.Operator().Exec(t.Context(), `
			INSERT INTO messages (workspace_id, conversation_id, type, author_type, body)
			VALUES ($1, $2, 'message', 'agent', 'Automatic hello')`, d.TC.Workspace.Workspace.ID, conversation)
		require.NoError(t, err)
		export := startExport(d, contact)
		_, err = d.TC.Operator().Exec(t.Context(), `UPDATE contact_exports SET requested_by_member_id = NULL`)
		require.NoError(t, err)

		require.NoError(t, build(t, d, export))
		download := d.Owner.GetJSON("/api/v1/contacts/" + contact + "/exports/" + export + "/download").AssertOK()
		files := unzip(t, download.Body)
		assert.Contains(t, files["conversations/1.txt"], "System, ")
	})

	t.Run("refuses to export a customer being erased", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202)

		d.Owner.PostJSON("/api/v1/contacts/"+contact+"/exports", testkit.JSON{"include_notes": true}).AssertNotFound()
	})
}
