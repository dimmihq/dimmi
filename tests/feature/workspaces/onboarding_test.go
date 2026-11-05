package workspaces_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestOnboarding(t *testing.T) {
	t.Parallel()

	t.Run("starts a new workspace on its first step and records the step reached", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		owner.GetJSON("/api/v1/workspace").AssertOK().
			AssertJSONPath("data.onboarding.step", "workspace").
			AssertJSONPath("data.onboarding.completed_at", nil)

		owner.PatchJSON("/api/v1/workspace/onboarding", testkit.JSON{"step": "tags"}).AssertOK().
			AssertJSONPath("data.onboarding.step", "tags").
			AssertJSONPath("data.onboarding.completed_at", nil)
	})

	t.Run("finishing opens the workspace and keeps the first time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		first := owner.PatchJSON("/api/v1/workspace/onboarding", testkit.JSON{"step": "done", "completed": true}).
			AssertOK().String("data.onboarding.completed_at")
		assert.NotEmpty(t, first)

		tc.Clock.Advance(3600 * 1e9)
		owner.PatchJSON("/api/v1/workspace/onboarding", testkit.JSON{"step": "done", "completed": true}).
			AssertOK().AssertJSONPath("data.onboarding.completed_at", first)
	})

	t.Run("refuses a step that does not exist and a member who cannot change settings", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)

		in(tc, tc.Owner).PatchJSON("/api/v1/workspace/onboarding", testkit.JSON{"step": "tiers"}).
			AssertStatus(422).AssertJSONPath("code", "onboarding_step_unknown")
		in(tc, member).PatchJSON("/api/v1/workspace/onboarding", testkit.JSON{"step": "tags"}).AssertStatus(403)
	})

	t.Run("starts five practice conversations marked as tests", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		in(tc, tc.Owner).PostJSON("/api/v1/workspace/onboarding/test-conversations", nil).
			AssertCreated().AssertJSONPath("data.count", float64(5))

		ws := tc.Workspace.Workspace.ID
		tc.Table("conversations").AssertCount(5, testkit.Row{"workspace_id": ws, "is_test": true})
		tc.Table("contacts").AssertHas(testkit.Row{"email": "emily.johnson@example.com"})
		in(tc, tc.Owner).GetJSON("/api/v1/conversations?filter[view]=all&limit=10").AssertOK().
			AssertJSONPath("data.#", float64(5)).AssertJSONPath("data.0.channel.type", "widget")
	})
}
