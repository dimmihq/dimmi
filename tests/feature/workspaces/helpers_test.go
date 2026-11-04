package workspaces_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
workspaceID returns the public id of the test case's workspace.
*/
func workspaceID(tc *tests.TestCase) string {
	return models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)
}

/*
memberID returns the public id of the member.
*/
func memberID(m models.WorkspaceMember) string {
	return models.PublicID(models.PrefixMember, m.ID)
}

/*
join adds a new user to the test case's workspace with the role and
returns the user and their membership.
*/
func join(t *testing.T, tc *tests.TestCase, role enums.Role, states ...func(models.User) models.User) (
	models.User, models.WorkspaceMember,
) {
	t.Helper()

	user := tc.CreateUser(states...)
	member, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return user, member
}

/*
in returns a client signed in as the user, working in the test case's
workspace.
*/
func in(tc *tests.TestCase, user models.User) *testkit.Client {
	return tc.ActingAs(user).WithHeader("Dimmi-Workspace", workspaceID(tc))
}

/*
exec runs a statement on the test's database outside any workspace, as
an operator would.
*/
func exec(t *testing.T, tc *tests.TestCase, sql string, args ...any) {
	t.Helper()

	_, err := tc.Operator().Exec(t.Context(), sql, args...)
	require.NoError(t, err)
}

/*
recorder keeps every event of one type the application dispatched after
its transaction committed.
*/
type recorder[E any] struct {
	mu   sync.Mutex
	seen []E
}

/*
record starts recording the events of type E.
*/
func record[E any](tc *tests.TestCase) *recorder[E] {
	r := &recorder[E]{}
	events.Listen(tc.App.Events, events.AfterCommit, func(_ context.Context, e E) error {
		r.mu.Lock()
		defer r.mu.Unlock()

		r.seen = append(r.seen, e)

		return nil
	})

	return r
}

/*
all returns the recorded events.
*/
func (r *recorder[E]) all() []E {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]E{}, r.seen...)
}
