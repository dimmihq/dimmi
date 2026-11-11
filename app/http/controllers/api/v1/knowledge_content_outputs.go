/*
|--------------------------------------------------------------------------
| Knowledge Content Outputs
|--------------------------------------------------------------------------
|
| What the knowledge sources API answers when a member chooses content:
| an archive they uploaded, and the tree of items a connected service
| lets them tick.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
KnowledgeUploadOutput is an uploaded archive.
*/
type KnowledgeUploadOutput struct {
	Body fhttp.Resource[KnowledgeUpload]
}

/*
KnowledgeUpload names an uploaded archive by the key a source's
settings use.
*/
type KnowledgeUpload struct {
	Object string `json:"object"`
	Key    string `json:"key"`
	Name   string `json:"name"`
}

/*
KnowledgeTree is a source's items as a tree.
*/
type KnowledgeTree struct {
	Object string         `json:"object"`
	Nodes  []sources.Node `json:"nodes"`
}
