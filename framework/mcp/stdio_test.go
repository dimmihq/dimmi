package mcp_test

import (
	"context"
	"io"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
serveOverPipes serves the server over in-memory pipes in the context
and connects a protocol client to the other ends. Serve's result arrives
on the returned channel.
*/
func serveOverPipes(ctx context.Context, t *testing.T, s *mcp.Server) (*sdk.ClientSession, <-chan error) {
	t.Helper()

	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()

	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, serverIn, serverOut) }()

	cs, err := sdk.NewClient(&sdk.Implementation{Name: "agent", Version: "1.0.0"}, nil).
		Connect(ctx, &sdk.IOTransport{Reader: clientIn, Writer: clientOut}, nil)
	require.NoError(t, err)

	return cs, done
}

func TestServe(t *testing.T) {
	t.Parallel()

	s := newServer(t, nil)
	user := ids.New()

	t.Run("serve the caller and toolsets of the context", func(t *testing.T) {
		t.Parallel()

		p := &auth.Identity{User: user, Via: "token", Granted: []string{"records:read"}}
		ctx := http.WithPrincipal(t.Context(), p)
		ctx, err := s.WithToolsets(ctx, "records")
		require.NoError(t, err)

		cs, _ := serveOverPipes(ctx, t, s)
		t.Cleanup(func() { _ = cs.Close() })

		assert.ElementsMatch(t, []string{"crash", "echo", "fail", "whoami", "records_read"}, toolNames(t, cs))
		assert.Equal(t, user.String(), structured(t, call(t, cs, "whoami", nil))["id"])
	})

	t.Run("hide features needing abilities from a context without a caller", func(t *testing.T) {
		t.Parallel()

		ctx, err := s.WithToolsets(t.Context(), mcp.AllToolsets)
		require.NoError(t, err)

		cs, _ := serveOverPipes(ctx, t, s)
		t.Cleanup(func() { _ = cs.Close() })

		assert.ElementsMatch(t, []string{"crash", "echo", "fail", "whoami", "reports_view"}, toolNames(t, cs))
	})

	t.Run("stop when the context ends", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cs, done := serveOverPipes(ctx, t, s)
		t.Cleanup(func() { _ = cs.Close() })

		cancel()

		require.ErrorIs(t, <-done, context.Canceled)
	})

	t.Run("name the toolsets features were registered under", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"records", "reports"}, s.Toolsets())
	})
}
