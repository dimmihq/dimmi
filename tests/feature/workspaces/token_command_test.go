package workspaces_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
mint runs token:create for the test case's owner with the extra
arguments.
*/
func mint(tc *tests.TestCase, extra ...string) (int, string) {
	tc.T.Helper()

	args := append([]string{
		"token:create", "--email", strings.ToUpper(tc.Owner.Email), "--workspace-slug", tc.Workspace.Workspace.Slug,
		"--name", "Terraform",
	}, extra...)

	return tc.Command(args...)
}

/*
secretOf returns the last line the command printed, where the secret
is.
*/
func secretOf(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")

	return strings.TrimSpace(lines[len(lines)-1])
}

func TestTokenCreateCommand(t *testing.T) {
	t.Parallel()

	t.Run("prints a token once that works against the api", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		code, out := mint(tc, "--abilities", "members:read, ", "--expires", "90d")
		require.Equal(t, 0, code, out)

		secret := secretOf(out)
		assert.True(t, strings.HasPrefix(secret, "dmi_"), out)
		tc.Table("personal_access_tokens").AssertHas(testkit.Row{"name": "Terraform"})
		tc.Client().WithHeader("Authorization", "Bearer "+secret).GetJSON("/api/v1/me").AssertOK()
	})

	t.Run("prints the token as json", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		code, out := mint(tc, "--abilities", "*", "--expires", "720h", "--json")
		require.Equal(t, 0, code, out)

		var made struct {
			ID        string  `json:"id"`
			Name      string  `json:"name"`
			ExpiresAt *string `json:"expires_at"`
			Token     string  `json:"token"`
		}
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &made))
		assert.Equal(t, "Terraform", made.Name)
		assert.NotNil(t, made.ExpiresAt)
		assert.True(t, strings.HasPrefix(made.Token, "dmi_"))
		assert.True(t, strings.HasPrefix(made.ID, "tok_"))

		code, out = mint(tc, "--abilities", "members:read")
		require.Equal(t, 0, code, out)
		tc.Table("personal_access_tokens").AssertCount(2, testkit.Row{"name": "Terraform"})
	})

	t.Run("refuses missing flags, a bad expiry, strangers and unknown workspaces", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		stranger := tc.CreateUser()

		for _, args := range [][]string{
			{"token:create", "--email", tc.Owner.Email},
			{"token:create", "--email", tc.Owner.Email, "--workspace-slug", "x", "--name", "n", "--abilities", " , "},
		} {
			code, out := tc.Command(args...)
			assert.Equal(t, 1, code)
			assert.Contains(t, out, "required")
		}

		for _, expiry := range []string{"soon", "-5d", "0h", "xd"} {
			code, out := mint(tc, "--abilities", "members:read", "--expires", expiry)
			assert.Equal(t, 1, code)
			assert.Contains(t, out, "--expires takes days")
		}

		code, out := tc.Command("token:create", "--email", "nobody@example.com", "--workspace-slug",
			tc.Workspace.Workspace.Slug, "--name", "n", "--abilities", "members:read")
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "no account has this email")

		code, out = tc.Command("token:create", "--email", stranger.Email, "--workspace-slug",
			tc.Workspace.Workspace.Slug, "--name", "n", "--abilities", "members:read")
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "Workspace not found")

		code, out = mint(tc, "--abilities", "members:read")
		assert.Equal(t, 0, code, out)

		code, _ = tc.Command("token:create", "--email", tc.Owner.Email, "--workspace-slug", "nowhere",
			"--name", "n", "--abilities", "members:read")
		assert.Equal(t, 1, code)

		code, _ = mint(tc, "--abilities", "nonsense:write")
		assert.Equal(t, 1, code)
		tc.Table("personal_access_tokens").AssertCount(1, testkit.Row{"name": "Terraform"})
	})
}
