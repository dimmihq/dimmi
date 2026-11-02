package httpclient_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

func TestRetry(t *testing.T) {
	t.Parallel()

	t.Run("reads a retry after header in seconds or as a date", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)

		cases := map[string]time.Duration{
			"7":                             7 * time.Second,
			" 7 ":                           7 * time.Second,
			"-1":                            0,
			"soon":                          0,
			"":                              0,
			"Tue, 01 Jan 2030 12:01:30 GMT": 90 * time.Second,
			"Tue, 01 Jan 2030 11:00:00 GMT": 0,
		}

		for header, want := range cases {
			assert.Equal(t, want, httpclient.RetryAfter(header, now), header)
		}
	})

	t.Run("tells refusals for good from those that pass with time", func(t *testing.T) {
		t.Parallel()

		for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusGone, 499} {
			assert.True(t, httpclient.Permanent(status), status)
		}

		for _, status := range []int{
			http.StatusOK, http.StatusFound, http.StatusRequestTimeout, http.StatusTooManyRequests,
			http.StatusInternalServerError,
		} {
			assert.False(t, httpclient.Permanent(status), status)
		}
	})
}
