package procedures_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
save writes a procedure's working copy from its current lock version.
*/
func save(t *testing.T, d tests.Desk, id string, def testkit.JSON) {
	t.Helper()
	res := d.Owner.GetJSON("/api/v1/procedures/" + id).AssertOK()
	lock, err := strconv.Atoi(res.String("data.lock_version"))
	require.NoError(t, err)
	d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"definition": def, "lock_version": lock}).AssertOK()
}

/*
live creates, saves and publishes a procedure, returning its id.
*/
func live(t *testing.T, d tests.Desk, name string, def testkit.JSON) string {
	t.Helper()
	id, _ := create(t, d, name)
	save(t, d, id, def)
	d.Owner.PostJSON("/api/v1/procedures/"+id+"/publications", nil).AssertOK()

	return id
}

/*
followsSub is a procedure whose second step follows a sub-procedure.
*/
func followsSub(sub string) testkit.JSON {
	return testkit.JSON{
		"description": "The customer wants a refund.",
		"steps": []testkit.JSON{
			{"id": "s1", "kind": "instruction", "text": "Ask for the order number."},
			{"id": "s2", "kind": "sub_procedure", "text": "", "sub_procedure_id": sub},
		},
		"handover": []string{},
	}
}

func TestProcedureVersions(t *testing.T) {
	t.Parallel()

	t.Run("keeps every publish as a version and restores one", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := live(t, d, "Sign in", finished())

		changed := finished()
		changed["description"] = "The customer cannot sign in, the second time."
		save(t, d, id, changed)
		d.Owner.PostJSON("/api/v1/procedures/"+id+"/publications", testkit.JSON{"change_note": "Clearer"}).AssertOK()

		list := d.Owner.GetJSON("/api/v1/procedures/" + id + "/versions").AssertOK()
		list.AssertJSONPath("data.#", float64(2)).AssertJSONPath("data.0.version", float64(2)).
			AssertJSONPath("data.0.change_note", "Clearer").AssertJSONPath("data.1.version", float64(1)).
			AssertJSONPath("data.0.object", "procedure_version")
		d.Owner.GetJSON("/api/v1/procedures/"+id+"/versions/1").AssertOK().
			AssertJSONPath("data.definition.description", "The customer cannot sign in.")
		d.Owner.GetJSON("/api/v1/procedures/" + id + "/versions/9").AssertStatus(404)

		d.Owner.PostJSON("/api/v1/procedures/"+id+"/versions/1/restore", nil).AssertOK().
			AssertJSONPath("data.definition.description", "The customer cannot sign in.").
			AssertJSONPath("data.has_unpublished_changes", true)
		d.Owner.PostJSON("/api/v1/procedures/"+id+"/versions/1/restore", testkit.JSON{"publish": true}).AssertOK().
			AssertJSONPath("data.has_unpublished_changes", false)
		d.Owner.GetJSON("/api/v1/procedures/"+id+"/versions").AssertOK().AssertJSONPath("data.#", float64(3)).
			AssertJSONPath("data.0.change_note", "Restored version 1")
		d.Owner.PostJSON("/api/v1/procedures/"+id+"/versions/1/restore", testkit.JSON{"lock_version": 1}).
			AssertStatus(409)
	})

	t.Run("pages versions newest first", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := live(t, d, "Paged", finished())
		for range 2 {
			d.Owner.PostJSON("/api/v1/procedures/"+id+"/publications", nil).AssertOK()
		}

		res := d.Owner.GetJSON("/api/v1/procedures/" + id + "/versions?limit=2").AssertOK()
		res.AssertJSONPath("data.#", float64(2)).AssertJSONPath("data.1.version", float64(2))
		d.Owner.GetJSON("/api/v1/procedures/"+id+"/versions?before=2").AssertOK().AssertJSONPath("data.#", float64(1))
	})
}

