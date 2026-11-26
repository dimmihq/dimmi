package middleware_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
)

func panickingHandler(http.ResponseWriter, *http.Request) {
	panic("boom")
}

func TestRecoverStack(t *testing.T) {
	t.Parallel()

	t.Run("reports the frames of the panic, not of the recovery", func(t *testing.T) {
		t.Parallel()
		var reported error
		errs := http.Errors{Report: func(_ context.Context, err error) { reported = err }}
		h := middleware.Recover(errs)(http.HandlerFunc(panickingHandler))

		serve(h, request(t, http.MethodGet, "/", ""))

		var traced interface{ StackTrace() []uintptr }
		require.ErrorAs(t, reported, &traced)
		frames := runtime.CallersFrames(traced.StackTrace())
		var names []string
		for frame, more := frames.Next(); ; frame, more = frames.Next() {
			names = append(names, frame.Function)
			if !more {
				break
			}
		}
		assert.Contains(t, names, "github.com/dimmihq/dimmi/framework/http/middleware_test.panickingHandler")
	})
}
