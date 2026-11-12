/*
|--------------------------------------------------------------------------
| Checking Help Center Domains
|--------------------------------------------------------------------------
|
| A help center's custom domain is checked from the queue: a scheduler
| runs every minute and queues the domains whose check is due, each with
| at most one check waiting.
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
HelpCenterDomainChecker checks one help center domain.
*/
type HelpCenterDomainChecker interface {
	/*
		Handle checks the domain of the workspace.
	*/
	Handle(ctx context.Context, workspaceID, domainID ids.UUID) error
}

/*
CheckHelpCenterDomainArgs names the domain to check.
*/
type CheckHelpCenterDomainArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	DomainID    ids.UUID `json:"domain_id"`
}

/*
Kind names the job.
*/
func (CheckHelpCenterDomainArgs) Kind() string {
	return "check_help_center_domain"
}

/*
Options keep one waiting check per domain and try it a few times.
*/
func (CheckHelpCenterDomainArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: domainCheckAttempts,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRunning, queue.StateScheduled, queue.StateRetryable,
		}},
	}
}

/*
CheckHelpCenterDomain runs the check of a domain.
*/
type CheckHelpCenterDomain struct {
	Domains HelpCenterDomainChecker
}

/*
Handle checks the domain the job names.
*/
func (w *CheckHelpCenterDomain) Handle(ctx context.Context, job *queue.Job[CheckHelpCenterDomainArgs]) error {
	return w.Domains.Handle(ctx, job.Args.WorkspaceID, job.Args.DomainID)
}

/*
ScheduleHelpCenterDomainChecksArgs names the job; it takes no arguments.
*/
type ScheduleHelpCenterDomainChecksArgs struct{}

/*
Kind names the job.
*/
func (ScheduleHelpCenterDomainChecksArgs) Kind() string {
	return "schedule_help_center_domain_checks"
}

/*
Options run the scheduler once; the next minute's run takes over.
*/
func (ScheduleHelpCenterDomainChecksArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1}
}

/*
ScheduleHelpCenterDomainChecks queues a check for every domain that is
due, across every workspace.
*/
type ScheduleHelpCenterDomainChecks struct {
	DB    *database.DB
	Queue *queue.Queue
	Clock clock.Clock
}

/*
Handle queues the due checks.
*/
func (w *ScheduleHelpCenterDomainChecks) Handle(ctx context.Context,
	_ *queue.Job[ScheduleHelpCenterDomainChecksArgs],
) error {
	due, err := models.New(w.DB).ListHelpCenterDomainsDue(ctx, models.ListHelpCenterDomainsDueParams{
		DueAt: w.Clock.Now(), RowLimit: domainCheckBatch,
	})
	if err != nil {
		return fmt.Errorf("list help center domains due: %w", err)
	}
	for _, d := range due {
		if err := w.Queue.Push(ctx, CheckHelpCenterDomainArgs{WorkspaceID: d.WorkspaceID, DomainID: d.ID}); err != nil {
			return fmt.Errorf("queue help center domain check: %w", err)
		}
	}
	return nil
}
