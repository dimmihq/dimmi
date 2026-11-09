package contacts_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestContactPages(t *testing.T) {
	t.Parallel()

	t.Run("counts the matching contacts and opens a numbered page", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		for _, name := range []string{"Ada", "Bea", "Cy"} {
			contact(t, d, testkit.JSON{"name": name, "email": strings.ToLower(name) + "@example.com"})
		}

		first := d.Owner.GetJSON("/api/v1/contacts?limit=2&sort=name&total=true").AssertOK().
			AssertJSONPath("meta.total", 3).AssertJSONPath("data.0.name", "Ada")
		assert.NotEmpty(t, first.String("meta.next_cursor"))
		d.Owner.GetJSON("/api/v1/contacts?limit=2&sort=name&page=2&total=true").AssertOK().
			AssertJSONPath("data.#", 1).AssertJSONPath("data.0.name", "Cy").AssertJSONPath("meta.total", 3)
		d.Owner.GetJSON("/api/v1/contacts?q=bea&total=true").AssertOK().AssertJSONPath("meta.total", 1)
		assert.NotContains(t, string(d.Owner.GetJSON("/api/v1/contacts").AssertOK().Body), `"total"`)

		d.Owner.GetJSON("/api/v1/contacts?page=0").AssertStatus(400).AssertJSONPath("code", "invalid_parameter")
		d.Owner.GetJSON("/api/v1/contacts?page=two").AssertStatus(400)
		d.Owner.GetJSON("/api/v1/contacts?limit=100&page=101").AssertStatus(400)
	})
}

func TestCompanyPages(t *testing.T) {
	t.Parallel()

	t.Run("counts companies and their people and opens a numbered page", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		var acme string
		for _, name := range []string{"Acme", "Globex", "Initech"} {
			id := d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": name}).AssertCreated().String("data.id")
			if name == "Acme" {
				acme = id
			}
		}
		ana := contact(t, d, testkit.JSON{"name": "Ana"})
		d.Owner.PatchJSON("/api/v1/contacts/"+ana, testkit.JSON{"company_id": acme}).AssertOK()

		d.Owner.GetJSON("/api/v1/companies?limit=2&total=true").AssertOK().
			AssertJSONPath("meta.total", 3).AssertJSONPath("data.#", 2)
		d.Owner.GetJSON("/api/v1/companies?limit=2&page=2&total=true").AssertOK().
			AssertJSONPath("data.#", 1).AssertJSONPath("data.0.name", "Acme").AssertJSONPath("data.0.people_count", 1)
		d.Owner.GetJSON("/api/v1/companies?q=glob&total=true").AssertOK().AssertJSONPath("meta.total", 1)
		d.Owner.GetJSON("/api/v1/companies?page=0").AssertStatus(400)
	})
}

func TestAudienceAndErasurePages(t *testing.T) {
	t.Parallel()

	t.Run("pages and searches audiences by name and still lists every one without a page", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		for _, name := range []string{"Pro", "Team", "Trial"} {
			d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": name, "rules": testkit.JSON{
				"rules": testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": "x"},
			}}).AssertCreated()
		}

		d.Owner.GetJSON("/api/v1/audiences?limit=2&page=1&total=true").AssertOK().
			AssertJSONPath("meta.total", 3).AssertJSONPath("data.#", 2).AssertJSONPath("data.0.name", "Pro")
		d.Owner.GetJSON("/api/v1/audiences?limit=2&page=2").AssertOK().
			AssertJSONPath("data.#", 1).AssertJSONPath("data.0.name", "Trial")
		d.Owner.GetJSON("/api/v1/audiences?q=t&page=1&total=true").AssertOK().AssertJSONPath("meta.total", 2)
		d.Owner.GetJSON("/api/v1/audiences").AssertOK().AssertJSONPath("data.#", 3)
		d.Owner.GetJSON("/api/v1/audiences?page=0").AssertStatus(400)
	})

	t.Run("pages erasures and narrows them to one status", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		var last string
		for _, name := range []string{"Ana", "Bo", "Cy"} {
			id := contact(t, d, testkit.JSON{"name": name})
			last = d.Owner.Delete("/api/v1/contacts/" + id + "?erase=true").AssertStatus(202).String("data.id")
		}
		d.Owner.PatchJSON("/api/v1/contact-erasures/"+last, testkit.JSON{"status": "canceled"}).AssertOK()

		d.Owner.GetJSON("/api/v1/contact-erasures?limit=2&total=true").AssertOK().
			AssertJSONPath("meta.total", 3).AssertJSONPath("data.#", 2)
		d.Owner.GetJSON("/api/v1/contact-erasures?limit=2&page=2").AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.GetJSON("/api/v1/contact-erasures?filter[status]=canceled&total=true").AssertOK().
			AssertJSONPath("meta.total", 1).AssertJSONPath("data.0.id", last)
	})
}

