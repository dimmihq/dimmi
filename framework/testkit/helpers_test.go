package testkit_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
)

/*
recorder records failures instead of failing the test. Fatalf stops the
goroutine it runs in, as the real one does, so code after it never runs.
*/
type recorder struct {
	testing.TB
	mu       sync.Mutex
	failures []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

func (r *recorder) Failures() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.failures...)
}

/*
record runs fn on its own goroutine with a recorder in place of the test
and returns the recorder once fn returns or stops at a fatal failure.
*/
func record(t *testing.T, fn func(tb *recorder)) *recorder {
	t.Helper()

	rec := &recorder{TB: t}
	done := make(chan struct{})

	go func() {
		defer close(done)
		fn(rec)
	}()

	<-done

	return rec
}
