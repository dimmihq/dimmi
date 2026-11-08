package attributes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
definition is one definition as the API lists it.
*/
type definition struct {
	ID             string `json:"id"`
	Of             string `json:"of"`
	Key            string `json:"key"`
	Label          string `json:"label"`
	DataType       string `json:"data_type"`
	Origin         string `json:"origin"`
	Status         string `json:"status"`
	ClientWritable bool   `json:"client_writable"`
}

/*
definitions lists the workspace's definitions by key.
*/
func definitions(t *testing.T, d tests.Desk) map[string]definition {
	t.Helper()
	var page struct {
		Data []definition `json:"data"`
	}
	d.Owner.GetJSON("/api/v1/attribute-definitions").AssertOK().Decode(&page)

	out := map[string]definition{}
	for _, def := range page.Data {
		out[def.Of+"."+def.Key] = def
	}

	return out
}

/*
contact adds a contact with custom attributes and returns its API path.
*/
func contact(d tests.Desk, email string, custom testkit.JSON) string {
	return "/api/v1/contacts/" + d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{
		"email": email, "custom_attributes": custom,
	}).AssertCreated().String("data.id")
}

func TestObservedAttributes(t *testing.T) {
	t.Parallel()

	t.Run("defines a new key from its first trusted value", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(d, "ana@acme.dev", testkit.JSON{"plan": "pro", "seats": 3, "vip": true, "trial_ends_at": "2026-12-01"})

		defs := definitions(t, d)
		assert.Equal(t, "string", defs["contact.plan"].DataType)
		assert.Equal(t, "number", defs["contact.seats"].DataType)
		assert.Equal(t, "boolean", defs["contact.vip"].DataType)
		assert.Equal(t, "date", defs["contact.trial_ends_at"].DataType)
		assert.Equal(t, "observed", defs["contact.plan"].Origin)
		assert.Equal(t, "Trial ends at", defs["contact.trial_ends_at"].Label)
	})

	t.Run("keeps dates as unix seconds, milliseconds included", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := contact(d, "ana@acme.dev", testkit.JSON{"signed_up_at": 1796083200000})

		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.custom_attributes.signed_up_at", float64(1796083200))
	})

	t.Run("drops a value of the wrong type, keeps the rest and counts the problem", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := contact(d, "ana@acme.dev", testkit.JSON{"seats": 3})

		d.Owner.PatchJSON(path, testkit.JSON{"custom_attributes": testkit.JSON{"seats": "many", "plan": "pro"}}).
			AssertOK().AssertJSONPath("data.custom_attributes.seats", float64(3)).
			AssertJSONPath("data.custom_attributes.plan", "pro")
		d.Owner.GetJSON("/api/v1/attribute-problems").AssertOK().AssertJSONPath("data.0.key", "seats").
			AssertJSONPath("data.0.code", "type_mismatch").AssertJSONPath("data.0.sample", "text of 4 characters")
	})

	t.Run("types company attributes apart from contact ones", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON("/api/v1/companies", testkit.JSON{
			"name": "Acme", "domain": "acme.dev", "custom_attributes": testkit.JSON{"plan": 3},
		}).AssertCreated()
		contact(d, "ana@acme.dev", testkit.JSON{"plan": "pro"})

		defs := definitions(t, d)
		assert.Equal(t, "number", defs["company.plan"].DataType)
		assert.Equal(t, "string", defs["contact.plan"].DataType)
	})
}

func TestAttributeDefinitions(t *testing.T) {
	t.Parallel()

	t.Run("defines a list and refuses values outside it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON("/api/v1/attribute-definitions", testkit.JSON{
			"object": "contact", "key": "tier", "label": "Tier", "data_type": "list",
			"options": []string{"gold", "silver"},
		}).AssertCreated().AssertJSONPath("data.origin", "created")

		path := contact(d, "ana@acme.dev", testkit.JSON{"tier": "bronze"})
		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.custom_attributes", map[string]any{})
		d.Owner.GetJSON("/api/v1/attribute-problems").AssertOK().AssertJSONPath("data.0.code", "invalid_option")
	})

	t.Run("refuses bad keys, standard names, repeats and lists without values", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		def := func(key, dataType string, options []string) *testkit.Response {
			return d.Owner.PostJSON("/api/v1/attribute-definitions", testkit.JSON{
				"object": "contact", "key": key, "label": "X", "data_type": dataType, "options": options,
			})
		}

		def("Plan", "string", nil).AssertStatus(422).AssertJSONPath("code", "key_invalid")
		def("email", "string", nil).AssertStatus(422).AssertJSONPath("code", "key_reserved")
		def("tier", "list", nil).AssertStatus(422).AssertJSONPath("code", "options_invalid")
		def("plan", "money", nil).AssertStatus(422)
		def("plan", "string", nil).AssertCreated()
		def("plan", "string", nil).AssertStatus(409).AssertJSONPath("code", "attribute_exists")
	})

	t.Run("archives only what nothing reads, and archived keys take no values", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := contact(d, "ana@acme.dev", testkit.JSON{"plan": "pro"})
		plan := definitions(t, d)["contact.plan"].ID
		audience := d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": "Pro", "rules": testkit.JSON{
			"rules": testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": "pro"},
		}}).AssertCreated().String("data.id")

		d.Owner.PatchJSON("/api/v1/attribute-definitions/"+plan, testkit.JSON{"status": "archived"}).
			AssertStatus(409).AssertJSONPath("code", "attribute_in_use").AssertJSONPath("used_by.0", "Pro")
		d.Owner.Delete("/api/v1/audiences/" + audience).AssertStatus(204)
		d.Owner.PatchJSON("/api/v1/attribute-definitions/"+plan, testkit.JSON{"status": "archived"}).AssertOK()

		d.Owner.PatchJSON(path, testkit.JSON{"custom_attributes": testkit.JSON{"plan": "team"}}).AssertOK().
			AssertJSONPath("data.custom_attributes.plan", "pro")
	})

	t.Run("deletes an archived attribute with its values", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := contact(d, "bo@acme.dev", testkit.JSON{"plan": "pro", "seats": 2})
		plan := definitions(t, d)["contact.plan"].ID

		d.Owner.Delete("/api/v1/attribute-definitions/"+plan).AssertStatus(409).
			AssertJSONPath("code", "attribute_active")
		d.Owner.PatchJSON("/api/v1/attribute-definitions/"+plan, testkit.JSON{"status": "archived"}).AssertOK()
		d.Owner.Delete("/api/v1/attribute-definitions/" + plan).AssertStatus(204)

		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.custom_attributes", map[string]any{"seats": float64(2)})
		assert.NotContains(t, definitions(t, d), "contact.plan")
	})

	t.Run("lets browsers write a contact attribute through the sign-in settings", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(d, "ana@acme.dev", testkit.JSON{"theme": "dark"})
		theme := definitions(t, d)["contact.theme"].ID

		d.Owner.PatchJSON("/api/v1/attribute-definitions/"+theme, testkit.JSON{"client_writable": true}).AssertOK().
			AssertJSONPath("data.client_writable", true)
		require.True(t, definitions(t, d)["contact.theme"].ClientWritable)
		d.Owner.GetJSON("/api/v1/identity-verification/settings").AssertOK().
			AssertJSONPath("data.client_writable_attributes", []any{"theme"})
	})

	t.Run("lets members read definitions but not change them", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/attribute-definitions").AssertOK()
		member.PostJSON("/api/v1/attribute-definitions", testkit.JSON{
			"object": "contact", "key": "plan", "label": "Plan", "data_type": "string",
		}).AssertStatus(403)
	})
}
