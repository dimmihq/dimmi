package contacts_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
newCompany adds a company and returns its public id.
*/
func newCompany(t *testing.T, d tests.Desk, name string) string {
	t.Helper()

	return d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": name}).AssertCreated().String("data.id")
}

/*
companyNames lists the names of a contact's companies, in the order the
contact resource gives them.
*/
func companyNames(t *testing.T, res *testkit.Response) []string {
	t.Helper()

	var body struct {
		Data struct {
			Companies []struct {
				Name      string `json:"name"`
				IsDefault bool   `json:"is_default"`
			} `json:"companies"`
		} `json:"data"`
	}
	res.Decode(&body)

	names := []string{}
	for _, c := range body.Data.Companies {
		name := c.Name
		if c.IsDefault {
			name += "*"
		}

		names = append(names, name)
	}

	return names
}

/*
peopleCount returns how many people the companies list counts for the
company with the name.
*/
func peopleCount(t *testing.T, d tests.Desk, name string) int {
	t.Helper()

	var page struct {
		Data []struct {
			PeopleCount int `json:"people_count"`
		} `json:"data"`
	}
	res := d.Owner.GetJSON("/api/v1/companies?q=" + url.QueryEscape(name)).AssertOK()
	res.AssertJSONPath("data.#", 1).Decode(&page)

	return page.Data[0].PeopleCount
}

