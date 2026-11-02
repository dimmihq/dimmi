package user_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestAccountFailures(t *testing.T) {
	t.Parallel()

	t.Run("stops every credential of a deleted account at once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		token := tc.ActingAsToken(tc.Workspace.Owner, "*")

		require.NoError(t, factories.DeleteAccount(t.Context(), tc.App, tc.Owner))

		browser.GetJSON("/api/v1/me").AssertUnauthorized().AssertJSONPath("code", "unauthenticated")
		token.GetJSON("/api/v1/me").AssertUnauthorized().AssertJSONPath("code", "invalid_token")
	})

	t.Run("answers a plain 500 when the user's workspaces cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		tc.BreakTable("workspace_members")

		browser.GetJSON("/api/v1/me").AssertStatus(500).AssertJSONPath("code", "internal_error")
		browser.WithHeader("Dimmi-Workspace", workspaceID(tc)).GetJSON("/api/v1/me").AssertStatus(500)
	})

	t.Run("answers a plain 500 when the named workspace cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		tc.BreakTable("workspaces")

		browser.WithHeader("Dimmi-Workspace", workspaceID(tc)).GetJSON("/api/v1/me").AssertStatus(500)
	})

	t.Run("answers a plain 500 when credentials cannot be checked", func(t *testing.T) {
		t.Parallel()
		for _, table := range []string{"users", "workspace_members", "personal_access_tokens"} {
			tc := tests.New(t)
			browser := tc.ActingAsOwner()
			token := tc.ActingAsToken(tc.Workspace.Owner, "*")
			tc.BreakTable(table)

			token.GetJSON("/api/v1/me").AssertStatus(500)
			if table == "users" {
				browser.GetJSON("/api/v1/me").AssertStatus(500)
			}
		}
	})

	t.Run("answers a plain 500 when sessions cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		tc.BreakTable("sessions")

		browser.GetJSON("/api/v1/me").AssertStatus(500).AssertJSONPath("code", "internal_error")
	})

	t.Run("ends no session when the ending cannot be logged", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tc.ActingAsOwner()
		otherID := other.GetJSON("/api/v1/me").String("data.auth.session_id")
		browser := tc.ActingAsOwner()
		tc.BreakTable("security_events")

		browser.Delete("/api/v1/sessions/" + otherID).AssertStatus(500)
		browser.Delete("/api/v1/sessions").AssertStatus(500)
	})

	t.Run("refuses to end the current session of a request that has none", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		me, err := tc.App.Actions.LoadMe.Handle(t.Context(), auth.LoadMeInput{Principal: principalOf(t, tc)})
		require.NoError(t, err)

		err = tc.App.Actions.EndSession.Handle(t.Context(), auth.EndSessionInput{
			Principal: me.Principal, SessionID: me.Principal.CredentialID,
		})

		require.Error(t, err)
	})
}

/*
principalOf returns the test case's owner as a browser caller whose
session the request does not carry.
*/
func principalOf(t *testing.T, tc *tests.TestCase) *appauth.Principal {
	t.Helper()

	return &appauth.Principal{
		User: tc.Owner, Kind: appauth.KindSession, CredentialID: ids.New(), Granted: []string{appauth.AllAbilities},
	}
}
