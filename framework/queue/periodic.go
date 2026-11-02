/*
|--------------------------------------------------------------------------
| Periodic Jobs
|--------------------------------------------------------------------------
|
| Some jobs are pushed on a clock rather than by an action: a nightly
| cleanup, an hourly digest. Every worker node knows the same periodic
| jobs, but only the one elected leader pushes them, so each run happens
| once however many nodes are up. When the leader goes away another node
| is elected and carries on.
|
*/

package queue

import (
	"time"

	"github.com/riverqueue/river"
)

/*
Periodic is a job pushed on a schedule. Next returns the first run
strictly after the given time. Args and Opts are what each run pushes.
RunOnStart also pushes it as soon as a node becomes leader.
*/
type Periodic struct {
	Next       func(after time.Time) time.Time
	Args       Args
	Opts       []Opt
	RunOnStart bool
}

/*
AddPeriodic adds a periodic job. It takes effect on nodes that take part
in leader election, and may be called before or after Start. On a node
configured with NoLeader it does nothing: that node never pushes
periodic jobs, so every node can register the same schedule.
*/
func (q *Queue) AddPeriodic(p Periodic) {
	if q.noLeader {
		return
	}

	q.client.PeriodicJobs().Add(river.NewPeriodicJob(
		schedule(p.Next),
		func() (river.JobArgs, *river.InsertOpts) { return p.Args, insertOpts(p.Args, p.Opts) },
		&river.PeriodicJobOpts{RunOnStart: p.RunOnStart},
	))
}

/*
schedule turns a next-run function into River's schedule.
*/
type schedule func(after time.Time) time.Time

/*
Next returns the first run after the given time.
*/
func (s schedule) Next(after time.Time) time.Time {
	return s(after)
}
