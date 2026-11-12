package contacts_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/imports"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
runImport uploads the CSV, runs it with the mapping and waits for it.
*/
func runImport(d *tests.Desk, kind, csv string, mapping ...testkit.JSON) string {
	id := uploadImport(d.Owner, kind, csv).AssertCreated().String("data.id")
	d.Owner.PostJSON("/api/v1/imports/"+id+"/runs", testkit.JSON{"mode": "update", "mapping": mapping}).
		AssertStatus(202)
	d.TC.Drain()

	return id
}

func TestImportRollbacks(t *testing.T) {
	t.Parallel()

	t.Run("removes what it made, puts back what it changed, keeps what was worked on since", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "cy@acme.dev", "name": "Old Cy"}).AssertCreated()
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "dee@acme.dev", "name": "Old Dee"}).AssertCreated()

		id := runImport(&d, "contacts",
			"email,name,plan,tags\nana@acme.dev,Ana,pro,vip\ncy@acme.dev,Cy,pro,vip\n"+
				"dee@acme.dev,Dee,,\neve@acme.dev,Eve,,\n",
			column(0, "email"), column(1, "name"), column(2, "attribute.plan"), column(3, "tags"))
		d.Owner.GetJSON("/api/v1/imports/"+id).AssertOK().AssertJSONPath("data.created_count", float64(2)).
			AssertJSONPath("data.updated_count", float64(2)).AssertJSONPath("data.rollback", nil)

		eve := d.Owner.GetJSON("/api/v1/contacts?filter[email]=eve@acme.dev").AssertOK().String("data.0.id")
		d.Owner.PatchJSON("/api/v1/contacts/"+eve, testkit.JSON{"name": "Eve Edited"}).AssertOK()
		dee := d.Owner.GetJSON("/api/v1/contacts?filter[email]=dee@acme.dev").AssertOK().String("data.0.id")
		d.Owner.PatchJSON("/api/v1/contacts/"+dee, testkit.JSON{"name": "Dee Manual"}).AssertOK()

		d.Owner.PostJSON("/api/v1/imports/"+id+"/rollbacks", nil).AssertStatus(202).
			AssertJSONPath("data.rollback.status", "queued").AssertJSONPath("data.rollback.total", float64(4))
		d.Owner.PostJSON("/api/v1/imports/"+id+"/rollbacks", nil).AssertStatus(409).
			AssertJSONPath("code", "import_rolled_back")
		d.TC.Drain()

		res := d.Owner.GetJSON("/api/v1/imports/"+id).AssertOK().
			AssertJSONPath("data.rollback.status", "completed").AssertJSONPath("data.rollback.processed", float64(4)).
			AssertJSONPath("data.rollback.removed", float64(1)).AssertJSONPath("data.rollback.reverted", float64(1)).
			AssertJSONPath("data.rollback.kept", float64(2))
		assert.ElementsMatch(t, []string{"Eve Edited", "Dee Manual"},
			[]string{res.String("data.rollback.kept_records.0.name"), res.String("data.rollback.kept_records.1.name")})
		res.AssertJSONPath("data.rollback.kept_records.0.reason", "Changed since the import.")

		d.Owner.GetJSON("/api/v1/contacts?filter[email]=ana@acme.dev").AssertOK().AssertJSONMissing("data.0")
		cy := d.Owner.GetJSON("/api/v1/contacts?filter[email]=cy@acme.dev").AssertOK().
			AssertJSONPath("data.0.name", "Old Cy").AssertJSONMissing("data.0.custom_attributes.plan")
		assert.Empty(t, names(cy, "data.0.tags"))
		d.Owner.GetJSON("/api/v1/contacts/"+eve).AssertOK().AssertJSONPath("data.name", "Eve Edited")
		d.Owner.GetJSON("/api/v1/contacts/"+dee).AssertOK().AssertJSONPath("data.name", "Dee Manual")

		d.Owner.GetJSON("/api/v1/imports?kind=contacts").AssertOK().
			AssertJSONPath("data.0.rollback.status", "completed").
			AssertJSONPath("data.0.created_by_name", d.TC.Owner.Name)
	})

	t.Run("removes the companies an import made", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		id := runImport(&d, "companies", "name,domain\nAcme,acme.dev\n", column(0, "name"), column(1, "domain"))
		d.Owner.GetJSON("/api/v1/companies?q=acme").AssertOK().AssertJSONPath("data.0.domain", "acme.dev")

		d.Owner.PostJSON("/api/v1/imports/"+id+"/rollbacks", nil).AssertStatus(202)
		d.TC.Drain()

		d.Owner.GetJSON("/api/v1/imports/"+id).AssertJSONPath("data.rollback.removed", float64(1))
		d.Owner.GetJSON("/api/v1/companies?q=acme").AssertOK().AssertJSONMissing("data.0")
	})

	t.Run("refuses a member without imports:rollback and an import past its week", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		id := runImport(&d, "contacts", "email\nzed@acme.dev\n", column(0, "email"))
		member, _ := d.Join(t, enums.RoleMember)
		member.PostJSON("/api/v1/imports/"+id+"/rollbacks", nil).AssertForbidden()

		d.TC.Clock.Travel(8 * 24 * time.Hour)
		d.Owner.PostJSON("/api/v1/imports/"+id+"/rollbacks", nil).AssertStatus(409).
			AssertJSONPath("code", "import_rollback_closed")
	})

	t.Run("empties the file a month after the import, keeping the import and its counts", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		id := runImport(&d, "contacts", "email,name\nyan@acme.dev,Yan\n,Nobody\n",
			column(0, "email"), column(1, "name"))
		d.Owner.Send("GET", "/api/v1/imports/"+id+"/errors", nil).AssertOK()

		later := clock.NewFake(d.TC.Clock.Now().Add(31 * 24 * time.Hour))
		_, err := (&imports.ExpireImports{DB: d.TC.App.DB, Clock: later}).Handle(t.Context())
		require.NoError(t, err)

		d.Owner.GetJSON("/api/v1/imports/"+id).AssertOK().AssertJSONPath("data.status", "completed").
			AssertJSONPath("data.created_count", float64(1)).AssertJSONPath("data.columns.0", "email")
		assert.NotEmpty(t, d.Owner.GetJSON("/api/v1/imports/"+id).String("data.file_expired_at"))
		d.Owner.Send("GET", "/api/v1/imports/"+id+"/errors", nil).AssertStatus(410).
			AssertJSONPath("code", "import_file_expired")
	})

	t.Run("limits each member to ten uploads an hour", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		for range 10 {
			uploadImport(d.Owner, "contacts", "email\na@b.dev\n").AssertCreated()
		}

		uploadImport(d.Owner, "contacts", "email\na@b.dev\n").AssertStatus(429).
			AssertJSONPath("code", "import_upload_limited")
	})
}
