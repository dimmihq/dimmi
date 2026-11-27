package arch_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	trackerSHA   = "0123456789abcdef0123456789abcdef01234567"
	trackerToken = "fedcba9876543210fedcba9876543210fedcba98"
)

/*
trackerCall is one request the fake tracker received, its body decoded.
*/
type trackerCall struct {
	Method, Path, Auth, ContentType string
	Body                            map[string]any
}

/*
fakeTracker records every request and answers each with the status the
answer function gives for it.
*/
func fakeTracker(t *testing.T, answer func(method, path string) int) (*httptest.Server, func() []trackerCall) {
	t.Helper()

	var (
		mu    sync.Mutex
		calls []trackerCall
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		var body map[string]any
		assert.NoError(t, json.Unmarshal(raw, &body), "the body is JSON: %s", raw)

		mu.Lock()
		calls = append(calls, trackerCall{
			Method: r.Method, Path: r.URL.Path, Auth: r.Header.Get("Authorization"),
			ContentType: r.Header.Get("Content-Type"), Body: body,
		})
		mu.Unlock()

		w.WriteHeader(answer(r.Method, r.URL.Path))
		_, _ = w.Write([]byte(`{"detail":"from the fake"}`))
	}))
	t.Cleanup(server.Close)

	return server, func() []trackerCall {
		mu.Lock()
		defer mu.Unlock()

		return append([]trackerCall(nil), calls...)
	}
}

/*
runTracker runs deploy/release/tracker.sh with only the environment given,
returning its combined output and whether it succeeded.
*/
func runTracker(t *testing.T, env []string, args ...string) (string, bool) {
	t.Helper()

	script := filepath.Join("..", "..", "deploy", "release", "tracker.sh")
	//nolint:gosec // the test's own script
	cmd := exec.CommandContext(t.Context(), "bash", append([]string{script}, args...)...)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin", "GITHUB_REPOSITORY=dimmihq/dimmi"}
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()

	return string(out), err == nil
}

func created(string, string) int { return http.StatusCreated }

func TestDeployTrackerRegistersAStagingRelease(t *testing.T) {
	t.Parallel()

	server, calls := fakeTracker(t, created)
	out, ok := runTracker(t, []string{"TRACKER_TOKEN=" + trackerToken, "TRACKER_URL=" + server.URL, "GITHUB_RUN_ID=42"},
		"staging", trackerSHA)
	require.True(t, ok, out)
	assert.NotContains(t, out, trackerToken, "the token is never printed")

	got := calls()
	require.Len(t, got, 3)

	for _, call := range got {
		assert.Equal(t, "Bearer "+trackerToken, call.Auth)
		assert.Equal(t, "application/json", call.ContentType)
	}

	projects := []any{"api", "dashboard", "website"}
	commit := "https://github.com/dimmihq/dimmi/commit/" + trackerSHA

	assert.Equal(t, "POST", got[0].Method)
	assert.Equal(t, "/api/0/organizations/_/releases/", got[0].Path)
	assert.Equal(t, trackerSHA, got[0].Body["version"])
	assert.Equal(t, projects, got[0].Body["projects"])
	assert.Equal(t, trackerSHA, got[0].Body["ref"])
	assert.Equal(t, commit, got[0].Body["url"])
	assert.NotEmpty(t, got[0].Body["dateReleased"])

	assert.Equal(t, "PUT", got[1].Method)
	assert.Equal(t, "/api/0/organizations/_/releases/"+trackerSHA+"/", got[1].Path)
	assert.Equal(t, trackerSHA, got[1].Body["ref"])
	assert.Equal(t, commit, got[1].Body["url"])
	assert.NotEmpty(t, got[1].Body["dateReleased"])

	assert.Equal(t, "POST", got[2].Method)
	assert.Equal(t, "/api/0/organizations/_/releases/"+trackerSHA+"/deploys/", got[2].Path)
	assert.Equal(t, "staging", got[2].Body["environment"])
	assert.Equal(t, trackerSHA[:12], got[2].Body["name"])
	assert.Equal(t, "https://github.com/dimmihq/dimmi/actions/runs/42", got[2].Body["url"])
	assert.Equal(t, projects, got[2].Body["projects"])
}

func TestDeployTrackerNamesTheProductionVersion(t *testing.T) {
	t.Parallel()

	server, calls := fakeTracker(t, created)
	out, ok := runTracker(t, []string{"TRACKER_TOKEN=" + trackerToken, "TRACKER_URL=" + server.URL},
		"production", trackerSHA, "v1.2.3")
	require.True(t, ok, out)

	got := calls()
	require.Len(t, got, 3)

	tag := "https://github.com/dimmihq/dimmi/releases/tag/v1.2.3"
	assert.Equal(t, trackerSHA, got[0].Body["version"], "the release stays the sha the reports carry")
	assert.Equal(t, "v1.2.3", got[1].Body["ref"])
	assert.Equal(t, tag, got[1].Body["url"])
	assert.Equal(t, "production", got[2].Body["environment"])
	assert.Equal(t, "v1.2.3", got[2].Body["name"])
	assert.Equal(t, tag, got[2].Body["url"])
}

func TestDeployTrackerGoesOnWhenTheReleaseExists(t *testing.T) {
	t.Parallel()

	server, calls := fakeTracker(t, func(method, path string) int {
		if method == "POST" && path == "/api/0/organizations/_/releases/" {
			return http.StatusAlreadyReported
		}

		return http.StatusOK
	})
	out, ok := runTracker(t, []string{"TRACKER_TOKEN=" + trackerToken, "TRACKER_URL=" + server.URL},
		"staging", trackerSHA)
	require.True(t, ok, out)
	assert.Len(t, calls(), 3)
}

func TestDeployTrackerFailsOnAServerError(t *testing.T) {
	t.Parallel()

	server, calls := fakeTracker(t, func(string, string) int { return http.StatusInternalServerError })
	out, ok := runTracker(t, []string{"TRACKER_TOKEN=" + trackerToken, "TRACKER_URL=" + server.URL},
		"staging", trackerSHA)
	assert.False(t, ok)
	assert.Contains(t, out, "create the release: POST /api/0/organizations/_/releases/ answered 500")
	assert.Contains(t, out, "from the fake")
	assert.NotContains(t, out, trackerToken)
	assert.Len(t, calls(), 1, "nothing is sent after a failure")
}

func TestDeployTrackerSkipsWithoutAToken(t *testing.T) {
	t.Parallel()

	server, calls := fakeTracker(t, created)
	out, ok := runTracker(t, []string{"TRACKER_URL=" + server.URL}, "staging", trackerSHA)
	require.True(t, ok, out)
	assert.Contains(t, out, "no TRACKER_TOKEN")
	assert.Empty(t, calls())
}

func TestDeployTrackerRefusesBadArguments(t *testing.T) {
	t.Parallel()

	server, calls := fakeTracker(t, created)
	env := []string{"TRACKER_TOKEN=" + trackerToken, "TRACKER_URL=" + server.URL}

	for _, args := range [][]string{
		{"staging", "abc123"},
		{"Staging Two", trackerSHA},
		{"production", trackerSHA, "1.2.3"},
	} {
		out, ok := runTracker(t, env, args...)
		assert.False(t, ok, "%v: %s", args, out)
		assert.True(t, strings.HasPrefix(out, "tracker: "), out)
	}

	assert.Empty(t, calls())
}
