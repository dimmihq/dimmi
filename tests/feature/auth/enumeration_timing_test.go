package auth_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/clock"

	"github.com/dimmihq/dimmi/tests"
)

/*
timingRuns is how many sign-ins each path is timed over.
*/
const timingRuns = 50

func TestEnumerationTiming(t *testing.T) {
	t.Parallel()

	t.Run("takes as long for an unknown email as for a wrong password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			behindProxy, "DIMMI_AUTH__LOCKOUT__PER_EMAIL_SOFT_LOCK=1000",
			"DIMMI_HASHING__HASHERS__ARGON2ID__MEMORY=4096", "DIMMI_HASHING__HASHERS__ARGON2ID__ITERATIONS=1",
		))

		unknown, wrong := timings(t, tc, "nobody@example.com", tc.Owner.Email)

		assert.Less(t, (unknown - wrong).Abs(), 10*time.Millisecond, "unknown %s, wrong password %s", unknown, wrong)
	})
}

/*
timings signs in with a wrong password at both emails in turn, each
time from a fresh address so no lockout interferes, and returns the
median time each answer took. Taking turns keeps the load of a busy
machine the same for both.
*/
func timings(t *testing.T, tc *tests.TestCase, first, second string) (time.Duration, time.Duration) {
	t.Helper()

	durations := [2][]time.Duration{}
	system := clock.System()

	for i := range timingRuns {
		for n, email := range []string{first, second} {
			client := from(tc, fmt.Sprintf("198.51.%d.%d", (2*i+n)/250, (2*i+n)%250+1))
			start := system.Now()
			client.PostJSON("/api/v1/sessions", login(email, "wrong-password")).AssertValidationError("email")
			durations[n] = append(durations[n], system.Since(start))
		}
	}

	return median(durations[0]), median(durations[1])
}

/*
median returns the middle duration.
*/
func median(durations []time.Duration) time.Duration {
	slices.Sort(durations)

	return durations[len(durations)/2]
}