func TestContactCompanies(t *testing.T) {
	t.Parallel()

	t.Run("adds, lists, makes default and removes companies", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		updated := tests.Record[appevents.ContactUpdated](d.TC)
		acme, globex, initech := newCompany(t, d, "Acme"), newCompany(t, d, "Globex"), newCompany(t, d, "Initech")
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})

		res := d.Owner.PostJSON(path+"/companies", testkit.JSON{"company_id": acme}).AssertOK().
			AssertJSONPath("data.company.id", acme)
		assert.Equal(t, []string{"Acme*"}, companyNames(t, res))

		d.TC.Clock.Travel(time.Minute)
		res = d.Owner.PostJSON(path+"/companies", testkit.JSON{"company_id": globex}).AssertOK().
			AssertJSONPath("data.company.id", acme)
		assert.Equal(t, []string{"Acme*", "Globex"}, companyNames(t, res))

		d.TC.Clock.Travel(time.Minute)
		res = d.Owner.PostJSON(path+"/companies", testkit.JSON{"company_id": initech, "default": true}).AssertOK().
			AssertJSONPath("data.company.id", initech)
		assert.Equal(t, []string{"Initech*", "Acme", "Globex"}, companyNames(t, res))

		res = d.Owner.PostJSON(path+"/companies/"+globex+"/default", nil).AssertOK().
			AssertJSONPath("data.company.id", globex)
		assert.Equal(t, []string{"Globex*", "Acme", "Initech"}, companyNames(t, res))

		d.Owner.PostJSON(path+"/companies", testkit.JSON{"company_id": acme}).AssertOK()
		res = d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.companies.#", 3).
			AssertJSONPath("data.companies.0.object", "company")
		assert.Equal(t, []string{"Globex*", "Acme", "Initech"}, companyNames(t, res))

		res = d.Owner.Delete(path + "/companies/" + acme).AssertOK()
		assert.Equal(t, []string{"Globex*", "Initech"}, companyNames(t, res))
		d.Owner.Delete(path + "/companies/" + acme).AssertOK()

		changes := [][]string{}
		for _, e := range updated.All() {
			changes = append(changes, e.Changes)
		}

		assert.Equal(t, [][]string{
			{"companies", "company_id"}, {"companies"}, {"companies", "company_id"}, {"company_id"}, {"companies"},
		}, changes)
	})

	t.Run("makes the oldest remaining company the default when the default goes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex, initech := newCompany(t, d, "Acme"), newCompany(t, d, "Globex"), newCompany(t, d, "Initech")
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})

		for _, id := range []string{acme, globex, initech} {
			d.Owner.PostJSON(path+"/companies", testkit.JSON{"company_id": id}).AssertOK()
			d.TC.Clock.Travel(time.Minute)
		}

		res := d.Owner.Delete(path+"/companies/"+acme).AssertOK().AssertJSONPath("data.company.id", globex)
		assert.Equal(t, []string{"Globex*", "Initech"}, companyNames(t, res))

		res = d.Owner.PatchJSON(path, testkit.JSON{"company_id": nil}).AssertOK().
			AssertJSONPath("data.company.id", initech)
		assert.Equal(t, []string{"Initech*"}, companyNames(t, res))

		d.Owner.Delete("/api/v1/companies/" + initech).AssertNoContent()
		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.company", nil).AssertJSONPath("data.companies.#", 0)
	})

	t.Run("moves the default when the default company is deleted", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex := newCompany(t, d, "Acme"), newCompany(t, d, "Globex")
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann", "company_ids": []string{acme, globex}})

		d.Owner.Delete("/api/v1/companies/" + acme).AssertNoContent()

		res := d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.company.id", globex)
		assert.Equal(t, []string{"Globex*"}, companyNames(t, res))
	})

	t.Run("takes a list on create and replaces it on update", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex, initech := newCompany(t, d, "Acme"), newCompany(t, d, "Globex"), newCompany(t, d, "Initech")

		res := d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{
			"name": "Ann", "company_ids": []string{acme, globex}, "company_id": globex,
		}).AssertCreated().AssertJSONPath("data.company.id", globex)
		assert.ElementsMatch(t, []string{"Globex*", "Acme"}, companyNames(t, res))
		path := "/api/v1/contacts/" + res.String("data.id")

		res = d.Owner.PatchJSON(path, testkit.JSON{"company_ids": []string{initech, globex}}).AssertOK().
			AssertJSONPath("data.company.id", globex)
		assert.ElementsMatch(t, []string{"Globex*", "Initech"}, companyNames(t, res))

		res = d.Owner.PatchJSON(path, testkit.JSON{"company_ids": []string{acme}}).AssertOK().
			AssertJSONPath("data.company.id", acme)
		assert.Equal(t, []string{"Acme*"}, companyNames(t, res))

		res = d.Owner.PatchJSON(path, testkit.JSON{"company_ids": []string{acme, initech}, "company_id": initech}).
			AssertOK().AssertJSONPath("data.company.id", initech)
		assert.ElementsMatch(t, []string{"Initech*", "Acme"}, companyNames(t, res))

		d.Owner.PatchJSON(path, testkit.JSON{"company_ids": []string{}}).AssertOK().
			AssertJSONPath("data.company", nil).AssertJSONPath("data.companies.#", 0)
	})

	t.Run("keeps the old default linked when company_id moves it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex := newCompany(t, d, "Acme"), newCompany(t, d, "Globex")
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})

		d.Owner.PatchJSON(path, testkit.JSON{"company_id": acme}).AssertOK()
		res := d.Owner.PatchJSON(path, testkit.JSON{"company_id": globex}).AssertOK().
			AssertJSONPath("data.company.id", globex)
		assert.ElementsMatch(t, []string{"Globex*", "Acme"}, companyNames(t, res))
	})

	t.Run("counts and lists everyone who belongs to a company", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex := newCompany(t, d, "Acme"), newCompany(t, d, "Globex")
		contact(t, d, testkit.JSON{"name": "Ann", "company_ids": []string{acme}})
		contact(t, d, testkit.JSON{"name": "Bo", "company_ids": []string{globex, acme}})
		contact(t, d, testkit.JSON{"name": "Cy", "company_ids": []string{globex}})

		assert.Equal(t, 2, peopleCount(t, d, "Acme"))
		d.Owner.GetJSON("/api/v1/companies/"+acme+"/contacts").AssertOK().AssertJSONPath("data.#", 2)
		d.Owner.GetJSON("/api/v1/contacts?filter[company]="+acme).AssertOK().AssertJSONPath("data.#", 2)
		assert.Equal(t, []string{"Ann", "Bo"},
			matching(t, d, `[{"field":"company","op":"is","values":["`+acme+`"]}]`))
		assert.Equal(t, []string{"Cy"},
			matching(t, d, `[{"field":"company","op":"is_not","values":["`+acme+`"]}]`))
	})

	t.Run("matches audience rules on any of the contact's companies", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme := d.Owner.PostJSON("/api/v1/companies", testkit.JSON{
			"name": "Acme", "custom_attributes": testkit.JSON{"plan": "pro"},
		}).AssertCreated().String("data.id")
		globex := d.Owner.PostJSON("/api/v1/companies", testkit.JSON{
			"name": "Globex", "custom_attributes": testkit.JSON{"plan": "free"},
		}).AssertCreated().String("data.id")
		ann := contact(t, d, testkit.JSON{"name": "Ann", "company_ids": []string{globex, acme}})
		bo := contact(t, d, testkit.JSON{"name": "Bo", "company_ids": []string{globex}})
		pro := testkit.JSON{"rules": testkit.JSON{"field": "company.custom.plan", "op": "eq", "value": "pro"}}
		notPro := testkit.JSON{"rules": testkit.JSON{"field": "company.custom.plan", "op": "neq", "value": "pro"}}

		evaluate := func(rules testkit.JSON, who string) string {
			return d.Owner.PostJSON("/api/v1/audience-evaluations", testkit.JSON{"rules": rules, "contact_id": who}).
				AssertOK().String("data.result")
		}

		assert.Equal(t, "match", evaluate(pro, ann))
		assert.Equal(t, "no_match", evaluate(pro, bo))
		assert.Equal(t, "no_match", evaluate(notPro, ann))
		assert.Equal(t, "match", evaluate(notPro, bo))
	})

	t.Run("refuses other workspaces' companies and contacts", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		other := tests.NewDesk(t)
		mine := newCompany(t, d, "Acme")
		theirs := newCompany(t, other, "Theirs")
		path := "/api/v1/contacts/" + contact(t, d, testkit.JSON{"name": "Ann"})
		stranger := "/api/v1/contacts/" + contact(t, other, testkit.JSON{"name": "Stranger"})

		d.Owner.PostJSON(path+"/companies", testkit.JSON{"company_id": theirs}).
			AssertStatus(422).AssertJSONPath("code", "company_not_found")
		d.Owner.PostJSON(path+"/companies/"+theirs+"/default", nil).
			AssertStatus(422).AssertJSONPath("code", "company_not_found")
		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Bo", "company_ids": []string{mine, theirs}}).
			AssertStatus(422).AssertJSONPath("code", "company_not_found")
		d.Owner.PatchJSON(path, testkit.JSON{"company_ids": []string{"bogus"}}).
			AssertStatus(422).AssertJSONPath("code", "company_not_found")
		d.Owner.PostJSON(stranger+"/companies", testkit.JSON{"company_id": mine}).AssertNotFound()
		assert.Equal(t, 0, peopleCount(t, other, "Theirs"))
	})

	t.Run("moves the source's companies over on a merge", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex, initech := newCompany(t, d, "Acme"), newCompany(t, d, "Globex"), newCompany(t, d, "Initech")
		target := contact(t, d, testkit.JSON{"name": "Ann", "company_ids": []string{acme, globex}})
		source := contact(t, d, testkit.JSON{"name": "Ann L", "company_ids": []string{initech, globex}})

		res := d.Owner.PostJSON("/api/v1/contacts/"+target+"/merges", testkit.JSON{"source_id": source}).AssertOK().
			AssertJSONPath("data.company.id", acme)
		assert.ElementsMatch(t, []string{"Acme*", "Globex", "Initech"}, companyNames(t, res))
		assert.Equal(t, 1, peopleCount(t, d, "Initech"))

		lead := contact(t, d, testkit.JSON{"name": "Lead"})
		res = d.Owner.PostJSON("/api/v1/contacts/"+lead+"/merges", testkit.JSON{"source_id": target}).AssertOK().
			AssertJSONPath("data.company.id", acme)
		assert.ElementsMatch(t, []string{"Acme*", "Globex", "Initech"}, companyNames(t, res))
	})

	t.Run("adds the companies an upsert lists without moving the default", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex := newCompany(t, d, "Acme"), newCompany(t, d, "Globex")
		upsert := "/api/v1/contacts?upsert=true"

		d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "u1", "company_ids": []string{acme}}).AssertCreated()
		res := d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "u1", "company_ids": []string{globex}}).
			AssertOK().AssertJSONPath("data.company.id", acme)
		assert.Equal(t, []string{"Acme*", "Globex"}, companyNames(t, res))
	})

	t.Run("refuses more than three hundred companies", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ids := make([]string, 301)
		for i := range ids {
			ids[i] = fmt.Sprintf("cmp_%d", i)
		}

		d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"name": "Ann", "company_ids": ids}).
			AssertValidationError("company_ids")
	})
}

