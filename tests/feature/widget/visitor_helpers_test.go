package widget_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
visitor is a browser with a messenger session on the workspace's first
widget.
*/
type visitor struct {
	tc      *tests.TestCase
	token   string
	id      string
	contact string
}

/*
startVisitor starts an anonymous session on the workspace's first widget.
*/
func startVisitor(t *testing.T, tc *tests.TestCase) *visitor {
	t.Helper()

	res := tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": appID(t, tc)}).AssertCreated()

	return &visitor{
		tc: tc, token: res.String("data.token"), id: res.String("data.id"), contact: res.String("data.contact.id"),
	}
}

/*
client returns the visitor's browser, sending their bearer token.
*/
func (v *visitor) client() *testkit.Client {
	return v.tc.Client().WithHeader("Authorization", "Bearer "+v.token)
}

/*
session reads the visitor's session row.
*/
func (v *visitor) session(t *testing.T) models.WidgetSession {
	t.Helper()

	var out models.WidgetSession

	ws := v.tc.Workspace.Workspace.ID
	id, err := v.tc.App.IDs.Decode(string(models.PrefixWidgetSession), v.id)
	require.NoError(t, err)

	inWorkspace(t, v.tc, ws, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.FindWidgetSession(ctx, models.FindWidgetSessionParams{WorkspaceID: ws, ID: id})

		return err
	})

	return out
}
