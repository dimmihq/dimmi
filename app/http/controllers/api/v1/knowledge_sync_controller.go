/*
|--------------------------------------------------------------------------
| Knowledge Sync Controller
|--------------------------------------------------------------------------
|
| POST /knowledge/sources/{source}/syncs starts a sync, or returns the
| one running; GET lists the latest fifty; DELETE on one cancels it. A
| source's documents are listed by status a page at a time, and one
| document can be synced again, set aside or let back in. POST
| .../imports turns a synced source into help center articles.
|
*/

package v1

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/knowledgesources"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Sync starts a sync of the source.
*/
func (c *KnowledgeSourceController) Sync(ctx context.Context, in *requests.KnowledgeSourceRequest,
) (*KnowledgeSyncRunOutput, error) {
	id, err := c.source(in.Source)
	if err != nil {
		return nil, err
	}

	return runOutput(c.Engine.Sync(ctx, membership(ctx), id))
}

/*
Runs lists the source's latest syncs.
*/
func (c *KnowledgeSourceController) Runs(ctx context.Context, in *requests.KnowledgeSourceRequest,
) (*KnowledgeSyncRunListOutput, error) {
	id, err := c.source(in.Source)
	if err != nil {
		return nil, err
	}

	runs, err := c.Engine.Runs(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	items := make([]resources.KnowledgeSyncRun, len(runs))
	for i, r := range runs {
		items[i] = resources.NewKnowledgeSyncRun(r)
	}

	page := pagination.Page[resources.KnowledgeSyncRun]{Items: items, Limit: len(items)}

	return &KnowledgeSyncRunListOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Cancel stops a running sync.
*/
func (c *KnowledgeSourceController) Cancel(ctx context.Context, in *requests.KnowledgeSyncRunRequest,
) (*KnowledgeSyncRunOutput, error) {
	source, err := c.source(in.Source)
	if err != nil {
		return nil, err
	}

	run, err := resources.KnowledgeSyncRunID(c.IDs, in.Run)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return runOutput(c.Engine.Cancel(ctx, membership(ctx), source, run))
}

/*
Documents lists a page of the source's documents.
*/
func (c *KnowledgeSourceController) Documents(ctx context.Context, in *requests.ListKnowledgeDocumentsRequest,
) (*KnowledgeDocumentListOutput, error) {
	source, err := c.source(in.Source)
	if err != nil {
		return nil, err
	}

	filter := knowledgesources.DocumentFilter{
		SourceID: source, Query: strings.TrimSpace(in.Query), Limit: max(in.Limit, 0),
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	if in.Status != "" {
		filter.Statuses = []string{in.Status}
	}

	if path, id, ok := strings.Cut(in.After, "\x1f"); ok {
		if after, err := resources.KnowledgeDocumentUUID(c.IDs, id); err == nil {
			filter.AfterPath, filter.AfterID = &path, after
		}
	}

	rows, err := c.Engine.Documents(ctx, membership(ctx), filter)
	if err != nil {
		return nil, err
	}

	items := make([]resources.KnowledgeDocument, len(rows))
	for i, r := range rows {
		items[i] = resources.NewKnowledgeDocument(r)
	}

	page := pagination.Page[resources.KnowledgeDocument]{Items: items, Limit: int(filter.Limit)}
	if len(rows) > 0 && filter.Limit > 0 && len(rows) == int(filter.Limit) {
		last := rows[len(rows)-1]
		page.NextCursor = last.Path + "\x1f" + items[len(items)-1].ID
	}

	return &KnowledgeDocumentListOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
ResyncDocument reads one document again now.
*/
func (c *KnowledgeSourceController) ResyncDocument(ctx context.Context, in *requests.KnowledgeDocumentRequest,
) (*KnowledgeSyncRunOutput, error) {
	source, doc, err := c.document(in)
	if err != nil {
		return nil, err
	}

	return runOutput(c.Engine.Resync(ctx, membership(ctx), source, doc))
}

/*
ExcludeDocument sets a document aside for good.
*/
func (c *KnowledgeSourceController) ExcludeDocument(ctx context.Context, in *requests.KnowledgeDocumentRequest,
) (*EmptyOutput, error) {
	source, doc, err := c.document(in)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Engine.Exclude(ctx, membership(ctx), source, doc)
}

/*
IncludeDocument lets a set-aside document back in.
*/
func (c *KnowledgeSourceController) IncludeDocument(ctx context.Context, in *requests.KnowledgeDocumentRequest,
) (*KnowledgeSyncRunOutput, error) {
	source, doc, err := c.document(in)
	if err != nil {
		return nil, err
	}

	return runOutput(c.Engine.Include(ctx, membership(ctx), source, doc))
}

/*
Convert turns a synced source into help center articles.
*/
func (c *KnowledgeSourceController) Convert(ctx context.Context, in *requests.ConvertKnowledgeSourceRequest,
) (*EmptyOutput, error) {
	source, err := c.source(in.Source)
	if err != nil {
		return nil, err
	}

	center, err := decodeID(c.IDs, models.PrefixHelpCenter, in.Body.HelpCenterID)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Engine.Convert(ctx, membership(ctx), source, center)
}

/*
document decodes a source's and a document's public ids.
*/
func (c *KnowledgeSourceController) document(in *requests.KnowledgeDocumentRequest) (ids.UUID, ids.UUID, error) {
	source, err := c.source(in.Source)
	if err != nil {
		return source, source, err
	}

	doc, err := resources.KnowledgeDocumentUUID(c.IDs, in.Document)
	if err != nil {
		return source, doc, exceptions.ErrNotFound
	}

	return source, doc, nil
}

/*
runOutput shapes a run or passes its error on.
*/
func runOutput(r models.KnowledgeSyncRun, err error) (*KnowledgeSyncRunOutput, error) {
	if err != nil {
		return nil, err
	}

	return &KnowledgeSyncRunOutput{Body: fhttp.NewResource(resources.NewKnowledgeSyncRun(r))}, nil
}
