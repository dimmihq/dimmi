/*
|--------------------------------------------------------------------------
| Report Digest Jobs
|--------------------------------------------------------------------------
|
| Every five minutes the report digests that are due go out.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
SendReportDigestsArgs names the digest send; it takes no arguments.
*/
type SendReportDigestsArgs struct{}

/*
Kind names the job.
*/
func (SendReportDigestsArgs) Kind() string { return "send_report_digests" }

/*
SendReportDigests sends the report digests that are due.
*/
type SendReportDigests struct {
	Work ReportWork
}

/*
Handle sends the digests.
*/
func (w *SendReportDigests) Handle(ctx context.Context, _ *queue.Job[SendReportDigestsArgs]) error {
	return w.Work.SendDigests(ctx)
}
