package user_test

import (
	"bytes"
	stdimage "image"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
pngPicture returns a real picture of the given size.
*/
func pngPicture(t *testing.T, width, height int) []byte {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, stdimage.NewRGBA(stdimage.Rect(0, 0, width, height))))

	return out.Bytes()
}

/*
teammate adds a new person to the test's workspace with the role.
*/
func teammate(t *testing.T, tc *tests.TestCase, role enums.Role) (models.User, models.WorkspaceMember) {
	t.Helper()

	user := tc.CreateUser()
	member, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return user, member
}
