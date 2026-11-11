/*
|--------------------------------------------------------------------------
| Knowledge Sync Outputs
|--------------------------------------------------------------------------
|
| What the knowledge sync API answers with: one sync, a source's syncs,
| and its documents.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
KnowledgeSyncRunOutput is one sync of a source.
*/
type KnowledgeSyncRunOutput struct {
	Body fhttp.Resource[resources.KnowledgeSyncRun]
}

/*
KnowledgeSyncRunListOutput is a source's latest syncs.
*/
type KnowledgeSyncRunListOutput struct {
	Body fhttp.Collection[resources.KnowledgeSyncRun]
}

/*
KnowledgeDocumentListOutput is a page of a source's documents.
*/
type KnowledgeDocumentListOutput struct {
	Body fhttp.Collection[resources.KnowledgeDocument]
}
