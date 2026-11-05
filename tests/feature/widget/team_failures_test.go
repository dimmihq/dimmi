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
	call   func(tc *tests.TestCase, w widgetRef) *testkit.Response
}

/*
widgetRef names the workspace's first widget, read before anything breaks.
*/
type widgetRef struct {
	path string
	app  string
}

func TestWidgetTeamFailures(t *testing.T) {
	t.Parallel()

	for name, f := range map[string]teamFailure{
		"listing widgets that cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("widget_settings") },
			func(tc *tests.TestCase, _ widgetRef) *testkit.Response {
				return owner(tc).GetJSON("/api/v1/widget-settings")
			},
		},
		"listing widgets whose channels cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("channel_connections") },
			func(tc *tests.TestCase, _ widgetRef) *testkit.Response {
				return owner(tc).GetJSON("/api/v1/widget-settings")
			},
		},
		"changing a widget that cannot be saved": {
			func(tc *tests.TestCase) { tc.FailWrites("widget_settings", "UPDATE") },
			func(tc *tests.TestCase, w widgetRef) *testkit.Response {
				return owner(tc).PatchJSON(w.path, testkit.JSON{"name": "Shop"})
			},
		},
		"moving a widget while inboxes cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("inboxes") },
			func(tc *tests.TestCase, w widgetRef) *testkit.Response {
				return owner(tc).PatchJSON(w.path, testkit.JSON{"inbox_id": "inbox_01m6m7c2dtewjt56a9y6fgf322"})
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
			func(tc *tests.TestCase, _ widgetRef) *testkit.Response {
				return owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{"name": "Shop"})
			},
		},
		"signing up while widgets cannot be counted": {
			func(tc *tests.TestCase) { tc.BreakTable("widget_settings") },
			func(tc *tests.TestCase, _ widgetRef) *testkit.Response {
				return tc.Client().PostJSON("/api/v1/users", testkit.JSON{
					"name": "Grace", "email": "grace@example.com", "password": "analytical-engine-1843",
					"workspace_name": "Shop",
				})
			},
		},
		"reading a widget that cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("widget_settings") },
			func(tc *tests.TestCase, w widgetRef) *testkit.Response {
				return owner(tc).GetJSON(w.path)
			},
		},
		"changing a widget that cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("widget_settings") },
			func(tc *tests.TestCase, w widgetRef) *testkit.Response {
				return owner(tc).PatchJSON(w.path, testkit.JSON{"name": "Shop"})
			},
		},
		"deleting a widget that cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("widget_settings") }, deleteWidget,
		},
		"a messenger's config while its workspace cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("workspaces") }, askConfig,
		},
		"a messenger's config while its channel cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("channel_connections") }, askConfig,
		},
		"the team's availability while members cannot be read": {
			func(tc *tests.TestCase) { tc.BreakTable("workspace_members") },
			func(tc *tests.TestCase, w widgetRef) *testkit.Response {
				return tc.Client().GetJSON("/widget/v1/apps/" + w.app + "/availability")
			},
		},
		"signing up while the first widget cannot be stored": {
			func(tc *tests.TestCase) { tc.FailWrites("widget_settings", "INSERT") },
			func(tc *tests.TestCase, _ widgetRef) *testkit.Response {
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
			w := widgetRef{path: settingsPath(t, tc), app: appID(t, tc)}
			f.breaks(tc)

			f.call(tc, w).AssertStatus(fhttp.StatusInternalServerError)
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
func deleteWidget(tc *tests.TestCase, w widgetRef) *testkit.Response {
	return owner(tc).Delete(w.path)
}

/*
askConfig asks for the widget's config as a visitor's browser does.
*/
func askConfig(tc *tests.TestCase, w widgetRef) *testkit.Response {
	return tc.Client().GetJSON("/widget/v1/apps/" + w.app + "/config")
}