/*
matching lists the names of the contacts the conditions keep, by name.
*/
func matching(t *testing.T, d tests.Desk, conditions string) []string {
	t.Helper()
	var page struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	d.Owner.GetJSON("/api/v1/contacts?sort=name&conditions=" + url.QueryEscape(conditions)).AssertOK().Decode(&page)

	names := []string{}
	for _, c := range page.Data {
		names = append(names, c.Name)
	}

	return names
}

func TestContactConditions(t *testing.T) {
	t.Parallel()

	t.Run("narrows the list by text, numbers, booleans, tags, company and time", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme := d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "Acme"}).AssertCreated().String("data.id")
		ada := contact(t, d, testkit.JSON{
			"name": "Ada", "email": "ada@acme.dev",
			"custom_attributes": testkit.JSON{"plan": "Pro", "seats": 40, "vip": true},
		})
		bea := contact(t, d, testkit.JSON{
			"name": "Bea", "email": "bea@other.io", "custom_attributes": testkit.JSON{"plan": "free", "seats": 3},
		})
		contact(t, d, testkit.JSON{"name": "Cy"})
		d.Owner.PatchJSON("/api/v1/contacts/"+ada, testkit.JSON{
			"company_id": acme, "add_tags": []string{"vip", "beta"},
		}).AssertOK()
		d.Owner.PatchJSON("/api/v1/contacts/"+bea, testkit.JSON{"add_tags": []string{"beta"}}).AssertOK()
		var tags struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		d.Owner.GetJSON("/api/v1/tags").AssertOK().Decode(&tags)
		tagID := map[string]string{}
		for _, tag := range tags.Data {
			tagID[tag.Name] = tag.ID
		}

		assert.Equal(t, []string{"Ada", "Bea"}, matching(t, d, `[{"field":"email","op":"known"}]`))
		assert.Equal(t, []string{"Ada"}, matching(t, d, `[{"field":"email","op":"ends_with","value":"ACME.dev"}]`))
		assert.Equal(t, []string{"Bea", "Cy"}, matching(t, d, `[{"field":"custom.plan","op":"is_not","value":"pro"}]`))
		assert.Equal(t, []string{"Ada"},
			matching(t, d, `[{"field":"custom.seats","op":"gt","value":"10","type":"number"}]`))
		assert.Equal(t, []string{"Ada"}, matching(t, d, `[{"field":"custom.vip","op":"true","type":"boolean"}]`))
		assert.Equal(t, []string{"Ada", "Bea"},
			matching(t, d, `[{"field":"tags","op":"has_any","values":["`+tagID["beta"]+`"]}]`))
		assert.Equal(t, []string{"Ada"},
			matching(t, d, `[{"field":"tags","op":"has_all","values":["`+tagID["beta"]+`","`+tagID["vip"]+`"]}]`))
		assert.Equal(t, []string{"Cy"},
			matching(t, d, `[{"field":"tags","op":"has_none","values":["`+tagID["beta"]+`"]}]`))
		assert.Equal(t, []string{"Ada"}, matching(t, d, `[{"field":"company","op":"is","values":["`+acme+`"]}]`))
		both := `[{"field":"email","op":"contains","value":"a"},` +
			`{"field":"custom.seats","op":"gt","value":"10","type":"number"}]`
		assert.Equal(t, []string{"Ada"}, matching(t, d, both))

		d.TC.Clock.Travel(10 * 24 * time.Hour)
		assert.Equal(t, []string{"Ada", "Bea", "Cy"}, matching(t, d, `[{"field":"created","op":"more_than","days":5}]`))
		assert.Empty(t, matching(t, d, `[{"field":"created","op":"less_than","days":5}]`))
		named := url.QueryEscape(`[{"field":"name","op":"is","value":"ada"}]`)
		d.Owner.GetJSON("/api/v1/contacts?total=true&conditions="+named).AssertOK().AssertJSONPath("meta.total", 1)
	})

	t.Run("refuses conditions it does not know", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		for _, bad := range []string{
			`not json`,
			`[{"field":"password","op":"is","value":"x"}]`,
			`[{"field":"email","op":"matches","value":"x"}]`,
			`[{"field":"email","op":"is"}]`,
			`[{"field":"custom.seats","op":"gt","value":"many","type":"number"}]`,
			`[{"field":"tags","op":"has_any","values":["tag_bogus"]}]`,
			`[{"field":"created","op":"more_than","days":0}]`,
		} {
			d.Owner.GetJSON("/api/v1/contacts?conditions="+url.QueryEscape(bad)).AssertStatus(400).
				AssertJSONPath("code", "invalid_parameter")
		}
	})
}

