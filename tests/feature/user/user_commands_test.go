package user_test

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
emptyInstall makes the test's install look fresh: the account the test
case made is deleted, so nobody counts as a user yet.
*/
func emptyInstall(t *testing.T, tc *tests.TestCase) {
	t.Helper()

	_, err := tc.Operator().Exec(t.Context(), "UPDATE users SET deleted_at = now()")
	require.NoError(t, err)
}

/*
signsIn reports whether the email and password open a session.
*/
func signsIn(tc *tests.TestCase, email, password string) *testkit.Response {
	return tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
		"grant": "password", "email": email, "password": password,
	})
}

func TestUserCreateCommand(t *testing.T) {
	t.Parallel()

	t.Run("makes the first user owner of a new workspace on an empty install", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		emptyInstall(t, tc)

		code, out := tc.Command("user:create", "--email", "ada@example.com", "--name", "Ada Lovelace",
			"--password", "correct horse battery", "--workspace", "Acme Labs")

		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "owner of Acme Labs")
		tc.Table("users").AssertHas(testkit.Row{"email": "ada@example.com", "created_via": "cli"})
		tc.Table("workspaces").AssertHas(testkit.Row{"name": "Acme Labs"})
		signsIn(tc, "ada@example.com", "correct horse battery").AssertCreated()
	})

	t.Run("adds a user to an existing workspace with a role, or on their own", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		slug := tc.Workspace.Workspace.Slug

		code, out := tc.Command("user:create", "--email", "grace@example.com", "--name", "Grace Hopper",
			"--generate-password", "--workspace-slug", slug, "--role", "admin")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "is now admin of /w/"+slug)
		_, password, found := strings.Cut(out, "Password: ")
		require.True(t, found, out)
		password = strings.TrimSpace(password)
		signsIn(tc, "grace@example.com", password).AssertCreated()

		code, out = tc.Command("user:create", "--email", "solo@example.com", "--name", "Solo",
			"--password", "correct horse battery")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "Invite them to a workspace")
	})

	t.Run("adds an existing account to a workspace, and only that", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)

		code, out := tc.Command("user:create", "--email", tc.Owner.Email, "--name", "x", "--password", "x",
			"--workspace-slug", other.Workspace.Slug)
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "is now member of")

		code, out = tc.Command("user:create", "--email", tc.Owner.Email, "--name", "x", "--password", "x",
			"--workspace-slug", other.Workspace.Slug)
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "already")

		code, _ = tc.Command("user:create", "--email", tc.Owner.Email, "--name", "x", "--password", "x")
		assert.Equal(t, 1, code)
	})

	t.Run("refuses missing or short passwords, unknown roles and unknown workspaces", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		for _, args := range [][]string{
			{"--email", "a@example.com", "--name", "A"},
			{"--email", "a@example.com", "--name", "A", "--password", "short"},
			{
				"--email", "a@example.com", "--name", "A", "--generate-password", "--role", "boss",
				"--workspace-slug", tc.Workspace.Workspace.Slug,
			},
			{"--email", "a@example.com", "--name", "A", "--generate-password", "--role", "admin"},
			{"--email", "a@example.com", "--name", "A", "--generate-password", "--workspace-slug", "nowhere"},
		} {
			code, out := tc.Command(append([]string{"user:create"}, args...)...)
			assert.Equal(t, 1, code, out)
		}
	})
}

func TestCreateFirstUser(t *testing.T) {
	t.Parallel()

	t.Run("lets only one of two at once create the first user", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		emptyInstall(t, tc)

		var wg sync.WaitGroup

		errs := make([]error, 2)

		for i := range errs {
			wg.Go(func() {
				_, errs[i] = tc.App.Actions.CreateFirstUser.Handle(t.Context(), users.CreateFirstUserInput{
					Name: "First", Email: []string{"one@example.com", "two@example.com"}[i],
					WorkspaceName: "Acme", Via: "setup",
				})
			})
		}

		wg.Wait()

		failed := 0
		for _, err := range errs {
			if err != nil {
				require.ErrorIs(t, err, exceptions.ErrSetupCompleted)

				failed++
			}
		}

		assert.Equal(t, 1, failed)
		tc.Table("workspaces").AssertCount(1, testkit.Row{"name": "Acme"})
	})
}

func TestUserAccountCommands(t *testing.T) {
	t.Parallel()

	t.Run("lists users with their second factors and workspaces", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, err := factories.WithTwoFactor(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		code, out := tc.Command("user:list", "--json")
		require.Equal(t, 0, code, out)

		var listed []struct {
			Email      string   `json:"email"`
			TwoFactor  []string `json:"two_factor"`
			Workspaces []string `json:"workspaces"`
		}
		_, list, found := strings.Cut(out, "[")
		require.True(t, found, out)
		require.NoError(t, json.Unmarshal([]byte("["+list), &listed))
		require.Len(t, listed, 1)
		assert.Equal(t, []string{"totp"}, listed[0].TwoFactor)
		assert.Equal(t, []string{tc.Workspace.Workspace.Slug}, listed[0].Workspaces)

		code, out = tc.Command("user:list")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, tc.Owner.Email)
	})

	t.Run("prints a reset link that sets a new password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		code, out := tc.Command("user:reset-password", tc.Owner.Email)
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "/reset-password#id=prs_")

		code, _ = tc.Command("user:reset-password", "nobody@example.com")
		assert.Equal(t, 1, code)
	})

	t.Run("sets a password and signs the user out everywhere", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		signedIn := tc.ActingAs(tc.Owner)
		signedIn.GetJSON("/api/v1/me").AssertOK()

		code, out := tc.Command("user:reset-password", tc.Owner.Email, "--password", "a brand new secret")
		require.Equal(t, 0, code, out)
		signedIn.GetJSON("/api/v1/me").AssertUnauthorized()
		signsIn(tc, tc.Owner.Email, "a brand new secret").AssertCreated()

		code, _ = tc.Command("user:reset-password", tc.Owner.Email, "--password", "short")
		assert.Equal(t, 1, code)
	})

	t.Run("verifies an email and signs a user out", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser(factories.Unverified)
		signedIn := tc.ActingAs(tc.Owner)

		code, out := tc.Command("user:verify-email", user.Email)
		require.Equal(t, 0, code, out)
		tc.Table("users").AssertMissing(testkit.Row{"id": user.ID, "email_verified_at": nil})

		code, out = tc.Command("user:logout", tc.Owner.Email)
		require.Equal(t, 0, code, out)
		signedIn.GetJSON("/api/v1/me").AssertUnauthorized()

		for _, name := range []string{"user:verify-email", "user:logout"} {
			code, _ = tc.Command(name, "nobody@example.com")
			assert.Equal(t, 1, code)
		}
	})
}
