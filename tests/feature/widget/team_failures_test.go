package widget_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
teamFailure is one request of the team that must answer a server error
once a piece underneath breaks. No visitor is involved, so the widget's
own table can break without its sightings needing a write at shutdown.
*/
type teamFailure struct {
	breaks func(tc *tests.TestCase)
	call   func(t *testing.T, tc *tests.TestCase) *testkit.Response
}

func TestWidgetTeamFailures(t *testing.T) {
	t.Parallel()

	widget := func(t *testing.T, tc *tests.TestCase) string {
		t.Helper()

		return settingsPath(t, tc)
	}

	for name, f := range map[string]teamFailure{
		"listing widgets that cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("widget_settings") },
			func(_ *testing.T, tc *tests.TestCase) *testkit.Response {
				return owner(tc).GetJSON("/api/v1/widget-settings")
			},
		},
		"listing widgets whose channels cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("channel_connections") },
			func(_ *testing.T, tc *tests.TestCase) *testkit.Response {
				return owner(tc).GetJSON("/api/v1/widget-settings")
			},
		},
		"changing a widget that cannot be saved": {
			func(tc *tests.TestCase) { tc.FailWrites("widget_settings", "UPDATE") },
			func(t *testing.T, tc *tests.TestCase) *testkit.Response {
				t.Helper()

				return owner(tc).PatchJSON(widget(t, tc), testkit.JSON{"name": "Shop"})
			},
		},
		"moving a widget while inboxes cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("inboxes") },
			func(t *testing.T, tc *tests.TestCase) *testkit.Response {
				t.Helper()

				return owner(tc).PatchJSON(widget(t, tc), testkit.JSON{"inbox_id": "inbox_01m6m7c2dtewjt56a9y6fgf322"})
			},
		},
		"deleting a widget that cannot be removed": {
			func(tc *tests.TestCase) { tc.FailWrites("widget_settings", "UPDATE", "DELETE") },
			deleteWidget,
		},
		"deleting a widget whose channel cannot be closed": {
			func(tc *tests.TestCase) { tc.FailWrites("channel_connections", "UPDATE") },
			deleteWidget,
		},
		"creating a widget that cannot be counted": {
			func(tc *tests.TestCase) { tc.BreakTable("widget_settings") },
			func(_ *testing.T, tc *tests.TestCase) *testkit.Response {
				return owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{"name": "Shop"})
			},
		},
		"signing up while the first widget cannot be stored": {
			func(tc *tests.TestCase) { tc.FailWrites("widget_settings", "INSERT") },
			func(_ *testing.T, tc *tests.TestCase) *testkit.Response {
				return tc.Client().PostJSON("/api/v1/users", testkit.JSON{
					"name": "Grace", "email": "grace@example.com", "password": "analytical-engine-1843",
					"workspace_name": "Shop",
				})
			},
		},
	} {
		t.Run("answers a server error "+name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)
			f.breaks(tc)

			f.call(t, tc).AssertStatus(fhttp.StatusInternalServerError)
		})
	}
}

func TestWidgetStreamRecheck(t *testing.T) {
	t.Parallel()

	t.Run("a stream closes once its session ended behind its back", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_BROADCASTING__RECHECK=1s"))
		v := startVisitor(t, tc)
		s := v.openStream(t, streamURL(t, tc), nil)
		require.NotNil(t, s.next().Connect)

		_, err := tc.Operator().Exec(t.Context(), "UPDATE widget_sessions SET revoked_at = now()")
		require.NoError(t, err)

		for {
			if p := s.next(); p.Disconnect != nil {
				break
			}
		}
	})

	t.Run("a stream closes once its session is gone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_BROADCASTING__RECHECK=1s"))
		v := startVisitor(t, tc)
		s := v.openStream(t, streamURL(t, tc), nil)
		require.NotNil(t, s.next().Connect)

		_, err := tc.Operator().Exec(t.Context(), "DELETE FROM widget_sessions")
		require.NoError(t, err)

		for {
			if p := s.next(); p.Disconnect != nil {
				break
			}
		}
	})
}

/*
deleteWidget has the owner delete the workspace's first widget.
*/
func deleteWidget(t *testing.T, tc *tests.TestCase) *testkit.Response {
	t.Helper()

	return owner(tc).Delete(settingsPath(t, tc))
}