func TestContactAudienceFilter(t *testing.T) {
	t.Parallel()

	t.Run("keeps people in any of the audiences, or in none, with other conditions", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(t, d, testkit.JSON{
			"name": "Ada", "email": "ada@acme.dev", "custom_attributes": testkit.JSON{"plan": "pro"},
		})
		contact(t, d, testkit.JSON{"name": "Bea", "custom_attributes": testkit.JSON{"plan": "team"}})
		contact(t, d, testkit.JSON{"name": "Cy", "custom_attributes": testkit.JSON{"plan": "free"}})
		audience := func(name, plan string) string {
			return d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": name, "rules": testkit.JSON{
				"rules": testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": plan},
			}}).AssertCreated().String("data.id")
		}
		pro, team := audience("Pro", "pro"), audience("Team", "team")

		assert.Equal(t, []string{"Ada", "Bea"},
			matching(t, d, `[{"field":"audience","op":"in_any","values":["`+pro+`","`+team+`"]}]`))
		none := `[{"field":"audience","op":"in_none","values":["` + pro + `","` + team + `"]}]`
		assert.Equal(t, []string{"Cy"}, matching(t, d, none))
		assert.Equal(t, []string{"Ada"}, matching(t, d,
			`[{"field":"audience","op":"in_any","values":["`+pro+`","`+team+`"]},{"field":"email","op":"known"}]`))

		either := url.QueryEscape(`[{"field":"audience","op":"in_any","values":["` + pro + `","` + team + `"]}]`)
		d.Owner.GetJSON("/api/v1/contacts?limit=1&page=2&sort=name&total=true&conditions="+either).AssertOK().
			AssertJSONPath("meta.total", 2).AssertJSONPath("data.0.name", "Bea")

		for _, bad := range []string{
			`[{"field":"audience","op":"in_any","values":[]}]`,
			`[{"field":"audience","op":"in_any","values":["audn_bogus"]}]`,
			`[{"field":"audience","op":"is","values":["` + pro + `"]}]`,
		} {
			d.Owner.GetJSON("/api/v1/contacts?conditions=" + url.QueryEscape(bad)).AssertStatus(400)
		}
	})
}