func TestContactCompanyConstraints(t *testing.T) {
	t.Parallel()

	t.Run("keeps one link per pair and one default per contact", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex := newCompany(t, d, "Acme"), newCompany(t, d, "Globex")
		contact(t, d, testkit.JSON{"name": "Ann", "company_ids": []string{acme, globex}})
		db := d.TC.Operator()

		var links, defaults int
		require.NoError(t, db.QueryRow(t.Context(),
			`SELECT count(*), count(*) FILTER (WHERE is_default) FROM contact_companies`).Scan(&links, &defaults))
		assert.Equal(t, 2, links)
		assert.Equal(t, 1, defaults)

		_, err := db.Exec(t.Context(), `INSERT INTO contact_companies (workspace_id, contact_id, company_id)
			SELECT workspace_id, contact_id, company_id FROM contact_companies LIMIT 1`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contact_companies_pkey")

		_, err = db.Exec(t.Context(), `UPDATE contact_companies SET is_default = true WHERE NOT is_default`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contact_companies_contact_id_default_key")

		var synced bool
		require.NoError(t, db.QueryRow(t.Context(), `SELECT bool_and(c.company_id = cc.company_id)
			FROM contacts c JOIN contact_companies cc ON cc.contact_id = c.id AND cc.is_default`).Scan(&synced))
		assert.True(t, synced)
	})
}

func TestConversationAndTicketCompanies(t *testing.T) {
	t.Parallel()

	t.Run("starts a conversation on the default company and switches it to another", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex, initech := newCompany(t, d, "Acme"), newCompany(t, d, "Globex"), newCompany(t, d, "Initech")
		ann := contact(t, d, testkit.JSON{"name": "Ann", "company_ids": []string{acme, globex}})
		inbox := models.PublicID(models.PrefixInbox, d.Inbox.Inbox.ID)

		conv := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": inbox, "author": "contact", "body": "Hi", "contact": testkit.JSON{"id": ann},
		}).AssertCreated().AssertJSONPath("data.company.id", acme).String("data.id")
		path := "/api/v1/conversations/" + conv

		d.Owner.PatchJSON(path, testkit.JSON{"company_id": globex}).AssertOK().
			AssertJSONPath("data.company.name", "Globex").
			AssertJSONPath("meta.changed.0", "company_id")
		d.Owner.PatchJSON(path, testkit.JSON{"company_id": initech}).
			AssertStatus(422).AssertJSONPath("code", "company_not_linked")
		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.company.id", globex)
		d.Owner.GetJSON(path+"/context").AssertOK().
			AssertJSONPath("data.company.name", "Globex").
			AssertJSONPath("data.contact.company.name", "Acme")
		d.Owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[company]="+globex).AssertOK().
			AssertJSONPath("data.#", 1)
	})

	t.Run("opens a ticket on the default company and moves it to another", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, globex, initech := newCompany(t, d, "Acme"), newCompany(t, d, "Globex"), newCompany(t, d, "Initech")
		ann := contact(t, d, testkit.JSON{"name": "Ann", "company_ids": []string{acme, globex}})

		var types struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		d.Owner.GetJSON("/api/v1/ticket-types").AssertOK().Decode(&types)
		backOffice := ""
		for _, ty := range types.Data {
			if ty.Name == "Back-office task" {
				backOffice = ty.ID
			}
		}

		ticket := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{
			"type_id": backOffice, "title": "Refund", "contact_id": ann,
		}).AssertCreated().AssertJSONPath("data.company_id", acme).String("data.id")
		path := "/api/v1/tickets/" + ticket

		d.Owner.PatchJSON(path, testkit.JSON{"company_id": globex}).AssertOK().AssertJSONPath("data.company_id", globex)
		d.Owner.PatchJSON(path, testkit.JSON{"company_id": initech}).
			AssertStatus(422).AssertJSONPath("code", "company_not_linked")
	})
}

func TestImportedCompanies(t *testing.T) {
	t.Parallel()

	t.Run("puts a customer in every company the row names, the first the default", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		csv := "Email,Company,Company domain\n" +
			"ana@acme.dev,\"Acme, Inc.; Globex | Initech\",acme.dev;;initech.io\n"
		id := uploadImport(d.Owner, "contacts", csv).AssertCreated().String("data.id")
		plan := testkit.JSON{"mode": "update", "mapping": []testkit.JSON{
			column(0, "email"), column(1, "company_name"), column(2, "company_domain"),
		}}

		d.Owner.PostJSON("/api/v1/imports/"+id+"/runs", plan).AssertStatus(202)
		d.TC.Drain()

		ana := d.Owner.GetJSON("/api/v1/contacts?filter[email]=ana@acme.dev").AssertOK().
			AssertJSONPath("data.0.company.name", "Acme, Inc.")
		path := "/api/v1/contacts/" + ana.String("data.0.id")
		res := d.Owner.GetJSON(path).AssertOK()
		assert.ElementsMatch(t, []string{"Acme, Inc.*", "Globex", "Initech"}, companyNames(t, res))
		assert.Equal(t, 1, peopleCount(t, d, "Initech"))
	})
}
