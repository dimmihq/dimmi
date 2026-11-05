package user_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
	pseudo "github.com/dimmihq/dimmi/tests/fixtures/lang"
)

/*
pseudoLocale is the test-only language the fixtures carry.
*/
const pseudoLocale = "en-XA"

/*
withPseudoLocale boots the application serving English and the pseudo
language.
*/
func withPseudoLocale(t *testing.T) *tests.TestCase {
	t.Helper()

	return tests.New(t, tests.WithEnv("DIMMI_APP__LOCALES=en,"+pseudoLocale), tests.WithLanguage(pseudo.Files))
}

func TestUserLocale(t *testing.T) {
	t.Parallel()

	t.Run("an unknown locale is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		for _, locale := range []string{"fr", "tlh", pseudoLocale, ""} {
			owner.PatchJSON("/api/v1/user", testkit.JSON{"locale": locale}).AssertValidationError("locale")
		}
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "locale": nil})
	})

	t.Run("null follows the workspace", func(t *testing.T) {
		t.Parallel()
		tc := withPseudoLocale(t)
		owner := tc.ActingAsOwner().WithHeader("Dimmi-Workspace", workspaceID(tc))

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"locale": pseudoLocale}).AssertOK()
		owner.GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.locale", nil).
			AssertJSONPath("data.locale", pseudoLocale)

		owner.PatchJSON("/api/v1/user", testkit.JSON{"locale": "en"}).AssertOK().AssertJSONPath("data.locale", "en")
		owner.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.locale", "en")

		owner.PatchJSON("/api/v1/user", testkit.JSON{"locale": nil}).AssertOK().AssertJSONPath("data.locale", nil)
		owner.GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.locale", nil).
			AssertJSONPath("data.locale", pseudoLocale)
	})

	t.Run("a person with no choice and no workspace in play reads in their browser's language", func(t *testing.T) {
		t.Parallel()
		tc := withPseudoLocale(t)

		tc.ActingAsOwner().WithHeader("Accept-Language", pseudoLocale+", en;q=0.5").GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.locale", pseudoLocale)
		tc.ActingAsOwner().GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.locale", "en")
	})

	t.Run("validation messages follow the person's language", func(t *testing.T) {
		t.Parallel()
		tc := withPseudoLocale(t)
		owner := tc.ActingAsOwner().WithHeader("Accept-Language", "en")

		owner.PatchJSON("/api/v1/user", testkit.JSON{"timezone": "Mars/Olympus"}).
			AssertValidationError("timezone").
			AssertJSONPath("errors.timezone.0", "The timezone must be a valid time zone.")

		owner.PatchJSON("/api/v1/user", testkit.JSON{"locale": pseudoLocale}).AssertOK().
			AssertJSONPath("data.locale", pseudoLocale)
		owner.PatchJSON("/api/v1/user", testkit.JSON{"timezone": "Mars/Olympus"}).
			AssertValidationError("timezone").
			AssertHeader("Content-Language", pseudoLocale).
			AssertJSONPath("errors.timezone.0", "[Ŧĥé timezone ɱûšţ ƀé å ṽåļîđ ţîɱé žöñé.]")
		owner.WithHeader("Dimmi-Workspace", workspaceID(tc)).
			PatchJSON("/api/v1/workspace", testkit.JSON{"timezone": "Mars/Olympus"}).
			AssertValidationError("timezone").
			AssertJSONPath("errors.timezone.0", "[Ŧĥé timezone ɱûšţ ƀé å ṽåļîđ ţîɱé žöñé.]")
	})

	t.Run("the workspace's language is used by a member who picked none", func(t *testing.T) {
		t.Parallel()
		tc := withPseudoLocale(t)
		owner := tc.ActingAsOwner().WithHeader("Dimmi-Workspace", workspaceID(tc)).WithHeader("Accept-Language", "en")

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"locale": pseudoLocale}).AssertOK()
		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"timezone": "Mars/Olympus"}).
			AssertValidationError("timezone").
			AssertHeader("Content-Language", pseudoLocale)
	})

	t.Run("auth config lists enabled locales", func(t *testing.T) {
		t.Parallel()

		tests.New(t).Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.locales", []map[string]string{{"code": "en", "name": "English"}})
		withPseudoLocale(t).Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.locales.1.code", pseudoLocale).
			AssertJSONPath("data.locales.1.name", "Éñĝļîšĥ (þšéûđö)")
	})
}
