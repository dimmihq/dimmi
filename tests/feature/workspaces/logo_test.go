package workspaces_test

import (
	"bytes"
	stdimage "image"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
logoPicture is a blank PNG of the size given.
*/
func logoPicture(t *testing.T, width, height int) []byte {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, stdimage.NewRGBA(stdimage.Rect(0, 0, width, height))))

	return out.Bytes()
}

func TestWorkspaceLogo(t *testing.T) {
	t.Parallel()

	t.Run("an admin uploads a logo, which is served re-encoded and shown on the workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		res := owner.PutFile("/api/v1/workspace/logo", "logo", "logo.png", logoPicture(t, 800, 300)).AssertOK()
		url := res.String("data.logo_url")
		require.True(t, strings.HasPrefix(url, "/avatars/"), url)

		tc.Client().GetJSON(url).AssertOK().AssertHeader("Content-Type", "image/png")
		owner.GetJSON("/api/v1/workspace").AssertOK().AssertJSONPath("data.logo_url", url)
	})

	t.Run("replacing a logo removes the old file", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		first := owner.PutFile("/api/v1/workspace/logo", "logo", "a.png", logoPicture(t, 10, 10)).AssertOK().
			String("data.logo_url")
		second := owner.PutFile("/api/v1/workspace/logo", "logo", "b.png", logoPicture(t, 10, 10)).AssertOK().
			String("data.logo_url")
		assert.NotEqual(t, first, second)

		tc.Drain()
		tc.Client().GetJSON(first).AssertNotFound()
		tc.Client().GetJSON(second).AssertOK()
	})

	t.Run("taking the logo down clears it and can be repeated", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()
		url := owner.PutFile("/api/v1/workspace/logo", "logo", "a.png", logoPicture(t, 10, 10)).AssertOK().
			String("data.logo_url")

		owner.Delete("/api/v1/workspace/logo").AssertNoContent()
		owner.Delete("/api/v1/workspace/logo").AssertNoContent()

		owner.GetJSON("/api/v1/workspace").AssertOK().AssertJSONPath("data.logo_url", nil)
		tc.Drain()
		tc.Client().GetJSON(url).AssertNotFound()
	})

	t.Run("a member who cannot change settings cannot change the logo", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)
		member := in(tc, user)

		member.PutFile("/api/v1/workspace/logo", "logo", "a.png", logoPicture(t, 10, 10)).AssertForbidden()
		member.Delete("/api/v1/workspace/logo").AssertForbidden()
	})

	t.Run("refuses what is not a picture, and a picture over the limit", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.PutFile("/api/v1/workspace/logo", "logo", "a.svg", []byte("<svg></svg>")).
			AssertValidationError("avatar").AssertJSONPath("code", "invalid_avatar")
		owner.PutFile("/api/v1/workspace/logo", "logo", "a.png", make([]byte, 3<<20)).
			AssertStatus(fhttp.StatusRequestEntityTooLarge)
	})
}
