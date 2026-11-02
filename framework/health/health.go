/*
|--------------------------------------------------------------------------
| Health Checks
|--------------------------------------------------------------------------
|
| Operators and load balancers ask one question: can this process do its
| work right now? Each check answers for one dependency with pass, warn,
| fail or skip, a detail saying what it saw and, when something is wrong,
| a fix saying what to do about it.
|
| Checks run in parallel and each one is cut off at the timeout, so one
| hung dependency never holds up the answer. A check that panics fails
| instead of taking the process down with it.
|
*/

package health

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Status is the verdict of a check, or of a whole run.
*/
type Status string

/*
The verdicts. Pass means healthy, Warn means working but worth a look,
Fail means broken, and Skip means the check does not apply here.
*/
const (
	Pass Status = "pass"
	Warn Status = "warn"
	Fail Status = "fail"
	Skip Status = "skip"
)

/*
DefaultTimeout bounds a check when the registry is given no timeout.
*/
const DefaultTimeout = 3 * time.Second

/*
Result is what a check found: its status, a detail describing what it
saw, and a fix telling the operator what to do when it did not pass.
*/
type Result struct {
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

/*
Check looks at one dependency.
*/
type Check interface {
	/*
		Name identifies the check in reports, such as "database".
	*/
	Name() string

	/*
		Run checks the dependency and gives up when the context ends.
	*/
	Run(ctx context.Context) Result
}

/*
Func returns a check with the name that runs the function.
*/
func Func(name string, run func(ctx context.Context) Result) Check {
	return funcCheck{name: name, run: run}
}

type funcCheck struct {
	name string
	run  func(ctx context.Context) Result
}

/*
Name returns the name the check was given.
*/
func (c funcCheck) Name() string {
	return c.name
}

/*
Run calls the function.
*/
func (c funcCheck) Run(ctx context.Context) Result {
	return c.run(ctx)
}

/*
Outcome is one check's result in a report, with its name and how long
it ran.
*/
type Outcome struct {
	Name     string
	Result   Result
	Duration time.Duration
}

/*
Report is the result of running every check: the worst status among
them and each outcome, in the order the checks were added.
*/
type Report struct {
	Status   Status
	Outcomes []Outcome
}

/*
Failed returns the names of the checks that failed.
*/
func (r Report) Failed() []string {
	var names []string

	for _, outcome := range r.Outcomes {
		if outcome.Result.Status == Fail {
			names = append(names, outcome.Name)
		}
	}

	return names
}

/*
Registry holds the checks to run.
*/
type Registry struct {
	timeout time.Duration
	clock   clock.Clock

	mu     sync.RWMutex
	checks []Check
}

/*
NewRegistry returns an empty registry whose checks are each cut off at
the timeout, or DefaultTimeout when it is zero. The clock measures how
long each check ran.
*/
func NewRegistry(timeout time.Duration, clk clock.Clock) *Registry {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	return &Registry{timeout: timeout, clock: clk}
}

/*
Add registers checks. A check named like one already registered
replaces it in place.
*/
func (r *Registry) Add(checks ...Check) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, check := range checks {
		index := slices.IndexFunc(r.checks, func(c Check) bool { return c.Name() == check.Name() })
		if index >= 0 {
			r.checks[index] = check

			continue
		}

		r.checks = append(r.checks, check)
	}
}

/*
Run runs every check in parallel and reports what they found. A run
with no checks passes.
*/
func (r *Registry) Run(ctx context.Context) Report {
	r.mu.RLock()
	checks := slices.Clone(r.checks)
	r.mu.RUnlock()

	outcomes := make([]Outcome, len(checks))

	var wg sync.WaitGroup
	for i, check := range checks {
		wg.Go(func() { outcomes[i] = r.runOne(ctx, check) })
	}

	wg.Wait()

	return Report{Status: worst(outcomes), Outcomes: outcomes}
}

/*
runOne runs the check under the timeout. A check that ignores its
context is abandoned when the timeout passes and reported failed.
*/
func (r *Registry) runOne(ctx context.Context, check Check) Outcome {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	start := r.clock.Now()
	done := make(chan Result, 1)

	go func() { done <- guard(ctx, check) }()

	var result Result
	select {
	case result = <-done:
	case <-ctx.Done():
		result = Result{Status: Fail, Detail: fmt.Sprintf("no answer within %s", r.timeout)}
	}

	return Outcome{Name: check.Name(), Result: result, Duration: r.clock.Since(start)}
}

/*
guard runs the check and turns a panic into a failure.
*/
func guard(ctx context.Context, check Check) (result Result) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = Result{Status: Fail, Detail: fmt.Sprintf("check panicked: %v", recovered)}
		}
	}()

	return check.Run(ctx)
}

/*
worst returns Fail if any outcome failed, else Warn if any warned, else
Pass. Skipped checks do not count.
*/
func worst(outcomes []Outcome) Status {
	status := Pass

	for _, outcome := range outcomes {
		switch outcome.Result.Status {
		case Fail:
			return Fail
		case Warn:
			status = Warn
		case Pass, Skip:
		}
	}

	return status
}
