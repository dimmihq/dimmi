package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/models"
)

func TestInboxReopenWindow(t *testing.T) {
	t.Parallel()

	const fallback = 720 * time.Hour

	cases := []struct {
		name     string
		settings string
		want     time.Duration
	}{
		{"uses the inbox's own window", `{"reopen_window_days": 7}`, 7 * 24 * time.Hour},
		{"allows a window longer than the server's", `{"reopen_window_days": 90}`, 90 * 24 * time.Hour},
		{"never reopens at zero", `{"reopen_window_days": 0}`, 0},
		{"falls back when the inbox sets none", `{"assign_on_reply": true}`, fallback},
		{"falls back for empty settings", ``, fallback},
		{"falls back for a null window", `{"reopen_window_days": null}`, fallback},
		{"falls back for settings that do not read", `{"reopen_window_days": "soon"}`, fallback},
		{"falls back for a negative window", `{"reopen_window_days": -1}`, fallback},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			inbox := models.Inbox{Settings: json.RawMessage(c.settings)}

			assert.Equal(t, c.want, inbox.ReopenWindow(fallback))
		})
	}
}
