package workspaces_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

/*
trashWith sends the test case's workspace to the trash through the
browser.
*/
func trashWith(tc *tests.TestCase, browser *testkit.Client) *testkit.Response {
	return browser.Send("DELETE", "/api/v1/workspace", testkit.JSON{"confirm_slug": tc.Workspace.Workspace.Slug})
}

/*
restoreWith takes the test case's workspace out of the trash through
the browser.
*/
func restoreWith(tc *tests.TestCase, browser *testkit.Client) *testkit.Response {
	return browser.PatchJSON("/api/v1/workspaces/"+workspaceID(tc), testkit.JSON{"trashed": false})
}

func TestWorkspaceTrashFailures(t *testing.T) {
	t.Parallel()

	t.Run("keeps the workspace live when trashing it fails", func(t *testing.T) {
		t.Parallel()

		for name, breaks := range map[string]func(tc *tests.TestCase){
			"marking it deleted":  func(tc *tests.TestCase) { tc.FailWrites("workspaces", "UPDATE") },
			"revoking tokens":     func(tc *tests.TestCase) { tc.FailWrites("personal_access_tokens", "UPDATE") },
			"withdrawing invites": func(tc *tests.TestCase) { tc.FailWrites("invitations", "UPDATE") },
			"mailing the owners":  func(tc *tests.TestCase) { tc.FailWrites("river_job", "INSERT") },
			"a refusing listener": refuse[appevents.WorkspaceDeleted],
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				owner := in(tc, tc.Owner)
				breaks(tc)

				trashWith(tc, owner).AssertStatus(500)
				tc.Table("workspaces").AssertHas(testkit.Row{"id": tc.Workspace.Workspace.ID, "deleted_at": nil})
			})
		}
	})

	t.Run("keeps the workspace in the trash when restoring it fails", func(t *testing.T) {
		t.Parallel()

		for name, breaks := range map[string]func(tc *tests.TestCase){
			"marking it restored": func(tc *tests.TestCase) { tc.FailWrites("workspaces", "UPDATE") },
			"mailing the owners":  func(tc *tests.TestCase) { tc.FailWrites("river_job", "INSERT") },
			"a refusing listener": refuse[appevents.WorkspaceRestored],
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				owner := in(tc, tc.Owner)
				trashWith(tc, owner).AssertOK()
				breaks(tc)

				restoreWith(tc, owner).AssertStatus(500)
				tc.Table("workspaces").AssertCount(0, testkit.Row{"id": tc.Workspace.Workspace.ID, "deleted_at": nil})
			})
		}
	})

	t.Run("treats a workspace whose settings cannot be read as being purged", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		trashWith(tc, owner).AssertOK()
		exec(t, tc, `UPDATE workspaces SET settings = '[]'`)

		restoreWith(tc, owner).AssertStatus(410)
	})
}
