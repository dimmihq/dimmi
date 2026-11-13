/*
|--------------------------------------------------------------------------
| Live Workflows
|--------------------------------------------------------------------------
|
| Every event asks which live workflows its trigger has, so the answer
| is kept in memory, compiled once: audiences ready to evaluate, step
| indexes ready to walk. Publishing, pausing, archiving or reordering
| drops the workspace's entries on this server; entries also expire
| after a few seconds, so every server sees a change soon after.
|
*/

package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/memo"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
liveTTL is how long a server keeps a workspace's live workflows.
*/
const liveTTL = 5 * time.Second

/*
liveKey names one trigger's live workflows in one workspace.
*/
type liveKey struct {
	workspace ids.UUID
	trigger   string
}

/*
candidate is a live workflow ready to match: its version compiled.
*/
type candidate struct {
	workflowID ids.UUID
	name       string
	versionID  *ids.UUID
	number     int32
	kind       string
	spec       workflowspec.Spec
	audience   *audience.Program
	index      *workflowspec.Index
}

/*
liveCache keeps compiled live workflows per workspace and trigger.
*/
type liveCache struct {
	entries *memo.Cache[liveKey, []candidate]
}

/*
newLiveCache returns an empty cache.
*/
func newLiveCache() *liveCache {
	return &liveCache{entries: memo.New[liveKey, []candidate](memo.Options{Size: 10_000, TTL: liveTTL})}
}

/*
drop forgets a workspace's live workflows of every trigger.
*/
func (c *liveCache) drop(workspace ids.UUID) {
	for _, t := range workflowspec.Triggers() {
		c.entries.Delete(liveKey{workspace: workspace, trigger: t.Key})
	}
}

/*
candidates returns the trigger's live workflows in list order, loading
and compiling them when they are not kept.
*/
func (e *Engine) candidates(ctx context.Context, q *models.Queries, workspace ids.UUID, trigger string,
) ([]candidate, error) {
	key := liveKey{workspace: workspace, trigger: trigger}
	if found, ok := e.live.entries.Get(key); ok {
		return found, nil
	}

	rows, err := q.ListLiveWorkflowVersions(ctx, models.ListLiveWorkflowVersionsParams{
		WorkspaceID: workspace, Trigger: trigger,
	})
	if err != nil {
		return nil, fmt.Errorf("list live workflows: %w", err)
	}

	out := make([]candidate, 0, len(rows))
	for _, row := range rows {
		version := row.VersionID
		c, err := compileCandidate(row.Spec)
		if err != nil {
			continue
		}

		c.workflowID, c.name, c.versionID, c.number, c.kind = row.WorkflowID, row.Name, &version, row.Number, row.Kind
		out = append(out, c)
	}

	e.live.entries.Set(key, out)

	return out, nil
}

/*
HasOpener reports whether the workspace has a live workflow that greets
a visitor as they open a chat. The messenger asks before it opens one,
so that a workspace without an opener still starts its conversations
from the visitor's first message and leaves nothing behind when nobody
answers.
*/
func (e *Engine) HasOpener(ctx context.Context, workspace ids.UUID) (bool, error) {
	var found []candidate

	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		found, err = e.candidates(ctx, models.New(tx), workspace, workflowspec.TriggerOpener)

		return err
	})

	return len(found) > 0, err
}

/*
compileCandidate reads a frozen definition and compiles its audience.
A version that no longer compiles is left out rather than matching
everyone.
*/
func compileCandidate(raw []byte) (candidate, error) {
	spec, err := workflowspec.Parse(raw)
	if err != nil {
		return candidate{}, fmt.Errorf("read workflow version: %w", err)
	}

	a, err := audience.Parse(spec.Audience)
	if err != nil {
		return candidate{}, fmt.Errorf("read workflow audience: %w", err)
	}

	surface := audience.WorkflowAudience()
	if spec.Trigger == workflowspec.TriggerOpener {
		surface = audience.WorkflowOpener()
	}

	program, err := audience.Compile(a, workflowspec.Catalog(), surface)
	if err != nil {
		return candidate{}, fmt.Errorf("compile workflow audience: %w", err)
	}

	return candidate{spec: spec, audience: program, index: workflowspec.NewIndex(spec)}, nil
}

/*
version returns the definition a run follows: its version's, compiled
from the cache of live workflows when it is the live one, else read
from the version row.
*/
func (e *Engine) version(ctx context.Context, q *models.Queries, run models.WorkflowRun) (candidate, error) {
	raw := run.Spec
	if run.VersionID != nil {
		v, err := q.FindWorkflowVersion(ctx, models.FindWorkflowVersionParams{
			WorkspaceID: run.WorkspaceID, ID: *run.VersionID,
		})
		if err != nil {
			return candidate{}, fmt.Errorf("find workflow version: %w", err)
		}

		raw = v.Spec
	}

	return compileCandidate(raw)
}
