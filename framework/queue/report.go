/*
|--------------------------------------------------------------------------
| Job Failure Reports
|--------------------------------------------------------------------------
|
| River logs every failed attempt and retries it. A job that fails for good,
| on its last attempt, needs a person, so it is reported; an attempt that
| a retry may still fix is only logged. A panic is always a bug and is
| reported on any attempt, with the stack River caught.
|
*/

package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

/*
reportingHandler is River's error handler. It never changes what River
does with the job: retries follow the job's own schedule.
*/
type reportingHandler struct {
	report Reporter
}

/*
HandleError reports the job's error when no attempt is left.
*/
func (h reportingHandler) HandleError(
	ctx context.Context, job *rivertype.JobRow, err error,
) *river.ErrorHandlerResult {
	if h.report != nil && job.Attempt >= job.MaxAttempts {
		h.report(ctx, fmt.Errorf("job %s failed on its last attempt: %w", job.Kind, err))
	}

	return nil
}

/*
HandlePanic reports the job's panic with the stack River caught.
*/
func (h reportingHandler) HandlePanic(
	ctx context.Context, job *rivertype.JobRow, value any, trace string,
) *river.ErrorHandlerResult {
	if h.report != nil {
		h.report(ctx, fmt.Errorf("job %s panicked: %v\n%s", job.Kind, value, trace))
	}

	return nil
}
