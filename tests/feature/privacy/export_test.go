package privacy_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/tests"
)

/*
surveyExporter is a part of the product that adds the customer's survey
answers to their export, failing while told to.
*/
type surveyExporter struct {
	fail bool
}

/*
Name names the section.
*/
func (surveyExporter) Name() string { return "surveys" }

/*
Export returns one answer and a file, or fails.
*/
func (e surveyExporter) Export(context.Context, ids.UUID, ids.UUID) (contracts.ExportSection, error) {
	if e.fail {
		return contracts.ExportSection{}, errors.New("surveys unavailable")
	}

	return contracts.ExportSection{
		Data:  map[string]any{"answers": []string{"Great"}},
		Files: []contracts.ExportFile{{Name: "answer.txt", Contents: []byte("Great")}},
	}, nil
}

/*
unzip reads every file of the zip by name.
*/
func unzip(t *testing.T, body []byte) map[string]string {
	t.Helper()

	r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)

	out := map[string]string{}

	for _, f := range r.File {
		rc, err := f.Open()
		require.NoError(t, err)
		contents, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		out[f.Name] = string(contents)
	}

	return out
}

func TestContactExport(t *testing.T) {
	t.Parallel()

	t.Run("builds a zip of the customer, their conversations, files and every exporter's section", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.TC.App.ContactExporters.Register(surveyExporter{})
		ready := tests.Record[appevents.ContactExportReady](d.TC)
		path, contact := customer(t, d)
		storeAttachment(t, d.TC, path)
		d.Owner.PostJSON(path+"/notes", testkit.JSON{"body": "VIP, be gentle"}).AssertCreated()
		d.Owner.PatchJSON("/api/v1/contacts/"+contact, testkit.JSON{"notes": "Prefers email"}).AssertOK()
		exports := "/api/v1/contacts/" + contact + "/exports"

		export := d.Owner.PostJSON(exports, nil).AssertStatus(202).
			AssertJSONPath("data.object", "contact_export").
			AssertJSONPath("data.status", "pending").
			AssertJSONPath("data.url", nil).String("data.id")
		d.Owner.PostJSON(exports, nil).AssertStatus(409).AssertJSONPath("code", "export_in_progress")
		d.Owner.GetJSON(exports+"/"+export+"/download").AssertStatus(409).
			AssertJSONPath("code", "export_not_ready")

		d.TC.Drain()
		require.Len(t, ready.All(), 1)
		res := d.Owner.GetJSON(exports+"/"+export).AssertOK().AssertJSONPath("data.status", "ready")
		zipped := d.Owner.GetJSON(res.String("data.url")).AssertOK().
			AssertHeader("Content-Type", "application/zip")
		files := unzip(t, zipped.Body)

		assert.Contains(t, files["contact.json"], `"name": "Ann"`)
		assert.Contains(t, files["contact.json"], `"value": "ann@example.com"`)
		assert.NotContains(t, files["contact.json"], "Prefers email")
		assert.Contains(t, files["conversations/1.txt"], "Card ending 1111, call me")
		assert.NotContains(t, files["conversations/1.txt"], "VIP, be gentle")
		assert.Contains(t, files["conversations/1.json"], `"subject": "My order"`)
		assert.Contains(t, files["surveys.json"], "Great")
		assert.Equal(t, "Great", files["surveys/answer.txt"])

		var attached int
		for name, contents := range files {
			if len(name) > 14 && name[:14] == "attachments/1/" {
				attached++
				assert.Equal(t, "receipt", contents)
			}
		}

		assert.Equal(t, 1, attached)
	})

	t.Run("adds the notes when asked and leaves an infected file out by name", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path, contact := customer(t, d)
		storeAttachment(t, d.TC, path)
		_, err := d.TC.Operator().Exec(t.Context(), `UPDATE attachments SET scan_status = 'infected'`)
		require.NoError(t, err)
		d.Owner.PostJSON(path+"/notes", testkit.JSON{"body": "VIP, be gentle"}).AssertCreated()
		d.Owner.PatchJSON("/api/v1/contacts/"+contact, testkit.JSON{"notes": "Prefers email"}).AssertOK()
		exports := "/api/v1/contacts/" + contact + "/exports"

		export := d.Owner.PostJSON(exports, testkit.JSON{"include_notes": true}).AssertStatus(202).String("data.id")
		d.TC.Drain()
		res := d.Owner.GetJSON(exports+"/"+export).AssertOK().AssertJSONPath("data.include_notes", true)
		files := unzip(t, d.Owner.GetJSON(res.String("data.url")).AssertOK().Body)

		assert.Contains(t, files["contact.json"], "Prefers email")
		assert.Contains(t, files["conversations/1.txt"], "VIP, be gentle")
		assert.Contains(t, files["conversations/1.json"], `"receipt.txt"`)
	})

	t.Run("marks an export failed when an exporter fails", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.TC.App.ContactExporters.Register(surveyExporter{fail: true})
		_, contact := customer(t, d)
		exports := "/api/v1/contacts/" + contact + "/exports"
		export := d.Owner.PostJSON(exports, nil).AssertStatus(202).String("data.id")
		id, err := d.TC.App.IDs.Decode("exp", export)
		require.NoError(t, err)

		err = d.TC.App.Actions.Inbox.BuildContactExport.Handle(t.Context(), contacts.BuildExportArgs{
			WorkspaceID: d.TC.Workspace.Workspace.ID, ExportID: id,
		})
		require.ErrorContains(t, err, "export surveys")
		d.Owner.GetJSON(exports+"/"+export).AssertOK().AssertJSONPath("data.status", "failed")
		require.NoError(t, d.TC.App.Actions.Inbox.BuildContactExport.Handle(t.Context(), contacts.BuildExportArgs{
			WorkspaceID: d.TC.Workspace.Workspace.ID, ExportID: ids.New(),
		}))
	})

	t.Run("deletes the zip once it expires", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		exports := "/api/v1/contacts/" + contact + "/exports"
		export := d.Owner.PostJSON(exports, nil).AssertStatus(202).String("data.id")
		d.TC.Drain()
		expire := d.TC.App.Actions.Inbox.ExpireContactExports

		expired, err := expire.Handle(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 0, expired)

		d.TC.Clock.Travel(25 * time.Hour)
		expired, err = expire.Handle(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 1, expired)
		d.Owner.GetJSON(exports+"/"+export).AssertOK().AssertJSONPath("data.status", "expired")
		d.Owner.GetJSON(exports + "/" + export + "/download").AssertStatus(409)
		d.TC.Table("contact_exports").AssertCount(0, testkit.Row{"status": "ready"})
	})

	t.Run("lets only those who may erase customers export them", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		member, _ := d.Join(t, enums.RoleMember)
		none := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, []string{"contacts:write"})

		member.PostJSON("/api/v1/contacts/"+contact+"/exports", nil).AssertForbidden()
		d.Owner.PostJSON("/api/v1/contacts/ctc_bogus/exports", nil).AssertNotFound()
		d.Owner.GetJSON("/api/v1/contacts/" + contact + "/exports/exp_bogus").AssertNotFound()
		d.Owner.GetJSON("/api/v1/contacts/ctc_bogus/exports/exp_bogus").AssertNotFound()
		d.Owner.GetJSON("/api/v1/contacts/" + contact + "/exports/" + "exp_01m6m7jx7gfmh9cdh45aqt7cs8").
			AssertNotFound()
		_, err := d.TC.App.Actions.Inbox.ShowContactExport.Handle(t.Context(), contacts.ContactExportInput{
			Membership: none,
		})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
	})
}
