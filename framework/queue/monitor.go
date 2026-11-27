/*
|--------------------------------------------------------------------------
| Job Monitors
|--------------------------------------------------------------------------
|
| A job on a clock can fail quietly: the leader dies, the job is never
| pushed, or it runs for hours. A monitored job checks in with the error
| tracker when a run starts and when it ends, so the tracker notices a
| run that is missed, fails or takes too long. Every node runs jobs, so
| every node knows the monitors, leader or not.
|
*/

package queue

import "sync"

/*
Monitor names a job on a clock to the error tracker: its slug, its
five-field cron schedule and the timezone the schedule is read in.
*/
type Monitor struct {
	Slug     string
	Schedule string
	Timezone string
}

/*
CheckIn tells whoever tracks errors, such as telemetry, that a run of
the monitored job started. It returns the function called with the
run's error, or nil, when the run ends. Nil checks in nothing.
*/
type CheckIn func(m Monitor) func(err error)

/*
monitors holds the monitor of each job kind that has one.
*/
type monitors struct {
	checkIn CheckIn
	mu      sync.RWMutex
	byKind  map[string]Monitor
}

/*
add monitors runs of the job kind.
*/
func (m *monitors) add(kind string, monitor Monitor) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.byKind[kind] = monitor
}

/*
start checks in a run of the job kind when it is monitored, and returns
the function that checks in its end.
*/
func (m *monitors) start(kind string) func(err error) {
	m.mu.RLock()
	monitor, ok := m.byKind[kind]
	m.mu.RUnlock()

	if !ok || m.checkIn == nil {
		return func(error) {}
	}

	return m.checkIn(monitor)
}
