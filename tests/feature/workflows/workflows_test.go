package workflows_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
routing is a first-message definition that tags and assigns a team.
*/
func routing(team, tag string) testkit.JSON {
	return testkit.JSON{
		"schema": 1, "trigger": "conversation.first_message",
		"steps": []testkit.JSON{
			{"id": "s_assign1", "kind": "assign", "settings": testkit.JSON{"mode": "team", "team_id": team}},
			{"id": "s_tag0001", "kind": "tag", "settings": testkit.JSON{"tag_ids": []string{tag}}},
		},
	}
}

/*
create makes a workflow through the API and returns its id.
*/
func create(t *testing.T, d tests.Desk, body testkit.JSON) string {
	t.Helper()

	return d.Owner.PostJSON("/api/v1/workflows", body).AssertCreated().String("data.id")
}

func TestWorkflows(t *testing.T) {
	t.Parallel()

	t.Run("creates a draft from a trigger with the trigger's defaults", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		res := d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Route", "trigger": "message.customer"}).
			AssertCreated().AssertJSONPath("data.status", "draft").AssertJSONPath("data.kind", "background").
			AssertJSONPath("data.spec.reentry.mode", "every_time").AssertJSONPath("data.has_unpublished_changes", true)
		assert.Regexp(t, `^wf_`, res.String("data.id"))

		d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "route", "trigger": "message.customer"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "name_taken")
		d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Visits", "trigger": "page.visited"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "unknown_trigger")
	})

	t.Run("saves drafts with lock versions and keeps the trigger", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := create(t, d, testkit.JSON{"name": "Route", "trigger": "conversation.first_message"})

		spec := testkit.JSON{"trigger": "conversation.first_message", "steps": []testkit.JSON{
			{"kind": "priority", "settings": testkit.JSON{"value": "high"}},
		}}
		saved := d.Owner.PatchJSON("/api/v1/workflows/"+id, testkit.JSON{"spec": spec, "lock_version": 1}).AssertOK().
			AssertJSONPath("data.lock_version", 2)
		assert.Regexp(t, `^s_`, saved.String("data.spec.steps.0.id"))

		d.Owner.PatchJSON("/api/v1/workflows/"+id, testkit.JSON{"name": "Late", "lock_version": 1}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "edit_conflict")
		d.Owner.PatchJSON("/api/v1/workflows/"+id, testkit.JSON{"spec": testkit.JSON{"trigger": "note.created"}}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "trigger_immutable")
		d.Owner.PatchJSON("/api/v1/workflows/"+id, testkit.JSON{"spec": testkit.JSON{"stpes": []string{}}}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "workflow_invalid")
	})

	t.Run("shows what stops a draft from going live", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := create(t, d, testkit.JSON{"name": "Broken", "spec": testkit.JSON{
			"schema": 1, "trigger": "note.created",
			"steps": []testkit.JSON{{"id": "s_msg001", "kind": "message", "settings": testkit.JSON{"text": "hi"}}},
		}})

		d.Owner.GetJSON("/api/v1/workflows/"+id).AssertOK().
			AssertJSONPath("data.report.errors.0.code", "step_unavailable_for_trigger").
			AssertJSONPath("data.report.errors.0.step_id", "s_msg001")
		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", nil).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "workflow_invalid").
			AssertJSONPath("errors.0.code", "step_unavailable_for_trigger")
	})

	t.Run("publishes versions, refuses publishing nothing new and rolls back", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		team := d.Owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().String("data.id")
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "billing"}).AssertCreated().String("data.id")
		id := create(t, d, testkit.JSON{"name": "Route", "spec": routing(team, tag)})

		first := d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"note": "first"}).AssertOK().
			AssertJSONPath("data.workflow.status", "live").AssertJSONPath("data.version.number", 1).
			AssertJSONPath("data.workflow.has_unpublished_changes", false)
		v1 := first.String("data.version.id")

		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", nil).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "nothing_to_publish")

		d.Owner.PatchJSON("/api/v1/workflows/"+id, testkit.JSON{"spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{},
		}}).AssertOK().AssertJSONPath("data.has_unpublished_changes", true)
		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"draft_hash": "stale"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "draft_changed")
		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", nil).AssertOK().
			AssertJSONPath("data.version.number", 2)

		d.Owner.PatchJSON("/api/v1/workflows/"+id, testkit.JSON{"restore_version_id": v1, "publish": true}).AssertOK().
			AssertJSONPath("data.spec.steps.0.id", "s_assign1").AssertJSONPath("data.has_unpublished_changes", false)
		versions := d.Owner.GetJSON("/api/v1/workflows/"+id+"/versions").AssertOK().
			AssertJSONPath("data.0.number", 3).AssertJSONPath("data.0.restored_from_version_id", v1)
		assert.Equal(t, "first", versions.String("data.2.note"))
	})

	t.Run("refuses references that name nothing usable", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		team := d.Owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Gone"}).AssertCreated().String("data.id")
		d.Owner.Delete("/api/v1/teams/" + team).AssertStatus(fhttp.StatusNoContent)
		id := create(t, d, testkit.JSON{"name": "Route", "spec": routing(team, "tag_nonsense")})

		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", nil).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("errors.0.code", "reference_missing").
			AssertJSONPath("errors.#", 2)
	})

	t.Run("asks to acknowledge warnings before publishing", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := create(t, d, testkit.JSON{"name": "Claimed", "spec": testkit.JSON{
			"schema": 1, "trigger": "message.customer",
			"audience": testkit.JSON{
				"rules": testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": "vip"},
			},
			"steps": []testkit.JSON{{"id": "s_prio01", "kind": "priority", "settings": testkit.JSON{"value": "high"}}},
		}})

		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", nil).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "warnings_not_acknowledged").
			AssertJSONPath("warnings.0.code", "claimed_field")
		d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).AssertOK()
	})

	t.Run("pauses, resumes, archives and reorders", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		first := create(t, d, testkit.JSON{"name": "First", "trigger": "message.customer"})
		second := create(t, d, testkit.JSON{"name": "Second", "trigger": "message.customer"})

		d.Owner.PatchJSON("/api/v1/workflows/"+first, testkit.JSON{"status": "paused"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "not_published")
		d.Owner.PostJSON("/api/v1/workflows/"+first+"/publications", nil).AssertOK()
		d.Owner.PatchJSON("/api/v1/workflows/"+first, testkit.JSON{"status": "paused"}).AssertOK().
			AssertJSONPath("data.status", "paused")
		d.Owner.PatchJSON("/api/v1/workflows/"+first, testkit.JSON{"status": "live"}).AssertOK().
			AssertJSONPath("data.status", "live").AssertJSONPath("data.paused_at", nil)

		d.Owner.PatchJSON("/api/v1/workflows", testkit.JSON{"order": []string{second}}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "order_stale")
		d.Owner.PatchJSON("/api/v1/workflows", testkit.JSON{"order": []string{second, first}}).AssertOK().
			AssertJSONPath("data.0.id", second)

		d.Owner.Delete("/api/v1/workflows/" + first).AssertStatus(fhttp.StatusNoContent)
		d.Owner.GetJSON("/api/v1/workflows").AssertOK().AssertJSONPath("data.#", 1)
		d.Owner.GetJSON("/api/v1/workflows?archived=true").AssertOK().AssertJSONPath("data.0.status", "paused")
		d.Owner.PatchJSON("/api/v1/workflows/"+first, testkit.JSON{"name": "x"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "workflow_archived")
		d.Owner.PostJSON("/api/v1/workflows/"+first+"/unarchive", nil).AssertOK().
			AssertJSONPath("data.archived", false).AssertJSONPath("data.status", "paused")
	})

	t.Run("refuses archiving a reusable workflow live workflows still call", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		reusable := create(t, d, testkit.JSON{"name": "Tag VIP", "spec": testkit.JSON{
			"schema": 1, "trigger": "reusable",
			"steps": []testkit.JSON{{"id": "s_prio01", "kind": "priority", "settings": testkit.JSON{"value": "high"}}},
		}})
		d.Owner.PostJSON("/api/v1/workflows/"+reusable+"/publications", nil).AssertOK()

		caller := create(t, d, testkit.JSON{"name": "Caller", "spec": testkit.JSON{
			"schema": 1, "trigger": "note.created", "steps": []testkit.JSON{
				{"id": "s_call01", "kind": "run_workflow", "settings": testkit.JSON{"workflow_id": reusable}},
			},
		}})
		d.Owner.PostJSON("/api/v1/workflows/"+caller+"/publications", nil).AssertOK()
		d.Owner.Delete("/api/v1/workflows/"+reusable).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "workflow_in_use")
	})

	t.Run("lets members read but not change workflows", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := create(t, d, testkit.JSON{"name": "Route", "trigger": "message.customer"})
		member, _ := d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/workflows/" + id).AssertOK()
		member.PatchJSON("/api/v1/workflows/"+id, testkit.JSON{"name": "Mine"}).AssertForbidden()
		member.PostJSON("/api/v1/workflows/"+id+"/publications", nil).AssertForbidden()
		member.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Mine", "trigger": "message.customer"}).
			AssertForbidden()
	})

	t.Run("keeps one workspace's workflows from another", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		other := tests.NewDesk(t)
		id := create(t, d, testkit.JSON{"name": "Route", "trigger": "message.customer"})

		other.Owner.GetJSON("/api/v1/workflows/" + id).AssertNotFound()
		other.Owner.GetJSON("/api/v1/workflows").AssertOK().AssertJSONPath("data.#", 0)
		d.Owner.GetJSON("/api/v1/workflows/nonsense").AssertNotFound()
		require.NotEmpty(t, id)
	})
}
