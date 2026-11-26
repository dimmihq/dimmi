/*
|--------------------------------------------------------------------------
| Panic Recovery
|--------------------------------------------------------------------------
|
| A panic in one request must not take the process down with every other
| request in flight. It is turned into a plain 500 for the client, and
| logged and reported with its stack, because it is always a bug. The
| report carries the panicking frames themselves, so an error tracker
| shows where the panic happened rather than where it was caught.
|
*/

package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
Recover answers a panicking request with a bare 500 and logs and
reports the panic with its stack. A deliberate abort of the
response is passed on, as the server expects.
*/
func Recover(errs fhttp.Errors) fhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				p := recover()
				if p == nil {
					return
				}

				if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(p)
				}

				errs.RenderBug(w, r, newPanicError(p))
			}()

			next.ServeHTTP(w, r)
		})
	}
}

/*
panicError is a recovered panic. Its message holds the value and the
stack as text for the log; StackTrace hands the same frames to an error
tracker, which reads that method by name.
*/
type panicError struct {
	value any
	stack []byte
	pcs   []uintptr
}

/*
newPanicError captures the stack of the goroutine that panicked. Called
from the deferred recover, the stack still holds the frames that led to
the panic.
*/
func newPanicError(value any) *panicError {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(3, pcs)

	return &panicError{value: value, stack: debug.Stack(), pcs: pcs[:n]}
}

func (e *panicError) Error() string {
	return fmt.Sprintf("panic: %v\n%s", e.value, e.stack)
}

/*
Unwrap returns the panic's value when it was an error.
*/
func (e *panicError) Unwrap() error {
	err, _ := e.value.(error)

	return err
}

/*
StackTrace returns the program counters of the panicking goroutine,
innermost first.
*/
func (e *panicError) StackTrace() []uintptr {
	return e.pcs
}
