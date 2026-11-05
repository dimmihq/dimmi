package user_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestUserShow(t *testing.T) {
	t.Parallel()

	t.Run("shows the person their own profile by browser and by token", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().GetJSON("/api/v1/user").AssertOK().
			AssertJSONPath("data.object", "user").
			AssertJSONPath("data.email", tc.Owner.Email)
		tc.ActingAsToken(tc.Workspace.Owner).GetJSON("/api/v1/user").AssertOK().
			AssertJSONPath("data.email", tc.Owner.Email)
	})

	t.Run("refuses anyone not signed in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().GetJSON("/api/v1/user").AssertUnauthorized()
	})
}

func TestUserUpdate(t *testing.T) {
	t.Parallel()

	t.Run("a person changes their name and teammates see it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := teammate(t, tc, enums.RoleMember)
		listener := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, listener.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)

		tc.ActingAs(user).PatchJSON("/api/v1/user", testkit.JSON{"name": "  Ada Lovelace  "}).AssertOK().
			AssertJSONPath("data.name", "Ada Lovelace")

		heard := listener.Publication()
		assert.Equal(t, "user.updated", heard.Event)
		assert.Contains(t, string(heard.Data), models.PublicID(models.PrefixMember, member.ID))
		tc.ActingAsOwner().GetJSON("/api/v1/members/"+models.PublicID(models.PrefixMember, member.ID)).AssertOK().
			AssertJSONPath("data.user.name", "Ada Lovelace")
	})

	t.Run("an empty time zone follows the workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.PatchJSON("/api/v1/user", testkit.JSON{"timezone": "Europe/Lisbon"}).AssertOK().
			AssertJSONPath("data.timezone", "Europe/Lisbon")
		owner.PatchJSON("/api/v1/user", testkit.JSON{"name": "Keeps The Zone"}).AssertOK().
			AssertJSONPath("data.timezone", "Europe/Lisbon")
		owner.PatchJSON("/api/v1/user", testkit.JSON{"timezone": nil}).AssertOK().
			AssertJSONPath("data.timezone", nil)
		owner.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.user.timezone", nil)
	})

	t.Run("an invalid time zone is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().PatchJSON("/api/v1/user", testkit.JSON{"timezone": "Mars/Olympus"}).
			AssertValidationError("timezone")
	})

	t.Run("sets and clears the language, refusing one the server lacks", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.PatchJSON("/api/v1/user", testkit.JSON{"locale": "tlh"}).AssertValidationError("locale")
		owner.PatchJSON("/api/v1/user", testkit.JSON{"locale": "en"}).AssertOK().AssertJSONPath("data.locale", "en")
		owner.PatchJSON("/api/v1/user", testkit.JSON{"locale": nil}).AssertOK()
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "locale": nil})
	})

	t.Run("a name that is too long or empty once cleaned is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.PatchJSON("/api/v1/user", testkit.JSON{"name": strings.Repeat("a", 121)}).AssertValidationError("name")
		owner.PatchJSON("/api/v1/user", testkit.JSON{"name": "\u200b"}).AssertValidationError("name")
	})

	t.Run("a token cannot change the profile", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner).PatchJSON("/api/v1/user", testkit.JSON{"name": "Thief"}).
			AssertForbidden()
	})
}

func TestUserAvatar(t *testing.T) {
	t.Parallel()

	t.Run("a person uploads a picture, which is served re-encoded and square", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		res := owner.PutFile("/api/v1/user/avatar", "avatar", "me.png", pngPicture(t, 800, 300)).AssertOK()
		url := res.String("data.avatar_url")
		require.True(t, strings.HasPrefix(url, "/avatars/"), url)

		tc.Client().GetJSON(url).AssertOK().
			AssertHeader("Content-Type", "image/png").
			AssertHeader("X-Content-Type-Options", "nosniff")
		owner.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.user.avatar_url", url)
	})

	t.Run("replacing a picture removes the old file", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		first := owner.PutFile("/api/v1/user/avatar", "avatar", "a.png", pngPicture(t, 10, 10)).AssertOK().
			String("data.avatar_url")
		second := owner.PutFile("/api/v1/user/avatar", "avatar", "b.png", pngPicture(t, 10, 10)).AssertOK().
			String("data.avatar_url")
		assert.NotEqual(t, first, second)

		tc.Drain()
		tc.Client().GetJSON(first).AssertNotFound()
		tc.Client().GetJSON(second).AssertOK()
	})

	t.Run("taking a picture down brings the initials back and can be repeated", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()
		url := owner.PutFile("/api/v1/user/avatar", "avatar", "a.png", pngPicture(t, 10, 10)).AssertOK().
			String("data.avatar_url")

		owner.Delete("/api/v1/user/avatar").AssertNoContent()
		owner.Delete("/api/v1/user/avatar").AssertNoContent()

		owner.GetJSON("/api/v1/user").AssertOK().AssertJSONPath("data.avatar_url", nil)
		tc.Drain()
		tc.Client().GetJSON(url).AssertNotFound()
	})

	t.Run("refuses what is not a picture of an accepted kind", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.PutFile("/api/v1/user/avatar", "avatar", "a.svg", []byte("<svg></svg>")).
			AssertValidationError("avatar").AssertJSONPath("code", "invalid_avatar")
		owner.PutFile("/api/v1/user/avatar", "other", "a.png", pngPicture(t, 10, 10)).
			AssertValidationError("avatar")
	})

	t.Run("refuses a picture over the limit", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().PutFile("/api/v1/user/avatar", "avatar", "a.png", make([]byte, 3<<20)).
			AssertStatus(fhttp.StatusRequestEntityTooLarge)
	})

	t.Run("only pictures of the stored shape are ever looked up", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().GetJSON("/avatars/secret.txt").AssertNotFound()
		tc.Client().GetJSON("/avatars/00000000-0000-4000-8000-000000000000.png").AssertNotFound()
	})
}
