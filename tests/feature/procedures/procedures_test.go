package procedures_test

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
finished is a procedure dimmi can follow.
*/
func finished() testkit.JSON {
	return testkit.JSON{
		"description": "The customer cannot sign in.",
		"steps": []testkit.JSON{
			{"id": "s1", "kind": "instruction", "text": "Ask which email address they sign in with."},
			{"id": "s2", "kind": "condition", "text": "If it is a work address, ask them to check spam first."},
		},
		"handover": []string{"They lost access to the mailbox."},
	}
}

func create(t *testing.T, d tests.Desk, name string) (string, int) {
	t.Helper()
	res := d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": name}).AssertCreated()

	lock, err := strconv.Atoi(res.String("data.lock_version"))
	require.NoError(t, err)

	return res.String("data.id"), lock
}

func TestProcedures(t *testing.T) {
	t.Parallel()

	t.Run("creates an empty draft and saves its working copy", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, lock := create(t, d, "Sign-in help")

		d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"definition": finished(), "lock_version": lock}).
			AssertOK().AssertJSONPath("data.status", "draft").AssertJSONPath("data.definition.steps.#", float64(2)).
			AssertJSONPath("data.lock_version", float64(lock+1))
		d.Owner.GetJSON("/api/v1/procedures").AssertOK().AssertJSONPath("data.0.name", "Sign-in help")
	})

	t.Run("refuses a save from an older copy", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, lock := create(t, d, "Refunds")
		d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"name": "Refunds 2", "lock_version": lock}).AssertOK()

		d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"name": "Refunds 3", "lock_version": lock}).
			AssertStatus(409).AssertJSONPath("code", "edit_conflict")
	})

	t.Run("refuses unknown keys and duplicate names", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, _ := create(t, d, "Billing")
		create(t, d, "Other")

		d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"definition": testkit.JSON{"steps": []any{}, "x": 1}}).
			AssertStatus(422).AssertJSONPath("code", "procedure_invalid")
		d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": "billing"}).AssertStatus(409)
	})

	t.Run("publishes only a finished procedure, listing every problem", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, lock := create(t, d, "Half done")

		d.Owner.PostJSON("/api/v1/procedures/"+id+"/publications", nil).AssertStatus(422).
			AssertJSONPath("code", "procedure_invalid").AssertJSONPath("errors.0.field", "description")

		def := finished()
		steps, ok := def["steps"].([]testkit.JSON)
		require.True(t, ok)
		def["steps"] = append(steps,
			testkit.JSON{"id": "s3", "kind": "action", "text": "Send a code.", "action_id": "aac_nope"})
		d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"definition": def, "lock_version": lock}).AssertOK()
		d.Owner.PostJSON("/api/v1/procedures/"+id+"/publications", nil).AssertStatus(422).
			AssertJSONPath("errors.0.code", "reference_missing").AssertJSONPath("errors.0.step_id", "s3")
	})

	t.Run("publishes, shows changes since, and unpublishes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, lock := create(t, d, "Sign in")
		d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"definition": finished(), "lock_version": lock}).
			AssertOK()

		res := d.Owner.PostJSON("/api/v1/procedures/"+id+"/publications", nil).AssertOK().
			AssertJSONPath("data.status", "live").AssertJSONPath("data.has_unpublished_changes", false)
		published, err := strconv.Atoi(res.String("data.lock_version"))
		require.NoError(t, err)
		d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{
			"name": "Sign in help", "definition": testkit.JSON{"description": "Changed.", "steps": []any{}},
			"lock_version": published,
		}).AssertOK().AssertJSONPath("data.has_unpublished_changes", true)

		d.Owner.Delete("/api/v1/procedures/"+id+"/publications").AssertOK().AssertJSONPath("data.status", "draft")
		d.Owner.Delete("/api/v1/procedures/" + id).AssertStatus(204)
		d.Owner.GetJSON("/api/v1/procedures/" + id).AssertStatus(404)
	})

	t.Run("lets members read procedures but not change them", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id, _ := create(t, d, "Read only")
		member, _ := d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/procedures/" + id).AssertOK()
		member.PostJSON("/api/v1/procedures", testkit.JSON{"name": "Mine"}).AssertStatus(403)
		assert.NotEmpty(t, id)
	})
}
