/*
|--------------------------------------------------------------------------
| Checking Sending Domains
|--------------------------------------------------------------------------
|
| A sending domain is checked from the queue, never inside a request: a
| check reads DNS and asks the provider, both of which can be slow. A
| scheduler runs every minute and queues the domains whose check is due;
| each domain has at most one check waiting, however often it is asked.
|
*/

package jobs

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of domain checks: how often one is tried, and how many domains one
scheduler run queues.
*/
const (
	domainCheckAttempts = 3
	domainCheckBatch    = 500
)

/*
CheckEmailDomainArgs names the domain to check.
*/
type CheckEmailDomainArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	DomainID    ids.UUID `json:"domain_id"`
}

/*
Kind names the job.
*/
func (CheckEmailDomainArgs) Kind() string {
	return "check_email_domain"
}

/*
Options keep one waiting check per domain and try it a few times.
*/
func (CheckEmailDomainArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: domainCheckAttempts,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRunning, queue.StateScheduled, queue.StateRetryable,
		}},
	}
}

/*
EmailDomainChecker checks one domain.
*/
type EmailDomainChecker interface {
	/*
		Handle checks the domain of the workspace.
	*/
	Handle(ctx context.Context, workspaceID, domainID ids.UUID) error
}

/*
CheckEmailDomain runs the check of a domain.
*/
type CheckEmailDomain struct {
	Checker EmailDomainChecker
}

/*
Handle checks the domain the job names.
*/
func (w *CheckEmailDomain) Handle(ctx context.Context, job *queue.Job[CheckEmailDomainArgs]) error {
	return w.Checker.Handle(ctx, job.Args.WorkspaceID, job.Args.DomainID)
}

/*
ScheduleEmailDomainChecksArgs names the job; it takes no arguments.
*/
type ScheduleEmailDomainChecksArgs struct{}

/*
Kind names the job.
*/
func (ScheduleEmailDomainChecksArgs) Kind() string {
	return "schedule_email_domain_checks"
}

/*
Options run the scheduler once; the next minute's run takes over.
*/
func (ScheduleEmailDomainChecksArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1}
}

/*
ScheduleEmailDomainChecks queues a check for every domain that is due,
across every workspace.
*/
type ScheduleEmailDomainChecks struct {
	DB    *database.DB
	Queue *queue.Queue
	Clock clock.Clock
}

/*
Handle queues the due checks.
*/
func (w *ScheduleEmailDomainChecks) Handle(ctx context.Context, _ *queue.Job[ScheduleEmailDomainChecksArgs]) error {
	due, err := models.New(w.DB).ListEmailDomainsDue(ctx, models.ListEmailDomainsDueParams{
		DueAt: w.Clock.Now(), RowLimit: domainCheckBatch,
	})
	if err != nil {
		return fmt.Errorf("list email domains due: %w", err)
	}

	for _, domain := range due {
		err := w.Queue.Push(ctx, CheckEmailDomainArgs{WorkspaceID: domain.WorkspaceID, DomainID: domain.ID})
		if err != nil {
			return fmt.Errorf("queue email domain check: %w", err)
		}
	}

	return nil
}
