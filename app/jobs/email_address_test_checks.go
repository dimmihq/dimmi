/*
|--------------------------------------------------------------------------
| Forwarding Test Timeouts
|--------------------------------------------------------------------------
|
| A forwarding test waits ten minutes for its mail. This job is queued
| for that moment when the test starts and times the test out unless the
| mail came back or bounced first. A daily job drops month-old tests.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
testCheckAttempts is how often a timeout is tried.
*/
const testCheckAttempts = 3

/*
CheckEmailAddressTestArgs names the test to time out.
*/
type CheckEmailAddressTestArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	TestID      ids.UUID `json:"test_id"`
}

/*
Kind names the job.
*/
func (CheckEmailAddressTestArgs) Kind() string {
	return "check_email_address_test"
}

/*
Options try the timeout a few times.
*/
func (CheckEmailAddressTestArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: testCheckAttempts}
}

/*
EmailAddressTestChecker times out one test.
*/
type EmailAddressTestChecker interface {
	/*
		Handle times out the test of the workspace unless it settled.
	*/
	Handle(ctx context.Context, workspaceID, testID ids.UUID) error
}

/*
CheckEmailAddressTest times out a forwarding test.
*/
type CheckEmailAddressTest struct {
	Checker EmailAddressTestChecker
}

/*
Handle times out the test the job names.
*/
func (w *CheckEmailAddressTest) Handle(ctx context.Context, job *queue.Job[CheckEmailAddressTestArgs]) error {
	return w.Checker.Handle(ctx, job.Args.WorkspaceID, job.Args.TestID)
}

/*
PruneEmailAddressTestsArgs names the job; it takes no arguments.
*/
type PruneEmailAddressTestsArgs struct{}

/*
Kind names the job.
*/
func (PruneEmailAddressTestsArgs) Kind() string {
	return "prune_email_address_tests"
}

/*
Options run the prune once; tomorrow's run takes over.
*/
func (PruneEmailAddressTestsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1}
}

/*
EmailAddressTestPruner deletes old tests.
*/
type EmailAddressTestPruner interface {
	/*
		Handle deletes the old tests and returns how many went.
	*/
	Handle(ctx context.Context) (int64, error)
}

/*
PruneEmailAddressTests deletes month-old forwarding tests.
*/
type PruneEmailAddressTests struct {
	Pruner EmailAddressTestPruner
}

/*
Handle prunes the tests.
*/
func (w *PruneEmailAddressTests) Handle(ctx context.Context, _ *queue.Job[PruneEmailAddressTestsArgs]) error {
	_, err := w.Pruner.Handle(ctx)

	return err
}