func TestSubProcedures(t *testing.T) {
	t.Parallel()

	t.Run("follows only a live sub-procedure and refuses loops", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		sub := live(t, d, "Verify identity", finished())
		parentID, _ := create(t, d, "Refunds")
		save(t, d, parentID, followsSub(sub))
		d.Owner.PostJSON("/api/v1/procedures/"+parentID+"/publications", nil).AssertOK()

		draftID, _ := create(t, d, "Draft only")
		other, _ := create(t, d, "Uses a draft")
		save(t, d, other, followsSub(draftID))
		d.Owner.PostJSON("/api/v1/procedures/"+other+"/publications", nil).AssertStatus(422).
			AssertJSONPath("errors.0.code", "reference_missing").AssertJSONPath("errors.0.step_id", "s2")

		save(t, d, sub, followsSub(parentID))
		res := d.Owner.PostJSON("/api/v1/procedures/"+sub+"/publications", nil).AssertStatus(422)
		assert.Contains(t, res.String("errors.0.message"), "lead back")
	})

	t.Run("keeps a sub-procedure live procedures follow", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		sub := live(t, d, "Verify identity", finished())
		parent := live(t, d, "Refunds", followsSub(sub))

		d.Owner.Delete("/api/v1/procedures/"+sub+"/publications").AssertStatus(409).
			AssertJSONPath("code", "procedure_in_use").AssertJSONPath("parents.0.id", parent)
		d.Owner.Delete("/api/v1/procedures/" + sub).AssertStatus(409)

		d.Owner.Delete("/api/v1/procedures/" + parent + "/publications").AssertOK()
		d.Owner.Delete("/api/v1/procedures/" + sub + "/publications").AssertOK()
	})

	t.Run("checks every reference and the rules of branches", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, _ := create(t, d, "Chips")
		save(t, d, id, testkit.JSON{
			"description": "Chips everywhere.",
			"steps": []testkit.JSON{
				{"id": "s1", "kind": "instruction", "text": "Read [[article:art_01jzzzzzzzzzzzzzzzzzzzzzzz]]."},
				{
					"id": "s2", "kind": "branch", "text": "Pro plan",
					"condition": testkit.JSON{"mode": "all", "rules": []testkit.JSON{
						{"field": "contact.attr.plan", "operator": "eq", "value": "pro"},
					}},
					"then_steps": []testkit.JSON{
						{"id": "s3", "kind": "instruction", "text": "Hand over to [[handoff:team_nope]]."},
					},
					"else_steps": []testkit.JSON{
						{"id": "s4", "kind": "instruction", "text": "Tag it [[tag:tag_nope]]."},
					},
				},
			},
			"handover": []string{},
		})
		res := d.Owner.PostJSON("/api/v1/procedures/"+id+"/publications", nil).AssertStatus(422)
		steps := map[string]bool{}
		for i := range 3 {
			steps[res.String("errors."+strconv.Itoa(i)+".step_id")] = true
		}
		assert.Equal(t, map[string]bool{"s1": true, "s3": true, "s4": true}, steps)
	})
}

func TestProcedureTestCases(t *testing.T) {
	t.Parallel()

	t.Run("saves, changes, lists and removes tests", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, _ := create(t, d, "Refunds")
		base := "/api/v1/procedures/" + id + "/test-cases"

		created := d.Owner.PostJSON(base, testkit.JSON{
			"name":         "Inside the window",
			"persona":      testkit.JSON{"verified": true, "attributes": testkit.JSON{"plan": "pro"}},
			"messages":     []testkit.JSON{{"text": "I want a refund for ORD-102"}},
			"action_mocks": testkit.JSON{"aac_x": testkit.JSON{"status": 200, "body": testkit.JSON{"days": 3}}},
			"assertions":   []testkit.JSON{{"type": "action_called", "action": "lookup_order"}},
		}).AssertCreated()
		created.AssertJSONPath("data.object", "procedure_test_case").AssertJSONPath("data.persona.verified", true).
			AssertJSONPath("data.last_result", nil)
		test := created.String("data.id")

		d.Owner.PatchJSON(base+"/"+test, testkit.JSON{"name": "Renamed"}).AssertOK().
			AssertJSONPath("data.name", "Renamed").AssertJSONPath("data.messages.#", float64(1))
		d.Owner.GetJSON(base).AssertOK().AssertJSONPath("data.#", float64(1))
		d.Owner.GetJSON(base+"/"+test).AssertOK().AssertJSONPath("data.assertions.0.type", "action_called")

		d.Owner.PostJSON(base, testkit.JSON{"name": "Bad", "assertions": []testkit.JSON{{"type": "nope"}}}).
			AssertStatus(422).AssertJSONPath("code", "procedure_test_case_invalid")
		d.Owner.PostJSON(base, testkit.JSON{"messages": []testkit.JSON{{"text": "hi"}}}).AssertStatus(422)

		d.Owner.Delete(base + "/" + test).AssertStatus(204)
		d.Owner.GetJSON(base + "/" + test).AssertStatus(404)
	})
}

func TestProcedureScorecard(t *testing.T) {
	t.Parallel()

	t.Run("answers an empty scorecard and report for a procedure nobody used", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := live(t, d, "Quiet", finished())

		d.Owner.GetJSON("/api/v1/procedures/"+id+"/metrics?days=7").AssertOK().
			AssertJSONPath("data.object", "procedure_metrics").AssertJSONPath("data.days", float64(7)).
			AssertJSONPath("data.matched_conversations", float64(0)).AssertJSONPath("data.ai_resolution_rate", nil)
		d.Owner.GetJSON("/api/v1/procedures/"+id+"/metrics?version=1").AssertOK().
			AssertJSONPath("data.version", float64(1))
		d.Owner.GetJSON("/api/v1/reports/procedures").AssertOK().AssertJSONPath("data.#", float64(0))
	})
}
