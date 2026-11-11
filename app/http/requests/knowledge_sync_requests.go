/*
|--------------------------------------------------------------------------
| Knowledge Sync Requests
|--------------------------------------------------------------------------
|
| The inputs of a source's syncs and documents: a run or a document in
| the path, filters for the document list, the help center a synced
| source's documents become articles in, and an uploaded archive.
|
*/

package requests

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
KnowledgeSyncRunRequest names a run of a source in the path.
*/
type KnowledgeSyncRunRequest struct {
	Source string `path:"source"`
	Run    string `path:"run"`
}

/*
ListKnowledgeDocumentsRequest narrows a source's documents: a status,
words in the title or path, and the cursor of the next page.
*/
type ListKnowledgeDocumentsRequest struct {
	Source string `path:"source"`
	Status string `query:"status" enum:"active,error,excluded,pending,removed,"`
	Query  string `query:"q"      maxLength:"200"`
	After  string `query:"after"  maxLength:"2100"`
	Limit  int32  `query:"limit"  maximum:"100"`
}

/*
KnowledgeDocumentRequest names a document of a source in the path.
*/
type KnowledgeDocumentRequest struct {
	Source   string `path:"source"`
	Document string `path:"document"`
}

/*
ConvertKnowledgeSourceRequest is the input of POST
/knowledge/sources/{source}/imports: the help center the articles go
to.
*/
type ConvertKnowledgeSourceRequest struct {
	Source string `path:"source"`
	Body   struct {
		HelpCenterID string `json:"help_center_id" minLength:"1"`
	}
}

/*
StoreKnowledgeUploadRequest is the input of POST /knowledge/uploads: a
multipart form with the archive in the file field.
*/
type StoreKnowledgeUploadRequest struct {
	RawBody fhttp.MultipartForm
}
