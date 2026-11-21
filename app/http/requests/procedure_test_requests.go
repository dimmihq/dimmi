/*
|--------------------------------------------------------------------------
| Procedure Test Requests
|--------------------------------------------------------------------------
|
| A saved test's scripted messages, customer, mocks and assertions, and
| the tests a run takes.
|
*/

package requests

/*
TestMessageInput is one scripted customer message of a test.
*/
type TestMessageInput struct {
	Text    string `json:"text"              maxLength:"4000"`
	Confirm bool   `json:"confirm,omitempty"`
}

/*
TestCaseBody is a test case's fields; on an update, a field left out
stays as it is.
*/
type TestCaseBody struct {
	Name                 *string                    `json:"name,omitempty"                   maxLength:"120"`
	SourceConversationID string                     `json:"source_conversation_id,omitempty"`
	Persona              *SimPersonaInput           `json:"persona,omitempty"`
	Messages             []TestMessageInput         `json:"messages,omitempty"               maxItems:"12"`
	ActionMocks          map[string]ActionMockInput `json:"action_mocks,omitempty"`
	Assertions           []map[string]any           `json:"assertions,omitempty"             maxItems:"30"`
}

/*
StoreTestCaseRequest is the input of POST
/procedures/{procedure}/test-cases.
*/
type StoreTestCaseRequest struct {
	Procedure string `path:"procedure"`
	Body      TestCaseBody
}
