/*
|--------------------------------------------------------------------------
| Task Schedule
|--------------------------------------------------------------------------
|
| Recurring work is declared once, in plain words: this job, daily at
| three, in this timezone. Each task becomes a periodic job on the queue,
| so every worker node knows the whole schedule, but only the node the
| queue elects as leader pushes the jobs. A run happens once however many
| nodes are up, and carries on when the leader goes away.
|
| A mistake in a schedule, such as a malformed time, is reported when the
| schedule is applied at boot, not the first time it would have run.
|
*/

package schedule

import (
	"errors"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
ErrInvalidTime is returned by Apply for a DailyAt time that is not
HH:MM on a 24-hour clock.
*/
var ErrInvalidTime = errors.New("schedule: invalid time of day")

/*
Schedule collects the tasks that push jobs on a clock.
*/
type Schedule struct {
	queue *queue.Queue
	tasks []*Task
}

/*
Task is one job pushed on a schedule. It runs every minute until one of
its methods says otherwise, in UTC unless Timezone says otherwise.
*/
type Task struct {
	args       queue.Args
	opts       []queue.Opt
	expr       string
	location   *time.Location
	runOnStart bool
	err        error
}

/*
New returns an empty schedule for the queue.
*/
func New(q *queue.Queue) *Schedule {
	return &Schedule{queue: q}
}

/*
Job adds a task that pushes the args, with the options, on the task's
schedule.
*/
func (s *Schedule) Job(args queue.Args, opts ...queue.Opt) *Task {
	task := &Task{args: args, opts: opts, expr: "* * * * *", location: time.UTC}
	s.tasks = append(s.tasks, task)

	return task
}

/*
Apply checks every task and hands it to the queue as a periodic job.
Call it once at boot, after declaring the tasks. It returns the first
mistake found and then hands over nothing.
*/
func (s *Schedule) Apply() error {
	for _, task := range s.tasks {
		if _, err := task.schedule(); err != nil {
			return err
		}
	}

	for _, task := range s.tasks {
		s.queue.AddPeriodic(queue.Periodic{
			Next:       task.Next,
			Args:       task.args,
			Opts:       task.opts,
			RunOnStart: task.runOnStart,
		})
	}

	return nil
}

/*
Cron runs the task on a standard five-field cron expression, such as
"0 3 * * *" for daily at three.
*/
func (t *Task) Cron(expr string) *Task {
	t.expr = expr

	return t
}

/*
EveryMinute runs the task at the start of every minute.
*/
func (t *Task) EveryMinute() *Task {
	return t.Cron("* * * * *")
}

/*
Hourly runs the task at the start of every hour.
*/
func (t *Task) Hourly() *Task {
	return t.Cron("0 * * * *")
}

/*
Daily runs the task at midnight.
*/
func (t *Task) Daily() *Task {
	return t.Cron("0 0 * * *")
}

/*
DailyAt runs the task every day at the time, written HH:MM on a 24-hour
clock, such as "03:00" or "17:30".
*/
func (t *Task) DailyAt(timeOfDay string) *Task {
	at, err := time.Parse("15:04", timeOfDay)
	if err != nil {
		t.err = fmt.Errorf("%w %q", ErrInvalidTime, timeOfDay)

		return t
	}

	return t.Cron(fmt.Sprintf("%d %d * * *", at.Minute(), at.Hour()))
}

/*
Timezone reads the task's times in the location instead of UTC.
*/
func (t *Task) Timezone(location *time.Location) *Task {
	t.location = location

	return t
}

/*
RunOnStart also runs the task as soon as a node becomes leader, which
is mostly at deploy time.
*/
func (t *Task) RunOnStart() *Task {
	t.runOnStart = true

	return t
}

/*
Next returns the task's first run strictly after the given time, or the
zero time while the schedule is invalid.
*/
func (t *Task) Next(after time.Time) time.Time {
	schedule, err := t.schedule()
	if err != nil {
		return time.Time{}
	}

	return schedule.Next(after.In(t.location))
}

/*
schedule parses the task's cron expression, or returns the mistake made
while declaring the task.
*/
func (t *Task) schedule() (cron.Schedule, error) {
	if t.err != nil {
		return nil, fmt.Errorf("schedule %s: %w", t.args.Kind(), t.err)
	}

	schedule, err := cron.ParseStandard(t.expr)
	if err != nil {
		return nil, fmt.Errorf("schedule %s: cron %q: %w", t.args.Kind(), t.expr, err)
	}

	return schedule, nil
}
