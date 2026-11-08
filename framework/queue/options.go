/*
|--------------------------------------------------------------------------
| Job Options
|--------------------------------------------------------------------------
|
| A job kind carries its own defaults, such as its queue and how often it
| may be tried, so callers push it without repeating them. A caller may
| still override one push, for example to run it later.
|
| Uniqueness keeps a job from piling up: a second push of the same job
| while the first is still waiting is skipped, not added.
|
*/

package queue

import (
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Args is the payload of a job, stored as JSON. Kind names the job type
and picks its worker; it must stay stable while jobs of that kind may
still be waiting. Args may also define Options() Options to give its
kind default options.
*/
type Args interface {
	/*
		Kind names the job type, such as "send_digest".
	*/
	Kind() string
}

/*
Options control how a job is pushed. Zero values keep the defaults.

Queue is the queue it waits in ("default"). Priority runs it ahead of
waiting jobs with a higher number, from 1 to 4 (1). MaxAttempts is how
often it is tried before it is given up (25). ScheduledAt holds it until
that time. Unique skips the push when a matching job already exists.
*/
type Options struct {
	Queue       string
	Priority    int
	MaxAttempts int
	ScheduledAt time.Time
	Unique      UniqueOpts
}

/*
UniqueOpts says which existing job makes a push a duplicate. Nothing is
unique unless at least one field is set.

ByArgs compares the encoded args. ByPeriod compares the time window the
push falls in, such as the same hour. ByState lists the states a
matching job must be in; it defaults to every state except canceled
and discarded, and must include available, pending, running and
scheduled.
*/
type UniqueOpts struct {
	ByArgs   bool
	ByPeriod time.Duration
	ByState  []State
}

/*
State is where a job is in its life.
*/
type State string

/*
The job states. Available jobs wait for a worker, scheduled ones for
their time and retryable ones for their next attempt; pending jobs are
not ready to be scheduled yet.
*/
const (
	StateAvailable = State(rivertype.JobStateAvailable)
	StateCanceled  = State(rivertype.JobStateCancelled)
	StateCompleted = State(rivertype.JobStateCompleted)
	StateDiscarded = State(rivertype.JobStateDiscarded)
	StatePending   = State(rivertype.JobStatePending)
	StateRetryable = State(rivertype.JobStateRetryable)
	StateRunning   = State(rivertype.JobStateRunning)
	StateScheduled = State(rivertype.JobStateScheduled)
)

/*
Opt overrides one option for a single push.
*/
type Opt func(*Options)

/*
OnQueue pushes the job onto the named queue.
*/
func OnQueue(name string) Opt {
	return func(o *Options) { o.Queue = name }
}

/*
Priority sets the job's priority, from 1 (first) to 4.
*/
func Priority(priority int) Opt {
	return func(o *Options) { o.Priority = priority }
}

/*
MaxAttempts sets how often the job is tried before it is given up.
*/
func MaxAttempts(attempts int) Opt {
	return func(o *Options) { o.MaxAttempts = attempts }
}

/*
At holds the job until the given time.
*/
func At(t time.Time) Opt {
	return func(o *Options) { o.ScheduledAt = t }
}

/*
Unique skips the push when a matching job already exists.
*/
func Unique(unique UniqueOpts) Opt {
	return func(o *Options) { o.Unique = unique }
}

/*
insertOpts is [insertOpts] with a held job's time read on the queue's
clock and moved onto the system's.
*/
func (q *Queue) insertOpts(args Args, opts []Opt) *river.InsertOpts {
	out := insertOpts(args, opts)
	if q.clock != nil && !out.ScheduledAt.IsZero() {
		out.ScheduledAt = clock.System().Now().Add(out.ScheduledAt.Sub(q.clock.Now()))
	}

	return out
}

/*
insertOpts merges the kind's default options with the overrides for
this push. The unique states stay nil unless some are given, since
River treats any non-nil list as asking for uniqueness.
*/
func insertOpts(args Args, opts []Opt) *river.InsertOpts {
	var o Options
	if defaults, ok := args.(interface{ Options() Options }); ok {
		o = defaults.Options()
	}

	for _, opt := range opts {
		opt(&o)
	}

	var states []rivertype.JobState
	for _, state := range o.Unique.ByState {
		states = append(states, rivertype.JobState(state))
	}

	return &river.InsertOpts{
		Queue:       o.Queue,
		Priority:    o.Priority,
		MaxAttempts: o.MaxAttempts,
		ScheduledAt: o.ScheduledAt,
		UniqueOpts: river.UniqueOpts{
			ByArgs:   o.Unique.ByArgs,
			ByPeriod: o.Unique.ByPeriod,
			ByState:  states,
		},
	}
}
