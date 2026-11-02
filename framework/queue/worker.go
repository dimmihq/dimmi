/*
|--------------------------------------------------------------------------
| Workers
|--------------------------------------------------------------------------
|
| A worker does the work of one job kind. A job may run more than once:
| after a crash, a timeout or a failed attempt it runs again, so a worker
| must be safe to repeat.
|
| A worker that returns an error is retried later with a growing delay,
| until the job runs out of attempts. It can also say what happens next:
| wait without using up an attempt, give up for good, or retry at a time
| it knows, such as one a rate-limited service asked for.
|
*/

package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/riverqueue/river"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Worker runs jobs of one kind.
*/
type Worker[A Args] interface {
	/*
		Handle does the job's work. Returning nil completes the job; an
		error, or one of Postpone, Cancel and RetryAfter, says what
		happens next.
	*/
	Handle(ctx context.Context, job *Job[A]) error
}

/*
Job is one run of a job: its id, args and queue, which attempt this is
out of how many, and when the job was pushed.
*/
type Job[A Args] struct {
	ID          int64
	Args        A
	Queue       string
	Attempt     int
	MaxAttempts int
	CreatedAt   time.Time
}

/*
Register makes the worker run jobs of its kind. Register every worker
before Start; registering a kind twice is an error.
*/
func Register[A Args](q *Queue, worker Worker[A]) error {
	adapter := &riverWorker[A]{worker: worker, retries: map[int64]time.Duration{}}
	if err := river.AddWorkerSafely(q.workers, adapter); err != nil {
		var args A

		return fmt.Errorf("register %s: %w", args.Kind(), err)
	}

	return nil
}

/*
Postpone runs the job again after the duration, without using up an
attempt. It suits waiting for something outside the job to be ready.
*/
func Postpone(d time.Duration) error {
	return river.JobSnooze(d)
}

/*
Cancel gives the job up for good, recording the error. It suits a job
that can never succeed, such as one whose row is gone.
*/
func Cancel(err error) error {
	return river.JobCancel(err)
}

/*
RetryAfter fails this attempt and retries the job after the duration
instead of the usual growing delay. The attempt counts, so a job out of
attempts is given up.
*/
func RetryAfter(d time.Duration) error {
	return &retryAfterError{delay: d}
}

/*
retryAfterError carries the delay RetryAfter asked for until River asks
the worker when to retry.
*/
type retryAfterError struct {
	delay time.Duration
}

func (e *retryAfterError) Error() string {
	return fmt.Sprintf("retry after %s", e.delay)
}

/*
riverWorker runs a Worker under River. River asks for the next retry
time right after the job returns, so the delay a RetryAfter asked for is
kept by job id between the two calls.
*/
type riverWorker[A Args] struct {
	river.WorkerDefaults[A]

	worker  Worker[A]
	mu      sync.Mutex
	retries map[int64]time.Duration
}

/*
Work hands the job to the worker and remembers a RetryAfter delay when
the job has attempts left.
*/
func (w *riverWorker[A]) Work(ctx context.Context, job *river.Job[A]) error {
	err := w.worker.Handle(ctx, &Job[A]{
		ID:          job.ID,
		Args:        job.Args,
		Queue:       job.Queue,
		Attempt:     job.Attempt,
		MaxAttempts: job.MaxAttempts,
		CreatedAt:   job.CreatedAt,
	})

	if retry, ok := errors.AsType[*retryAfterError](err); ok && job.Attempt < job.MaxAttempts {
		w.mu.Lock()
		w.retries[job.ID] = retry.delay
		w.mu.Unlock()
	}

	return err
}

/*
NextRetry returns the time a RetryAfter asked for, or the zero time to
let River's backoff decide.
*/
func (w *riverWorker[A]) NextRetry(job *river.Job[A]) time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()

	delay, ok := w.retries[job.ID]
	if !ok {
		return time.Time{}
	}

	delete(w.retries, job.ID)

	return clock.System().Now().Add(delay)
}
