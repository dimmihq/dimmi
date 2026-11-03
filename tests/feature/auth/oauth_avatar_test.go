package auth_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	fimage "github.com/dimmihq/dimmi/framework/image"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
pictureURL is where the provider keeps the person's picture.
*/
const pictureURL = "https://avatars.example.com/u/583231"

/*
picture returns a small PNG.
*/
func picture(t *testing.T) []byte {
	t.Helper()

	canvas := fimage.NewCanvas(8, 8)
	canvas.Fill(color.RGBA{R: 200, A: 255}, image.Rect(0, 0, 8, 8))

	var out bytes.Buffer
	require.NoError(t, canvas.EncodePNG(&out))

	return out.Bytes()
}

/*
signUpWithPicture signs grace@example.com up through GitHub with the
picture at the address, then runs the queued jobs.
*/
func signUpWithPicture(tc *tests.TestCase, address string) {
	tc.T.Helper()

	grace := person("583231", "grace@example.com")
	grace.AvatarURL = address
	tc.SignInWith("github", grace)
	tc.Drain()
}

/*
assertAvatarJob checks the state the picture's job ended in.
*/
func assertAvatarJob(tc *tests.TestCase, state queue.State) {
	tc.T.Helper()

	tc.Table("river_job").AssertCount(1, testkit.Row{"kind": "fetch_user_avatar", "state": string(state)})
}

/*
assertAvatarRetried checks the picture's job failed once and waits to
be tried again.
*/
func assertAvatarRetried(tc *tests.TestCase) {
	tc.T.Helper()

	tc.Table("river_job").AssertCount(1, testkit.Row{
		"kind": "fetch_user_avatar", "state": string(queue.StateAvailable), "attempt": 1,
	})
}

func TestFetchUserAvatar(t *testing.T) {
	t.Parallel()

	t.Run("copies the picture into storage as a fresh image", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		httpclienttest.New(tc.App.HTTP).Stub(pictureURL, httpclient.Response{Body: picture(t)})

		signUpWithPicture(tc, pictureURL)

		user := userByEmail(t, tc, "grace@example.com")
		require.NotNil(t, user.AvatarPath)
		disk, err := tc.App.Storage.Default()
		require.NoError(t, err)
		stored, err := disk.Get(t.Context(), *user.AvatarPath)
		require.NoError(t, err)
		format, err := fimage.Sniff(bytes.NewReader(stored))
		require.NoError(t, err)
		assert.Equal(t, fimage.PNG, format)
	})

	t.Run("gives up on a picture that is gone or not an image", func(t *testing.T) {
		t.Parallel()

		for _, resp := range []httpclient.Response{
			{Status: 404}, {Body: []byte("<svg onload=alert(1)>")},
		} {
			tc := tests.New(t, withProviders())
			httpclienttest.New(tc.App.HTTP).Stub(pictureURL, resp)

			signUpWithPicture(tc, pictureURL)

			assert.Nil(t, userByEmail(t, tc, "grace@example.com").AvatarPath)
			assertAvatarJob(tc, queue.StateCanceled)
		}
	})

	t.Run("never fetches a picture from a private address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())

		signUpWithPicture(tc, "http://127.0.0.1:1/avatar.png")

		assert.Nil(t, userByEmail(t, tc, "grace@example.com").AvatarPath)
		assertAvatarJob(tc, queue.StateCanceled)
	})

	t.Run("tries again later when the provider does not answer", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.WithEnv("DIMMI_HTTP_CLIENT__ALLOW_PRIVATE_NETWORKS=true"))

		signUpWithPicture(tc, "http://127.0.0.1:1/avatar.png")

		assertAvatarRetried(tc)
	})

	t.Run("keeps an avatar the user chose in the meantime", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		httpclienttest.New(tc.App.HTTP).Stub(pictureURL, httpclient.Response{Body: picture(t)})
		grace := person("583231", "grace@example.com")
		grace.AvatarURL = pictureURL
		tc.SignInWith("github", grace)
		_, err := tc.App.DB.Exec(t.Context(), "UPDATE users SET avatar_path = 'avatars/chosen.png'")
		require.NoError(t, err)

		tc.Drain()

		assert.Equal(t, "avatars/chosen.png", *userByEmail(t, tc, "grace@example.com").AvatarPath)
		assertAvatarJob(tc, queue.StateCompleted)
	})

	t.Run("tries again later when the avatar cannot be recorded", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		httpclienttest.New(tc.App.HTTP).Stub(pictureURL, httpclient.Response{Body: picture(t)})
		grace := person("583231", "grace@example.com")
		grace.AvatarURL = pictureURL
		tc.SignInWith("github", grace)
		tc.FailWrites("users", "UPDATE")

		tc.Drain()

		assertAvatarRetried(tc)
	})
}
