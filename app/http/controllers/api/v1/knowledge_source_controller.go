/*
|--------------------------------------------------------------------------
| Knowledge Source Controller
|--------------------------------------------------------------------------
|
| GET /knowledge/source-types lists the drivers a source can be added
| from, with the settings each takes. Sources are listed, previewed,
| created, read, changed and removed; a source's syncs are started,
| listed and canceled; its documents are listed, synced again, set
| aside and let back in; a synced source is turned into articles; and a
| Markdown archive is uploaded for a source to read.
|
*/

package v1

import (
	"context"
	"fmt"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/knowledgesources"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
KnowledgeSourceController serves knowledge sources.
*/
type KnowledgeSourceController struct {
	Engine *knowledgesources.Engine
	IDs    *ids.Registry
}

/*
Types lists the drivers a source can be added from.
*/
func (c *KnowledgeSourceController) Types(ctx context.Context, _ *requests.NoInput) (*KnowledgeSourceTypesOutput,
	error,
) {
	types, err := c.Engine.Gallery(membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.KnowledgeSourceType, len(types))
	for i, t := range types {
		items[i] = resources.NewKnowledgeSourceType(t)
	}

	page := pagination.Page[resources.KnowledgeSourceType]{Items: items, Limit: len(items)}

	return &KnowledgeSourceTypesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Index lists the workspace's sources.
*/
func (c *KnowledgeSourceController) Index(ctx context.Context, _ *requests.NoInput) (*KnowledgeSourceListOutput,
	error,
) {
	views, err := c.Engine.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.KnowledgeSource, len(views))
	for i, v := range views {
		items[i] = resources.NewKnowledgeSource(v)
	}

	page := pagination.Page[resources.KnowledgeSource]{Items: items, Limit: len(items)}

	return &KnowledgeSourceListOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store creates a source.
*/
func (c *KnowledgeSourceController) Store(ctx context.Context, in *requests.StoreKnowledgeSourceRequest,
) (*KnowledgeSourceOutput, error) {
	input, err := c.input(ctx, in.Body)
	if err != nil {
		return nil, err
	}

	return sourceOutput(c.Engine.Create(ctx, input))
}

/*
Preview checks settings and counts what they would read.
*/
func (c *KnowledgeSourceController) Preview(ctx context.Context, in *requests.PreviewKnowledgeSourceRequest,
) (*KnowledgePreviewOutput, error) {
	input, err := c.input(ctx, in.Body)
	if err != nil {
		return nil, err
	}

	p, err := c.Engine.Preview(ctx, input)
	if err != nil {
		return nil, err
	}

	return &KnowledgePreviewOutput{Body: fhttp.NewResource(resources.NewKnowledgeSourcePreview(p))}, nil
}

/*
Show reads one source.
*/
func (c *KnowledgeSourceController) Show(ctx context.Context, in *requests.KnowledgeSourceRequest,
) (*KnowledgeSourceOutput, error) {
	id, err := c.source(in.Source)
	if err != nil {
		return nil, err
	}

	return sourceOutput(c.Engine.Show(ctx, membership(ctx), id))
}

/*
Update changes a source.
*/
func (c *KnowledgeSourceController) Update(ctx context.Context, in *requests.UpdateKnowledgeSourceRequest,
) (*KnowledgeSourceOutput, error) {
	id, err := c.source(in.Source)
	if err != nil {
		return nil, err
	}

	input, err := c.input(ctx, in.Body)
	if err != nil {
		return nil, err
	}

	input.ID = id

	return sourceOutput(c.Engine.Update(ctx, input))
}

/*
Destroy removes a source with its documents.
*/
func (c *KnowledgeSourceController) Destroy(ctx context.Context, in *requests.KnowledgeSourceRequest,
) (*EmptyOutput, error) {
	id, err := c.source(in.Source)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Engine.Delete(ctx, membership(ctx), id)
}

/*
input reads a body into the engine's input.
*/
func (c *KnowledgeSourceController) input(ctx context.Context, b requests.KnowledgeSourceBody,
) (knowledgesources.SourceInput, error) {
	in := knowledgesources.SourceInput{
		Membership: membership(ctx), Type: b.Type, Name: b.Name, Mode: b.Mode, Targets: b.Targets,
		Settings: b.Settings, Include: b.Include, Exclude: b.Exclude, Schedule: b.Schedule, SyncNow: b.SyncNow,
	}

	if b.HelpCenterID != nil && *b.HelpCenterID != "" {
		id, err := decodeID(c.IDs, models.PrefixHelpCenter, *b.HelpCenterID)
		if err != nil {
			return in, err
		}

		in.HelpCenterID = &id
	}

	return in, nil
}

/*
source decodes a source's public id.
*/
func (c *KnowledgeSourceController) source(public string) (ids.UUID, error) {
	id, err := resources.KnowledgeSourceUUID(c.IDs, public)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}

/*
sourceOutput shapes a source or passes its error on.
*/
func sourceOutput(v knowledgesources.SourceView, err error) (*KnowledgeSourceOutput, error) {
	if err != nil {
		return nil, err
	}

	return &KnowledgeSourceOutput{Body: fhttp.NewResource(resources.NewKnowledgeSource(v))}, nil
}

/*
Upload keeps a Markdown archive for a source to read.
*/
func (c *KnowledgeSourceController) Upload(ctx context.Context, in *requests.StoreKnowledgeUploadRequest,
) (*KnowledgeUploadOutput, error) {
	defer func() { _ = in.RawBody.RemoveAll() }()

	files := in.RawBody.File["file"]
	if len(files) == 0 {
		return nil, knowledgesources.ErrUploadInvalid
	}

	f, err := files[0].Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded archive: %w", err)
	}

	defer func() { _ = f.Close() }()

	key, err := c.Engine.StoreUpload(ctx, membership(ctx), files[0].Filename, f)
	if err != nil {
		return nil, err
	}

	upload := KnowledgeUpload{Object: "knowledge_upload", Key: key, Name: files[0].Filename}

	return &KnowledgeUploadOutput{Body: fhttp.NewResource(upload)}, nil
}

/*
Tree lists the items a source being added can choose from.
*/
func (c *KnowledgeSourceController) Tree(ctx context.Context, in *requests.PreviewKnowledgeSourceRequest,
) (*KnowledgeTreeOutput, error) {
	input, err := c.input(ctx, in.Body)
	if err != nil {
		return nil, err
	}

	return treeOutput(c.Engine.Tree(ctx, input))
}

/*
SourceTree lists the items a saved source can choose from.
*/
func (c *KnowledgeSourceController) SourceTree(ctx context.Context, in *requests.KnowledgeSourceRequest,
) (*KnowledgeTreeOutput, error) {
	id, err := c.source(in.Source)
	if err != nil {
		return nil, err
	}

	return treeOutput(c.Engine.SourceTree(ctx, membership(ctx), id))
}

/*
treeOutput shapes a tree or passes its error on.
*/
func treeOutput(nodes []sources.Node, err error) (*KnowledgeTreeOutput, error) {
	if err != nil {
		return nil, err
	}

	tree := KnowledgeTree{Object: "knowledge_source_tree", Nodes: nodes}

	return &KnowledgeTreeOutput{Body: fhttp.NewResource(tree)}, nil
}
