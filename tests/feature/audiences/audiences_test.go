package audiences_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
leaf is one condition of a rule.
*/
func leaf(field, op string, value any) testkit.JSON {
	out := testkit.JSON{"field": field, "op": op}
	if value != nil {
		out["value"] = value
	}

	return out
}

/*
all joins conditions that must all hold.
*/
func all(children ...testkit.JSON) testkit.JSON {
	return testkit.JSON{"all": children}
}

/*
save creates a saved audience and returns its id and lock version.
*/
func save(t *testing.T, d tests.Desk, name string, rules testkit.JSON) (string, int) {
	t.Helper()
	res := d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": name, "rules": rules}).AssertCreated()
	lock, err := strconv.Atoi(res.String("data.lock_version"))
	require.NoError(t, err)

	return res.String("data.id"), lock
}

/*
contact adds a contact with custom attributes and returns its id.
*/
func contact(d tests.Desk, email string, custom testkit.JSON) string {
	return d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{
		"email": email, "name": email, "custom_attributes": custom,
	}).AssertCreated().String("data.id")
}

func TestSavedAudiences(t *testing.T) {
	t.Parallel()

	t.Run("creates, lists and saves an audience, refusing an older copy", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, lock := save(t, d, "Pro customers", testkit.JSON{
			"visitors": "identified", "rules": all(leaf("contact.custom.plan", "eq", "pro")),
		})

		d.Owner.GetJSON("/api/v1/audiences").AssertOK().AssertJSONPath("data.0.name", "Pro customers").
			AssertJSONPath("data.0.rules.visitors", "identified")
		d.Owner.PatchJSON("/api/v1/audiences/"+id, testkit.JSON{"name": "Pro", "lock_version": lock}).AssertOK().
			AssertJSONPath("data.lock_version", float64(lock+1))
		d.Owner.PatchJSON("/api/v1/audiences/"+id, testkit.JSON{"name": "Pro 2", "lock_version": lock}).
			AssertStatus(409).AssertJSONPath("code", "edit_conflict")
		d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": "pro"}).AssertStatus(409)
		d.Owner.GetJSON("/api/v1/audiences/"+id).AssertOK().AssertJSONPath("data.used_by.#", float64(0))
	})

	t.Run("refuses rules it cannot read, naming every problem", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		res := d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": "Broken", "rules": testkit.JSON{
			"rules": all(leaf("contact.shoe_size", "eq", "9"), leaf("contact.email", "gt", 3)),
		}}).AssertStatus(422).AssertJSONPath("code", "audience_invalid")
		res.AssertJSONPath("errors.0.code", "unknown_attribute").AssertJSONPath("errors.1.code", "op_invalid_for_type")
	})

	t.Run("refuses missing, looping and too deeply nested audiences", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		in := func(id string) testkit.JSON { return testkit.JSON{"rules": leaf("audience", "in", []string{id})} }

		d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": "Ghost", "rules": in("audn_nope")}).
			AssertStatus(422).AssertJSONPath("errors.0.code", "missing_reference")

		base, _ := save(t, d, "Base", testkit.JSON{"rules": leaf("contact.kind", "eq", "user")})
		one, _ := save(t, d, "One", in(base))
		two, _ := save(t, d, "Two", in(one))
		d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": "Three", "rules": in(two)}).
			AssertStatus(422).AssertJSONPath("errors.0.code", "limit_exceeded")
		d.Owner.PatchJSON("/api/v1/audiences/"+base, testkit.JSON{"rules": in(one)}).
			AssertStatus(422).AssertJSONPath("errors.0.code", "audience_cycle")
	})

	t.Run("refuses to delete an audience another audience or a live workflow uses", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		vip, _ := save(t, d, "VIP", testkit.JSON{"rules": leaf("contact.custom.vip", "is_true", nil)})
		wrapper, _ := save(t, d, "Wrapper", testkit.JSON{"rules": leaf("audience", "in", []string{vip})})

		d.Owner.Delete("/api/v1/audiences/"+vip).AssertStatus(409).AssertJSONPath("code", "audience_in_use")
		d.Owner.Delete("/api/v1/audiences/" + wrapper).AssertStatus(204)

		wf := d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "VIP lane", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"audience": testkit.JSON{"rules": leaf("audience", "in", []string{vip})},
			"steps":    []testkit.JSON{{"id": "s_note0001", "kind": "note", "settings": testkit.JSON{"text": "vip"}}},
		}}).AssertCreated().String("data.id")
		d.Owner.PostJSON("/api/v1/workflows/"+wf+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertOK()

		d.Owner.GetJSON("/api/v1/audiences/"+vip).AssertOK().AssertJSONPath("data.used_by.0.object", "workflow").
			AssertJSONPath("data.used_by.0.live", true)
		d.Owner.Delete("/api/v1/audiences/"+vip).AssertStatus(409).AssertJSONPath("used_by.0", "VIP lane")
	})

	t.Run("lets members read and preview audiences but not change them", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, _ := save(t, d, "Everyone", testkit.JSON{})
		member, _ := d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/audiences/" + id).AssertOK()
		member.PostJSON("/api/v1/audience-previews", testkit.JSON{"rules": testkit.JSON{}}).AssertOK()
		member.PostJSON("/api/v1/audiences", testkit.JSON{"name": "Mine"}).AssertStatus(403)
		member.Delete("/api/v1/audiences/" + id).AssertStatus(403)
	})
}

