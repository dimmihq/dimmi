/*
|--------------------------------------------------------------------------
| Finishing Runs
|--------------------------------------------------------------------------
|
| A run ends once its listing was read and every fetch settled. Only a
| complete run removes documents: those it never saw are gone from the
| source, unless a member set them aside. A run where some documents
| failed is partial; a site that mostly renders in the browser says so.
| The source records how the run went and when it runs next, the oldest
| runs beyond fifty are dropped, and the rest of the product hears. When
| a source that worked stops working, its admins get one email; they
| hear again only after it worked in between. An import source goes on
| to write its articles.
|
*/

package knowledgesources

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
outcome is how a run ends: its status and, when it failed, the error
code and message.
*/
type outcome struct {
	status  string
	code    string
	message string
}

/*
closed is a run that just ended and how.
*/
type closed struct {
	run     models.KnowledgeSyncRun
	outcome outcome
}

/*
Finish closes a run whose work is done.
*/
func (e *Engine) Finish(ctx context.Context, job *queue.Job[jobs.FinishKnowledgeSyncArgs]) error {
	a := job.Args

	return e.inWorkspace(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		run, err := findRun(ctx, q, a.WorkspaceID, a.RunID)
		if errors.Is(err, ErrRunNotFound) {
			return nil
		}

		if err != nil || run.Status != "running" {
			return err
		}

		_, err = e.end(ctx, tx, run, judge(run))

		return err
	})
}

/*
judge reads how a complete run went from its counters.
*/
func judge(run models.KnowledgeSyncRun) outcome {
	fetched := stat(run, "created") + stat(run, "updated") + stat(run, "unchanged") + stat(run, "skipped_js")
	if js := stat(run, "skipped_js"); js > 0 && js*2 > fetched {
		return outcome{
			status: "partial", code: "site_requires_javascript",
			message: "This site renders its pages with JavaScript. Use a sitemap of server-rendered pages, " +
				"upload Markdown, or connect a Git repository.",
		}
	}

	if failed := stat(run, "failed"); failed > 0 {
		return outcome{
			status: "partial", code: "documents_failed",
			message: fmt.Sprintf("%d documents could not be read. They are retried on the next sync.", failed),
		}
	}

	return outcome{status: "succeeded"}
}

