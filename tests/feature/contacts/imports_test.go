package contacts_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
uploadImport sends a CSV as a new import of the kind.
*/
func uploadImport(c *testkit.Client, kind, csv string) *testkit.Response {
	return c.PostForm("/api/v1/imports",
		testkit.FormFile{Field: "file", Name: "people.csv", Contents: []byte(csv)}, map[string]string{"kind": kind})
}

/*
column maps one column to a field.
*/
func column(index int, field string) testkit.JSON {
	return testkit.JSON{"column": index, "field": field}
}

const peopleCSV = "\xef\xbb\xbfEmail,Full name,Plan,Seats,Tags,Company\n" +
	"ana@acme.dev,Ana,pro,3,vip; beta,Acme\n" +
	"bo@acme.dev,Bo,free,two,,Acme\n" +
	",Nobody,free,1,,\n" +
	"cy@acme.dev,Cy,pro,5,beta,\n"

func TestContactImports(t *testing.T) {
	t.Parallel()

	t.Run("previews a file, suggests a mapping, dry runs and imports in the background", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON("/api/v1/attribute-definitions", testkit.JSON{
			"object": "contact", "key": "seats", "label": "Seats", "data_type": "number",
		}).AssertCreated()
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "cy@acme.dev", "name": "Old Cy"}).AssertCreated()

		res := uploadImport(d.Owner, "contacts", peopleCSV).AssertCreated().
			AssertJSONPath("data.status", "draft").AssertJSONPath("data.row_count", float64(4)).
			AssertJSONPath("data.columns.0", "Email").AssertJSONPath("data.preview.0.1", "Ana").
			AssertJSONPath("data.suggested_mapping.0.field", "email").
			AssertJSONPath("data.suggested_mapping.1.field", "name").
			AssertJSONPath("data.suggested_mapping.2.field", "attribute.seats")
		id := res.String("data.id")

		plan := testkit.JSON{"mode": "update", "mapping": []testkit.JSON{
			column(0, "email"), column(1, "name"), column(2, "attribute.plan"), column(3, "attribute.seats"),
			column(4, "tags"), column(5, "company_name"),
		}}

		d.Owner.PostJSON("/api/v1/imports/"+id+"/previews", plan).AssertOK().
			AssertJSONPath("data.rows", float64(4)).AssertJSONPath("data.created", float64(1)).
			AssertJSONPath("data.updated", float64(1)).AssertJSONPath("data.failed", float64(2)).
			AssertJSONPath("data.errors.0.row", float64(3)).
			AssertJSONPath("data.errors.0.message", "Seats: not a number").
			AssertJSONPath("data.errors.1.message", "needs an email or external id")

		d.Owner.GetJSON("/api/v1/contacts?filter[email]=ana@acme.dev").AssertOK().AssertJSONMissing("data.0")

		d.Owner.PostJSON("/api/v1/imports/"+id+"/runs", plan).AssertStatus(202).
			AssertJSONPath("data.status", "queued")
		d.Owner.PostJSON("/api/v1/imports/"+id+"/runs", plan).AssertStatus(409)
		d.TC.Drain()

		d.Owner.GetJSON("/api/v1/imports/"+id).AssertOK().AssertJSONPath("data.status", "completed").
			AssertJSONPath("data.processed_rows", float64(4)).AssertJSONPath("data.created_count", float64(1)).
			AssertJSONPath("data.updated_count", float64(1)).AssertJSONPath("data.failed_count", float64(2)).
			AssertJSONPath("data.error_report_url", "/api/v1/imports/"+id+"/errors")

		ana := d.Owner.GetJSON("/api/v1/contacts?filter[email]=ana@acme.dev").AssertOK().
			AssertJSONPath("data.0.name", "Ana").AssertJSONPath("data.0.custom_attributes.plan", "pro").
			AssertJSONPath("data.0.custom_attributes.seats", float64(3))
		assert.ElementsMatch(t, []string{"vip", "beta"}, names(ana, "data.0.tags"))
		assert.Equal(t, "Acme", ana.String("data.0.company.name"))

		d.Owner.GetJSON("/api/v1/contacts?filter[email]=cy@acme.dev").AssertOK().AssertJSONPath("data.0.name", "Cy")

		report := d.Owner.Send("GET", "/api/v1/imports/"+id+"/errors", nil).AssertOK()
		assert.Contains(t, report.Header.Get("Content-Type"), "text/csv")
		lines := strings.Split(strings.TrimSpace(string(report.Body)), "\n")
		require.Len(t, lines, 3)
		assert.Equal(t, "row,error,Email,Full name,Plan,Seats,Tags,Company", lines[0])
		assert.Equal(t, "3,Seats: not a number,bo@acme.dev,Bo,free,two,,Acme", lines[1])

		d.Owner.GetJSON("/api/v1/imports?kind=contacts").AssertOK().AssertJSONPath("data.0.id", id)
	})

	t.Run("skips customers already there when asked to", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"external_id": "u_1", "name": "Kept"}).AssertCreated()

		id := uploadImport(d.Owner, "contacts", "user_id,name\nu_1,Changed\nu_2,New\n").
			AssertCreated().String("data.id")
		plan := testkit.JSON{"mode": "skip", "mapping": []testkit.JSON{column(0, "external_id"), column(1, "name")}}

		d.Owner.PostJSON("/api/v1/imports/"+id+"/previews", plan).AssertOK().
			AssertJSONPath("data.skipped", float64(1)).AssertJSONPath("data.created", float64(1))
		d.Owner.PostJSON("/api/v1/imports/"+id+"/runs", plan).AssertStatus(202)
		d.TC.Drain()

		d.Owner.GetJSON("/api/v1/imports/"+id).AssertJSONPath("data.skipped_count", float64(1)).
			AssertJSONPath("data.created_count", float64(1)).AssertJSONPath("data.error_report_url", nil)
		d.Owner.GetJSON("/api/v1/contacts?filter[external_id]=u_1").AssertJSONPath("data.0.name", "Kept")
	})

	t.Run("imports companies by domain with tags and attributes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		id := uploadImport(d.Owner, "companies", "Company,Website,Tier,Tags\nAcme,www.acme.dev,gold,key\nNo key,,x,\n").
			AssertCreated().AssertJSONPath("data.suggested_mapping.0.field", "name").
			AssertJSONPath("data.suggested_mapping.1.field", "domain").String("data.id")
		plan := testkit.JSON{"mode": "update", "mapping": []testkit.JSON{
			column(0, "name"), column(1, "domain"), column(2, "attribute.tier"), column(3, "tags"),
		}}

		d.Owner.PostJSON("/api/v1/imports/"+id+"/runs", plan).AssertStatus(202)
		d.TC.Drain()

		d.Owner.GetJSON("/api/v1/imports/"+id).AssertJSONPath("data.created_count", float64(1)).
			AssertJSONPath("data.failed_count", float64(1)).
			AssertJSONPath("data.errors.0.message", "needs a domain or external id")
		d.Owner.GetJSON("/api/v1/companies?q=acme").AssertOK().AssertJSONPath("data.0.domain", "acme.dev").
			AssertJSONPath("data.0.custom_attributes.tier", "gold")
	})

	t.Run("refuses a bad file, a bad mapping and a member without imports:manage", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		uploadImport(d.Owner, "contacts", "email\n").AssertStatus(422).AssertJSONPath("code", "import_empty")
		uploadImport(d.Owner, "people", "email\na@b.c\n").AssertStatus(422).
			AssertJSONPath("code", "import_kind_invalid")

		id := uploadImport(d.Owner, "contacts", "email,name\na@b.dev,A\n").AssertCreated().String("data.id")
		d.Owner.PostJSON("/api/v1/imports/"+id+"/previews", testkit.JSON{
			"mode": "update", "mapping": []testkit.JSON{column(1, "name")},
		}).AssertStatus(422).AssertJSONPath("code", "import_mapping_invalid")
		d.Owner.PostJSON("/api/v1/imports/"+id+"/previews", testkit.JSON{
			"mode": "update", "mapping": []testkit.JSON{column(0, "email"), column(5, "name")},
		}).AssertStatus(422).AssertJSONPath("code", "import_mapping_invalid")

		member, _ := d.Join(t, enums.RoleMember)
		uploadImport(member, "contacts", "email\na@b.dev\n").AssertForbidden()
		member.GetJSON("/api/v1/imports/" + id).AssertForbidden()
	})
}

/*
names returns the names of the tags at the path.
*/
func names(res *testkit.Response, path string) []string {
	var body map[string]any
	res.Decode(&body)

	var out []string

	for _, t := range lookupList(body, path) {
		m, ok := t.(map[string]any)
		if name, named := m["name"].(string); ok && named {
			out = append(out, name)
		}
	}

	return out
}

/*
lookupList walks a dotted path of keys and list indexes to a list.
*/
func lookupList(v any, path string) []any {
	for part := range strings.SplitSeq(path, ".") {
		switch node := v.(type) {
		case map[string]any:
			v = node[part]
		case []any:
			i := 0
			for _, r := range part {
				i = i*10 + int(r-'0')
			}

			if i >= len(node) {
				return nil
			}

			v = node[i]
		}
	}

	list, _ := v.([]any)

	return list
}
