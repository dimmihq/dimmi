package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
rules saves the workspace's first messenger's rules.
*/
func rules(t *testing.T, tc *tests.TestCase, body testkit.JSON) *testkit.Response {
	t.Helper()

	return owner(tc).PatchJSON(settingsPath(t, tc)+"/audiences", body)
}

/*
cond is one condition of a rule.
*/
func cond(field, op string, value any) testkit.JSON {
	return testkit.JSON{"field": field, "op": op, "value": value}
}

func TestMessengerRules(t *testing.T) {
	t.Parallel()

	t.Run("sends no surface when the messenger has no rules", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		body := tc.Client().GetJSON(configPath(t, tc)).AssertOK().Body
		assert.NotContains(t, string(body), `"surface"`)
		v := startVisitor(t, tc)
		assert.NotContains(t, string(v.client().GetJSON("/widget/v1/session").AssertOK().Body), `"surface"`)
	})

	t.Run("tells anonymous visitors only the page conditions left to check", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		rules(t, tc, testkit.JSON{"launcher": testkit.JSON{"rules": testkit.JSON{"any": []testkit.JSON{
			cond("contact.custom.secret_plan", "eq", "platinum"),
			cond("page.path", "starts_with", "/pricing"),
		}}}}).AssertOK()

		res := tc.Client().GetJSON(configPath(t, tc)).AssertOK()
		res.AssertJSONPath("data.surface.anonymous.launcher.field", "page.path").
			AssertJSONPath("data.surface.anonymous.start", true)
		assert.NotContains(t, string(res.Body), "secret_plan")
		assert.NotContains(t, string(res.Body), "platinum")
	})

	t.Run("expands saved audiences into page conditions without naming them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		saved := owner(tc).PostJSON("/api/v1/audiences", testkit.JSON{"name": "Pricing readers", "rules": testkit.JSON{
			"rules": cond("page.path", "starts_with", "/pricing"),
		}}).AssertCreated().String("data.id")
		in := testkit.JSON{"rules": cond("audience", "in", []string{saved})}
		rules(t, tc, testkit.JSON{"launcher": in}).AssertOK()

		res := tc.Client().GetJSON(configPath(t, tc)).AssertOK()
		res.AssertJSONPath("data.surface.anonymous.launcher.field", "page.path")
		assert.NotContains(t, string(res.Body), saved)
		assert.NotContains(t, string(res.Body), "Pricing readers")
	})

	t.Run("refuses a new conversation the start rule does not allow", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		rules(t, tc, testkit.JSON{"start": testkit.JSON{"visitors": "identified"}}).AssertOK()
		v := startVisitor(t, tc)

		v.client().GetJSON("/widget/v1/session").AssertOK().AssertJSONPath("data.surface.start", false)
		v.client().PostJSON("/widget/v1/conversations", testkit.JSON{"client_id": ids.New().String(), "body": "Hi"}).
			AssertStatus(403).AssertJSONPath("code", "conversation_start_restricted")
	})

	t.Run("decides a known visitor with what the server knows about them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		rules(t, tc, testkit.JSON{
			"launcher": testkit.JSON{"rules": cond("contact.custom.plan", "eq", "pro")},
			"tabs":     testkit.JSON{"help": testkit.JSON{"rules": cond("contact.tags", "has", "tag_none")}},
		}).AssertOK()
		v := startVisitor(t, tc)

		v.client().GetJSON("/widget/v1/session").AssertOK().AssertJSONPath("data.surface.launcher", false).
			AssertJSONPath("data.surface.tabs.help", false)
		owner(tc).PatchJSON("/api/v1/contacts/"+v.contact, testkit.JSON{
			"custom_attributes": testkit.JSON{"plan": "pro"},
		}).AssertOK()
		v.client().GetJSON("/widget/v1/session").AssertOK().AssertJSONPath("data.surface.launcher", true)
	})

	t.Run("refuses start rules a browser could satisfy", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		saved := owner(tc).PostJSON("/api/v1/audiences", testkit.JSON{"name": "Gmail", "rules": testkit.JSON{
			"rules": cond("contact.email_domain", "eq", "gmail.com"),
		}}).AssertCreated().String("data.id")

		rules(t, tc, testkit.JSON{"start": testkit.JSON{"rules": cond("contact.email", "eq", "a@b.c")}}).
			AssertStatus(422).AssertJSONPath("errors.0.code", "claimed_field_on_access_gate")
		rules(t, tc, testkit.JSON{"start": testkit.JSON{"rules": cond("page.path", "eq", "/a")}}).
			AssertStatus(422).AssertJSONPath("errors.0.code", "field_unavailable_for_surface")
		rules(t, tc, testkit.JSON{"start": testkit.JSON{"rules": cond("audience", "in", []string{saved})}}).
			AssertStatus(422).AssertJSONPath("errors.0.code", "claimed_field_on_access_gate")
		rules(t, tc, testkit.JSON{"tabs": testkit.JSON{"home": testkit.JSON{}}}).AssertStatus(422)
	})

	t.Run("lets only attributes the team's systems set open a start rule", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		rules(t, tc, testkit.JSON{"start": testkit.JSON{"rules": cond("contact.custom.plan", "neq", "free")}}).
			AssertOK()
		v := startVisitor(t, tc)

		v.client().GetJSON("/widget/v1/session").AssertOK().AssertJSONPath("data.surface.start", false)
		owner(tc).PatchJSON("/api/v1/contacts/"+v.contact, testkit.JSON{
			"custom_attributes": testkit.JSON{"plan": "pro"},
		}).AssertOK()
		v.client().GetJSON("/widget/v1/session").AssertOK().AssertJSONPath("data.surface.start", true)
	})

	t.Run("keeps rules left out of a change and removes those sent as null", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		rules(t, tc, testkit.JSON{
			"launcher": testkit.JSON{"visitors": "identified"},
			"tabs":     testkit.JSON{"help": testkit.JSON{"visitors": "leads"}},
		}).AssertOK()

		rules(t, tc, testkit.JSON{"start": testkit.JSON{"visitors": "identified"}, "launcher": nil}).AssertOK().
			AssertJSONPath("data.rules.start.visitors", "identified").
			AssertJSONPath("data.rules.tabs.help.visitors", "leads")
		owner(tc).GetJSON(settingsPath(t, tc)).AssertOK().AssertJSONPath("data.audiences", map[string]any{
			"start": map[string]any{"visitors": "identified"},
			"tabs":  map[string]any{"help": map[string]any{"visitors": "leads"}},
		})
	})

	t.Run("shows the rules in the widget's settings and lets only managers change them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		rules(t, tc, testkit.JSON{"launcher": testkit.JSON{"visitors": "identified"}}).AssertOK()

		owner(tc).GetJSON(settingsPath(t, tc)).AssertOK().
			AssertJSONPath("data.audiences.launcher.visitors", "identified")
		_, member := memberClient(t, tc, enums.RoleMember)
		member.PatchJSON(settingsPath(t, tc)+"/audiences", testkit.JSON{}).AssertStatus(403)
	})
}
