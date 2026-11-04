package user_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestUserPreferences(t *testing.T) {
	t.Parallel()

	t.Run("every preference reads as its default until changed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.preferences.send_with_enter", false)
	})

	t.Run("send with enter preference is stored and returned", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.PatchJSON("/api/v1/user", testkit.JSON{"preferences": testkit.JSON{"send_with_enter": true}}).
			AssertOK().AssertJSONPath("data.preferences.send_with_enter", true)
		owner.PatchJSON("/api/v1/user", testkit.JSON{"name": "Keeps Preferences"}).AssertOK().
			AssertJSONPath("data.preferences.send_with_enter", true)
		owner.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.user.preferences.send_with_enter", true)
	})

	t.Run("null returns a preference to its default", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()
		owner.PatchJSON("/api/v1/user", testkit.JSON{"preferences": testkit.JSON{"send_with_enter": true}}).AssertOK()

		owner.PatchJSON("/api/v1/user", testkit.JSON{"preferences": testkit.JSON{"send_with_enter": nil}}).
			AssertOK().AssertJSONPath("data.preferences.send_with_enter", false)
		owner.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.user.preferences.send_with_enter", false)
	})

	t.Run("an unknown preference is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().PatchJSON("/api/v1/user", testkit.JSON{"preferences": testkit.JSON{"autoplay": true}}).
			AssertValidationError("preferences.autoplay")
		tc.ActingAsOwner().PatchJSON("/api/v1/user", testkit.JSON{"preferences": testkit.JSON{"autoplay": nil}}).
			AssertValidationError("preferences.autoplay")
	})

	t.Run("a preference of the wrong kind is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		wrong := testkit.JSON{"preferences": testkit.JSON{"send_with_enter": "yes"}}
		tc.ActingAsOwner().PatchJSON("/api/v1/user", wrong).AssertValidationError("preferences.send_with_enter")
		tc.ActingAsOwner().GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.preferences.send_with_enter", false)
	})

	t.Run("a token cannot change preferences", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner).
			PatchJSON("/api/v1/user", testkit.JSON{"preferences": testkit.JSON{"send_with_enter": true}}).
			AssertForbidden()
	})
}