func TestAudiencePreviews(t *testing.T) {
	t.Parallel()

	t.Run("counts the people a rule matches with a sample", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(d, "ana@acme.dev", testkit.JSON{"plan": "pro", "seats": 12})
		contact(d, "bo@acme.dev", testkit.JSON{"plan": "free", "seats": 2})
		contact(d, "cy@other.dev", testkit.JSON{"plan": "pro", "seats": 30})

		res := d.Owner.PostJSON("/api/v1/audience-previews", testkit.JSON{"rules": testkit.JSON{"rules": all(
			leaf("contact.custom.plan", "eq", "PRO"), leaf("contact.custom.seats", "gte", 10),
			leaf("contact.email_domain", "eq", "acme.dev"),
		)}}).AssertOK()

		res.AssertJSONPath("data.count", float64(1)).AssertJSONPath("data.total_contacts", float64(3)).
			AssertJSONPath("data.approximate", false).AssertJSONPath("data.sample.0.email", "ana@acme.dev")
	})

	t.Run("matches on tags and saved audiences, and keeps a saved count", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", testkit.JSON{})
		contact(d, "bo@acme.dev", testkit.JSON{})
		tag := d.Owner.PatchJSON("/api/v1/contacts/"+ana, testkit.JSON{"add_tags": []string{"beta"}}).AssertOK().
			String("data.tags.0.id")
		beta, _ := save(t, d, "Beta", testkit.JSON{"rules": leaf("contact.tags", "has", tag)})

		d.Owner.PostJSON("/api/v1/audience-previews", testkit.JSON{
			"rules": testkit.JSON{"rules": leaf("audience", "not_in", []string{beta})},
		}).AssertOK().AssertJSONPath("data.count", float64(1)).AssertJSONPath("data.sample.0.email", "bo@acme.dev")

		d.Owner.PostJSON("/api/v1/audiences/"+beta+"/previews", nil).AssertOK().AssertJSONPath("data.count", float64(1))
		d.Owner.GetJSON("/api/v1/audiences").AssertOK().AssertJSONPath("data.0.last_estimate.count", float64(1))
	})

	t.Run("counts page-only matches apart and warns about likely mistakes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(d, "ana@acme.dev", testkit.JSON{})

		res := d.Owner.PostJSON("/api/v1/audience-previews", testkit.JSON{"rules": testkit.JSON{"rules": testkit.JSON{
			"any": []testkit.JSON{
				leaf("contact.email", "neq", "a@b.c"), leaf("contact.email", "neq", "d@e.f"),
				leaf("page.path", "starts_with", "/pricing"),
			},
		}}}).AssertOK()

		res.AssertJSONPath("data.count", float64(1)).AssertJSONPath("data.warnings.0.code", "negative_or").
			AssertJSONPath("data.warnings.1.code", "page_rule_is_public")

		d.Owner.PostJSON("/api/v1/audience-previews", testkit.JSON{
			"rules": testkit.JSON{"rules": leaf("page.path", "starts_with", "/pricing")},
		}).AssertOK().AssertJSONPath("data.count", float64(0)).AssertJSONPath("data.pages", float64(1))
	})

	t.Run("lists the fields a rule may read, with custom attributes in use", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		contact(d, "ana@acme.dev", testkit.JSON{"plan": "pro", "trial_ends_at": "2026-12-01T00:00:00Z", "seats": 3})

		var page struct {
			Data []struct {
				Key  string   `json:"key"`
				Type string   `json:"type"`
				Ops  []string `json:"ops"`
			} `json:"data"`
		}
		d.Owner.GetJSON("/api/v1/audience-fields").AssertOK().Decode(&page)

		types := map[string]string{}
		for _, f := range page.Data {
			types[f.Key] = f.Type
		}

		assert.Equal(t, "string", types["contact.custom.plan"])
		assert.Equal(t, "date", types["contact.custom.trial_ends_at"])
		assert.Equal(t, "number", types["contact.custom.seats"])
		assert.Equal(t, "url", types["page.path"])
		assert.NotContains(t, types, "conversation.status")
	})
}

