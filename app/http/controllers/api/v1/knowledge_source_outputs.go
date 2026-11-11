/*
|--------------------------------------------------------------------------
| Knowledge Source Outputs
|--------------------------------------------------------------------------
|
| What the knowledge sources API answers with: one source, the list of
| sources, and the kinds of source that can be added.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
KnowledgeSourceOutput is one source.
*/
type KnowledgeSourceOutput struct {
	Body fhttp.Resource[resources.KnowledgeSource]
}

/*
KnowledgeSourceListOutput is the workspace's sources.
*/
type KnowledgeSourceListOutput struct {
	Body fhttp.Collection[resources.KnowledgeSource]
}

/*
KnowledgeSourceTypesOutput is the kinds of source that can be added.
*/
type KnowledgeSourceTypesOutput struct {
	Body fhttp.Collection[resources.KnowledgeSourceType]
}

/*
KnowledgePreviewOutput is what a source would read before it is saved.
*/
type KnowledgePreviewOutput struct {
	Body fhttp.Resource[resources.KnowledgeSourcePreview]
}

/*
KnowledgeTreeOutput is the items a source can choose from.
*/
type KnowledgeTreeOutput struct {
	Body fhttp.Resource[KnowledgeTree]
}
