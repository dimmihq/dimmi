/*
|--------------------------------------------------------------------------
| Test Conversations Resource
|--------------------------------------------------------------------------
|
| The answer to starting a new workspace's practice conversations: how
| many arrived in the inbox.
|
*/

package resources

/*
TestConversations counts the practice conversations that started.
*/
type TestConversations struct {
	Object string `json:"object"`
	Count  int    `json:"count"`
}

/*
NewTestConversations shapes the count.
*/
func NewTestConversations(count int) TestConversations {
	return TestConversations{Object: "test_conversations", Count: count}
}
