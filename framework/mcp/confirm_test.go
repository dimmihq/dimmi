package mcp_test

import (
	"context"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
answering returns client options whose person answers every question
with the action and form values, recording the questions asked.
*/
func answering(action string, content map[string]any, asked *[]string) *sdk.ClientOptions {
	return &sdk.ClientOptions{
		ElicitationHandler: func(_ context.Context, req *sdk.ElicitRequest) (*sdk.ElicitResult, error) {
			*asked = append(*asked, req.Params.Message)

			return &sdk.ElicitResult{Action: action, Content: content}, nil
		},
	}
}

func TestConfirmation(t *testing.T) {
	t.Parallel()

	ts := newHTTP(t, newServer(t, confirmRecords))
	url := ts.URL + "/mcp?toolsets=records"

	t.Run("ask the person through the client and run once they accept", func(t *testing.T) {
		t.Parallel()

		var asked []string
		cs := connect(t, url, "tok_records:write", answering("accept", map[string]any{"delete": true}, &asked))

		res := call(t, cs, "records_delete", map[string]any{"id": "7"})

		assert.Equal(t, []string{"Delete record 7?"}, asked)
		assert.False(t, res.IsError)
		assert.Equal(t, map[string]any{"status": "deleted 7"}, structured(t, res))
	})

	t.Run("not run when the person declines", func(t *testing.T) {
		t.Parallel()

		var asked []string
		cs := connect(t, url, "tok_records:write", answering("decline", nil, &asked))

		res := call(t, cs, "records_delete", map[string]any{"id": "7"})

		assert.Len(t, asked, 1)
		assert.False(t, res.IsError)
		assert.Equal(t, "Not deleted.", text(t, res))
		assert.Equal(t, map[string]any{"status": "declined"}, structured(t, res))
	})

	t.Run("give a client that cannot ask a preview instead of running", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, url, "tok_records:write", nil)

		res := call(t, cs, "records_delete", map[string]any{"id": "7"})

		assert.False(t, res.IsError)
		assert.Contains(t, text(t, res), "confirm: true")
		assert.Equal(t, map[string]any{"status": "confirmation_required"}, structured(t, res))
	})

	t.Run("run when the policy accepts the agent's confirmation", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, url, "tok_records:write", nil)

		res := call(t, cs, "records_delete", map[string]any{"id": "7", "confirm": true})

		assert.Equal(t, map[string]any{"status": "deleted 7"}, structured(t, res))
	})

	t.Run("fail the call with the fault the policy returns", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, url, "tok_records:write", nil)

		res := call(t, cs, "records_delete", map[string]any{"id": "locked", "confirm": true})

		assert.True(t, res.IsError)
		assert.Equal(t, "record_locked", errorCode(t, res))
	})

	t.Run("run every call when the server has no policy", func(t *testing.T) {
		t.Parallel()
		open := newHTTP(t, newServer(t, nil))
		cs := connect(t, open.URL+"/mcp?toolsets=records", "tok_records:write", nil)

		res := call(t, cs, "records_delete", map[string]any{"id": "7"})

		require.False(t, res.IsError)
		assert.Equal(t, map[string]any{"status": "deleted 7"}, structured(t, res))
	})
}
