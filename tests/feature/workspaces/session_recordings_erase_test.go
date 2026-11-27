package workspaces_test

import (
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
erased is one call the fake recorder received.
*/
type erased struct {
	Method, Path, Authorization, ContentType string
	Body                                     map[string]any
}

/*
fakeRecorder is a kimmi server that answers the erase calls with the
statuses it is given in turn, the last one again once they run out.
*/
type fakeRecorder struct {
	URL string

	mu       sync.Mutex
	statuses []int
	calls    []erased
}

func newFakeRecorder(t *testing.T, statuses ...int) *fakeRecorder {
	t.Helper()

	f := &fakeRecorder{statuses: statuses}
	f.URL = testkit.Serve(t, fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		raw, _ := io.ReadAll(r.Body)
		call := erased{
			Method: r.Method, Path: r.URL.Path, Authorization: r.Header.Get("Authorization"),
			ContentType: r.Header.Get("Content-Type"),
		}
		_ = json.Unmarshal(raw, &call.Body)

		f.mu.Lock()
		status := f.statuses[min(len(f.calls), len(f.statuses)-1)]
		f.calls = append(f.calls, call)
		f.mu.Unlock()

		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"deleted":2}`))
	}))

	return f
}

func (f *fakeRecorder) received() []erased {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]erased(nil), f.calls...)
}

/*
recordingOn boots the application with the fake recorder as its session
recorder, project proj_1 and a write token.
*/
func recordingOn(t *testing.T, f *fakeRecorder) *tests.TestCase {
	t.Helper()

	return tests.New(t, tests.WithEnv(
		"DIMMI_FRONTEND__SESSIONS_HOST="+f.URL,
		"DIMMI_FRONTEND__SESSIONS_KEY=km_pub_0123456789abcdef",
		"DIMMI_FRONTEND__SESSIONS_PROJECT_ID=proj_1",
		"DIMMI_FRONTEND__SESSIONS_TOKEN=km_sec_test",
		"DIMMI_HTTP_CLIENT__ALLOW_PRIVATE_NETWORKS=true",
	))
}

func TestEraseSessionRecordings(t *testing.T) {
	t.Parallel()

	t.Run("a purge erases the workspace's recordings, retrying while the recorder fails", func(t *testing.T) {
		t.Parallel()
		f := newFakeRecorder(t, fhttp.StatusServiceUnavailable, fhttp.StatusOK)
		tc := recordingOn(t, f)
		ws := tc.Workspace.Workspace
		overdue(t, tc)

		tc.Drain()
		assert.Empty(t, f.received(), "trashing alone erases nothing")

		push(tc, jobs.PurgeWorkspaceArgs{WorkspaceID: ws.ID})

		tc.Table("workspaces").AssertCount(0, testkit.Row{"id": ws.ID})
		tc.Table("river_job").AssertCount(1, testkit.Row{"kind": "purge_workspace", "state": "completed"})
		tc.Table("river_job").AssertCount(1, testkit.Row{"kind": "erase_session_recordings", "attempt": 1})
		tc.Table("river_job").AssertCount(0, testkit.Row{"kind": "erase_session_recordings", "state": "completed"})

		// The retry waits for its backoff; the test makes it due now.
		exec(t, tc, `UPDATE river_job SET state = 'available', scheduled_at = now()
			WHERE kind = 'erase_session_recordings' AND state IN ('retryable', 'available')`)
		tc.Drain()

		tc.Table("river_job").AssertCount(1, testkit.Row{"kind": "erase_session_recordings", "state": "completed"})

		calls := f.received()
		require.Len(t, calls, 2)

		prefix := models.PublicID(models.PrefixWorkspace, ws.ID) + ":"
		assert.True(t, strings.HasPrefix(prefix, "ws_"))

		for _, call := range calls {
			assert.Equal(t, fhttp.MethodPost, call.Method)
			assert.Equal(t, "/api/v1/projects/proj_1/visitors/erase", call.Path)
			assert.Equal(t, "Bearer km_sec_test", call.Authorization)
			assert.Contains(t, call.ContentType, "application/json")
			assert.Equal(t, map[string]any{"externalIdPrefix": prefix}, call.Body)
		}
	})

	t.Run("a purge queues no erase without a recorder", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws := tc.Workspace.Workspace
		overdue(t, tc)

		push(tc, jobs.PurgeWorkspaceArgs{WorkspaceID: ws.ID})

		tc.Table("workspaces").AssertCount(0, testkit.Row{"id": ws.ID})
		tc.Table("river_job").AssertCount(0, testkit.Row{"kind": "erase_session_recordings"})
	})

	t.Run("a recorder without a project id or token gets no erase", func(t *testing.T) {
		t.Parallel()
		f := newFakeRecorder(t, fhttp.StatusOK)
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_FRONTEND__SESSIONS_HOST="+f.URL,
			"DIMMI_FRONTEND__SESSIONS_KEY=km_pub_0123456789abcdef",
			"DIMMI_FRONTEND__SESSIONS_PROJECT_ID=proj_1",
			"DIMMI_HTTP_CLIENT__ALLOW_PRIVATE_NETWORKS=true",
		))
		ws := tc.Workspace.Workspace
		overdue(t, tc)

		push(tc, jobs.PurgeWorkspaceArgs{WorkspaceID: ws.ID})
		push(tc, jobs.EraseSessionRecordingsArgs{WorkspaceID: ws.ID})

		tc.Table("workspaces").AssertCount(0, testkit.Row{"id": ws.ID})
		tc.Table("river_job").AssertCount(1, testkit.Row{"kind": "erase_session_recordings", "state": "completed"})
		assert.Empty(t, f.received())
	})

	t.Run("the job fails on a 5xx and completes on a 2xx", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
		require.NoError(t, err)
		tc := tests.New(t)
		ws := tc.Workspace.Workspace

		for status, fails := range map[int]bool{
			fhttp.StatusBadGateway: true, fhttp.StatusUnauthorized: true,
			fhttp.StatusOK: false, fhttp.StatusNoContent: false,
		} {
			f := newFakeRecorder(t, status)
			job := &jobs.EraseSessionRecordings{
				HTTP: client, URL: f.URL + "/api/v1/projects/proj_1/visitors/erase", Token: "km_sec_test",
				Log: tc.App.Log,
			}

			err := job.Handle(t.Context(), &queue.Job[jobs.EraseSessionRecordingsArgs]{
				Args: jobs.EraseSessionRecordingsArgs{WorkspaceID: ws.ID}, Attempt: 1, MaxAttempts: 12,
			})
			if fails {
				require.Error(t, err, status)
				assert.NotContains(t, err.Error(), "km_sec_test")
			} else {
				require.NoError(t, err, status)
			}

			assert.Len(t, f.received(), 1, status)
		}
	})

	t.Run("the job retries for a while", func(t *testing.T) {
		t.Parallel()
		assert.Greater(t, jobs.EraseSessionRecordingsArgs{}.Options().MaxAttempts, 5)
	})
}
