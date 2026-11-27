package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportingHandler(t *testing.T) {
	t.Parallel()

	collect := func() (*[]error, reportingHandler) {
		var got []error

		return &got, reportingHandler{report: func(_ context.Context, err error) { got = append(got, err) }}
	}

	t.Run("leaves an attempt a retry may fix to the log", func(t *testing.T) {
		t.Parallel()
		got, h := collect()

		job := &rivertype.JobRow{Kind: "send_mail", Attempt: 1, MaxAttempts: 3}

		h.HandleError(t.Context(), job, errors.New("timeout"))

		assert.Empty(t, *got)
	})

	t.Run("reports the last attempt with the job's kind", func(t *testing.T) {
		t.Parallel()
		got, h := collect()
		cause := errors.New("timeout")

		h.HandleError(t.Context(), &rivertype.JobRow{Kind: "send_mail", Attempt: 3, MaxAttempts: 3}, cause)

		require.Len(t, *got, 1)
		require.ErrorIs(t, (*got)[0], cause)
		assert.ErrorContains(t, (*got)[0], "job send_mail failed on its last attempt")
	})

	t.Run("reports a panic on any attempt, with its stack", func(t *testing.T) {
		t.Parallel()
		got, h := collect()

		job := &rivertype.JobRow{Kind: "send_mail", Attempt: 1, MaxAttempts: 3}

		h.HandlePanic(t.Context(), job, "boom", "goroutine 1")

		require.Len(t, *got, 1)
		assert.ErrorContains(t, (*got)[0], "job send_mail panicked: boom\ngoroutine 1")
	})

	t.Run("reports nothing without a reporter", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, reportingHandler{}.HandlePanic(t.Context(), &rivertype.JobRow{}, "boom", ""))
	})
}