/*
end closes the run with the outcome, records it on the source and
tells whoever listens.
*/
func (e *Engine) end(ctx context.Context, tx *database.Tx, run models.KnowledgeSyncRun, o outcome,
) (models.KnowledgeSyncRun, error) {
	q, now := models.New(tx), e.Clock.Now()

	source, err := findSource(ctx, q, run.WorkspaceID, run.SourceID)
	if err != nil {
		return run, err
	}

	if o.status == "succeeded" || o.status == "partial" {
		if err := e.removeUnseen(ctx, q, run); err != nil {
			return run, err
		}
	}

	ended, err := q.EndKnowledgeSyncRun(ctx, models.EndKnowledgeSyncRunParams{
		Status: o.status, ErrorCode: nonEmpty(o.code), Error: nonEmpty(o.message), Now: &now,
		WorkspaceID: run.WorkspaceID, ID: run.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return run, nil
	}

	if err != nil {
		return run, fmt.Errorf("end sync: %w", err)
	}

	if err := e.recordOnSource(ctx, tx, source, o); err != nil {
		return ended, err
	}

	return ended, e.announce(ctx, tx, source, closed{run: ended, outcome: o})
}

/*
removeUnseen removes the documents a complete run never saw and drops
their chunks.
*/
func (e *Engine) removeUnseen(ctx context.Context, q *models.Queries, run models.KnowledgeSyncRun) error {
	if !run.FullRun || !run.Discovered {
		return nil
	}

	removed, err := q.RemoveUnseenKnowledgeDocuments(ctx, models.RemoveUnseenKnowledgeDocumentsParams{
		Now: e.Clock.Now(), WorkspaceID: run.WorkspaceID, SourceID: run.SourceID, RunID: &run.ID,
	})
	if err != nil {
		return fmt.Errorf("remove unseen documents: %w", err)
	}

	if err := q.DeleteRemovedKnowledgeDocumentChunks(ctx, models.DeleteRemovedKnowledgeDocumentChunksParams{
		WorkspaceID: run.WorkspaceID, SourceID: run.SourceID,
	}); err != nil {
		return fmt.Errorf("drop removed chunks: %w", err)
	}

	if len(removed) == 0 {
		return nil
	}

	_, err = e.addWork(ctx, q, run, work{deltas: map[string]int64{"removed": int64(len(removed))}})

	return err
}

/*
recordOnSource records how the run went on its source, schedules the
next one and keeps the last fifty runs.
*/
func (e *Engine) recordOnSource(ctx context.Context, tx *database.Tx, s models.KnowledgeSource, o outcome) error {
	q, now := models.New(tx), e.Clock.Now()
	status := "idle"

	if o.status == "failed" {
		status = "error"
	}

	if err := q.FinishKnowledgeSource(ctx, models.FinishKnowledgeSourceParams{
		Status: status, LastErrorCode: nonEmpty(o.code), LastError: nonEmpty(o.message), Now: now,
		Succeeded: o.status == "succeeded" || o.status == "partial", NextSyncAt: nextSync(s.Schedule, now),
		WorkspaceID: s.WorkspaceID, ID: s.ID,
	}); err != nil {
		return fmt.Errorf("record sync on source: %w", err)
	}

	if err := q.PruneKnowledgeSyncRuns(ctx, models.PruneKnowledgeSyncRunsParams{
		WorkspaceID: s.WorkspaceID, SourceID: s.ID, Keep: keptRuns,
	}); err != nil {
		return fmt.Errorf("prune sync runs: %w", err)
	}

	if s.Mode == string(sources.ModeImport) && o.status != "failed" && o.status != "canceled" {
		args := jobs.ImportKnowledgeSourceArgs{WorkspaceID: s.WorkspaceID, SourceID: s.ID}

		return wrap("queue import", e.Queue.PushTx(ctx, tx, args))
	}

	return nil
}

/*
announce tells the rest of the product how the run went, and the
admins when a working source just failed.
*/
func (e *Engine) announce(ctx context.Context, tx *database.Tx, s models.KnowledgeSource, c closed) error {
	run, o := c.run, c.outcome

	base := appevents.Base{ID: ids.NewV7At(e.Clock.Now()), WorkspaceID: s.WorkspaceID, OccurredAt: e.Clock.Now()}

	if o.status != "failed" {
		return e.dispatch(ctx, tx, appevents.KnowledgeSourceSyncCompleted{
			Base: base, SourceID: s.ID, RunID: run.ID, Status: run.Status, Stats: run.Stats,
		})
	}

	if err := e.dispatch(ctx, tx, appevents.KnowledgeSourceSyncFailed{
		Base: base, SourceID: s.ID, RunID: run.ID, Code: o.code,
	}); err != nil {
		return err
	}

	if s.LastErrorCode != nil && *s.LastErrorCode == o.code {
		return nil
	}

	return e.mailAdmins(ctx, tx, s, o)
}

func (e *Engine) dispatch(ctx context.Context, tx *database.Tx, ev interface{ EventName() string }) error {
	if e.Events == nil {
		return nil
	}

	return wrap("dispatch sync event", e.Events.Dispatch(ctx, tx, ev))
}

/*
mailAdmins tells the workspace's owners and admins that a source
stopped syncing.
*/
func (e *Engine) mailAdmins(ctx context.Context, tx *database.Tx, s models.KnowledgeSource, o outcome) error {
	if e.Mail == nil {
		return nil
	}

	q := models.New(tx)

	workspace, err := q.FindWorkspace(ctx, s.WorkspaceID)
	if err != nil {
		return fmt.Errorf("find workspace to tell: %w", err)
	}

	people, err := q.ListKnowledgeAlertRecipients(ctx, s.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list knowledge alert recipients: %w", err)
	}

	link := e.Links.App() + "/settings/knowledge/" + PublicID(s.ID)

	for _, p := range people {
		m := appmail.KnowledgeSourceFailingMail{
			Name: p.User.Name, Workspace: workspace.Name, Source: s.Name, Error: cmpString(o.message, o.code),
			SettingsURL: link,
		}
		if err := appmail.ToUser(e.Mail, p.User).QueueTx(ctx, tx, m); err != nil {
			return fmt.Errorf("queue knowledge alert mail: %w", err)
		}
	}

	return nil
}

/*
PublicID is a source's public id.
*/
func PublicID(id ids.UUID) string {
	return ids.Encode("ks", id)
}

/*
errorCode names a failure for the dashboard: the service refused the
credentials, a setting is wrong, the source is gone, or it failed.
*/
func errorCode(err error) string {
	var cfgErr *sources.ConfigError

	switch {
	case errors.Is(err, sources.ErrUnauthorized):
		return "auth_failed"
	case errors.As(err, &cfgErr):
		return "settings_invalid"
	case errors.Is(err, httpclient.ErrBlockedHost):
		return "host_blocked"
	case errors.Is(err, sources.ErrNotFound):
		return "source_not_found"
	}

	return "failed"
}

/*
message is a failure in words a member can act on.
*/
func message(err error) string {
	var cfgErr *sources.ConfigError

	switch {
	case errors.Is(err, sources.ErrUnauthorized):
		return "The service refused the source's credentials. Update the key in the source's settings."
	case errors.As(err, &cfgErr):
		return cfgErr.Message
	case errors.Is(err, httpclient.ErrBlockedHost):
		return "This address is not allowed: it points at a private network."
	case errors.Is(err, sources.ErrNotFound):
		return "The source could not be found. It may have been deleted or made private."
	}

	text := err.Error()
	if len(text) > 300 {
		text = text[:300]
	}

	return text
}

/*
pathOf is the path of an address, as patterns see it.
*/
func pathOf(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return address
	}

	return cmpString(u.EscapedPath(), "/")
}
