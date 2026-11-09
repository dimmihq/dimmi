/*
|--------------------------------------------------------------------------
| Changelog Jobs
|--------------------------------------------------------------------------
|
| A scheduled entry publishes itself when its time comes, and an entry
| the team chose to send is mailed to its subscribers, both in the
| background and inside the entry's workspace. Either job can run twice
| without doing its work twice.
|
*/

package changelog

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"
)

/*
PublishWorker publishes an entry whose time has come.
*/
type PublishWorker struct {
	Entries *Entries
}

/*
Handle publishes the entry when it is still due.
*/
func (w *PublishWorker) Handle(ctx context.Context, job *queue.Job[PublishEntryArgs]) error {
	return w.Entries.PublishDue(tenancy.With(ctx, job.Args.WorkspaceID.String()), job.Args)
}

/*
MailWorker mails an entry to its changelog's subscribers.
*/
type MailWorker struct {
	Subscribers *Subscribers
}

/*
Handle sends the entry's one mailing.
*/
func (w *MailWorker) Handle(ctx context.Context, job *queue.Job[MailEntryArgs]) error {
	return w.Subscribers.MailEntry(tenancy.With(ctx, job.Args.WorkspaceID.String()), job.Args)
}
