/*
|--------------------------------------------------------------------------
| Workflow Publish Requests
|--------------------------------------------------------------------------
|
| Publishing names the working copy the publisher reviewed, so a change
| saved meanwhile is never published unseen. Reordering lists every
| workflow, so an order built on a stale list is refused.
|
*/

package requests

/*
PublishWorkflowRequest is the input of POST
/workflows/{workflow}/publications.
*/
type PublishWorkflowRequest struct {
	Workflow string `path:"workflow"`
	Body     *PublishWorkflowBody
}

/*
PublishWorkflowBody is the optional body of a publish.
*/
type PublishWorkflowBody struct {
	Note                *string `json:"note,omitempty"       maxLength:"280"`
	AcknowledgeWarnings bool    `json:"acknowledge_warnings,omitempty"`
	DraftHash           string  `json:"draft_hash,omitempty" maxLength:"64"`
}

/*
ReorderWorkflowsRequest is the input of PATCH /workflows: every
workflow that is not archived, in the new order.
*/
type ReorderWorkflowsRequest struct {
	Body struct {
		Order []string `json:"order" maxItems:"500"`
	}
}