func TestAudienceEvaluations(t *testing.T) {
	t.Parallel()

	rules := testkit.JSON{"rules": all(
		leaf("contact.custom.plan", "in", []string{"pro", "team"}),
		leaf("page.path", "starts_with", "/billing"),
	)}

	t.Run("explains every condition with the person's own values", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ana := contact(d, "ana@acme.dev", testkit.JSON{"plan": "pro"})

		d.Owner.PostJSON("/api/v1/audience-evaluations", testkit.JSON{"rules": rules, "contact_id": ana}).AssertOK().
			AssertJSONPath("data.result", "depends_on_page").AssertJSONPath("data.leaves.0.actual", "pro").
			AssertJSONPath("data.leaves.0.trust", "verified").AssertJSONPath("data.leaves.0.result", "true").
			AssertJSONPath("data.leaves.1.result", "unknown")

		d.Owner.PostJSON("/api/v1/audience-evaluations", testkit.JSON{
			"rules": rules, "contact_id": ana, "context": testkit.JSON{"page_url": "https://app.acme.dev/billing/plan"},
		}).AssertOK().AssertJSONPath("data.result", "match").AssertJSONPath("data.leaves.1.actual", "/billing/plan")
	})

	t.Run("says no when a condition fails and refuses unknown people", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		bo := contact(d, "bo@acme.dev", testkit.JSON{"plan": "free"})

		d.Owner.PostJSON("/api/v1/audience-evaluations", testkit.JSON{"rules": rules, "contact_id": bo}).AssertOK().
			AssertJSONPath("data.result", "no_match")
		d.Owner.PostJSON("/api/v1/audience-evaluations", testkit.JSON{"rules": rules, "contact_id": "ctc_nope"}).
			AssertStatus(404)
	})
}

func TestWorkflowsWithSavedAudiences(t *testing.T) {
	t.Parallel()

	t.Run("runs a workflow only for people in a saved audience", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acme, _ := save(t, d, "Acme", testkit.JSON{"rules": leaf("contact.email_domain", "eq", "acme.dev")})
		wf := d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Acme lane", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"audience": testkit.JSON{"rules": leaf("audience", "in", []string{acme})},
			"steps": []testkit.JSON{
				{"id": "s_note0001", "kind": "note", "settings": testkit.JSON{"text": "acme customer"}},
			},
		}}).AssertCreated().String("data.id")
		d.Owner.PostJSON("/api/v1/workflows/"+wf+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertOK()

		start := func(email string) []string {
			id := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
				"inbox_id": d.InboxID(), "author": "contact", "subject": "Help", "body": "Hi",
				"contact": testkit.JSON{"email": email},
			}).AssertCreated().String("data.id")
			d.TC.Drain()

			var page struct {
				Data []struct {
					Body *string `json:"body"`
				} `json:"data"`
			}
			d.Owner.GetJSON("/api/v1/conversations/" + id + "/messages?order=asc").AssertOK().Decode(&page)
			var out []string
			for _, m := range page.Data {
				if m.Body != nil {
					out = append(out, *m.Body)
				}
			}

			return out
		}

		assert.Contains(t, start("ana@acme.dev"), "acme customer")
		assert.NotContains(t, start("bo@other.dev"), "acme customer")
	})

	t.Run("refuses to publish a workflow naming a missing audience", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		wf := d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Ghost", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"audience": testkit.JSON{"rules": leaf("audience", "in", []string{"audn_01jbgone"})},
			"steps":    []testkit.JSON{},
		}}).AssertCreated().String("data.id")

		d.Owner.PostJSON("/api/v1/workflows/"+wf+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertStatus(422)
	})
}
