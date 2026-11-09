/*
|--------------------------------------------------------------------------
| Changelog Subscriber Requests
|--------------------------------------------------------------------------
|
| The inputs of a changelog's subscriber list and of naming one label
| or subscriber.
|
*/

package requests

/*
IndexChangelogSubscribersRequest is the input of GET
/help-centers/{help_center}/changelog-subscribers, narrowed by state and
address.
*/
type IndexChangelogSubscribersRequest struct {
	HelpCenter string `path:"help_center"`
	Status     string `query:"status" enum:"pending,active,unsubscribed,"`
	Query      string `query:"query"  maxLength:"200"`
}

/*
ChangelogSubscriberRequest names a subscriber in the path.
*/
type ChangelogSubscriberRequest struct {
	Subscriber string `path:"subscriber"`
}

/*
ChangelogLabelRequest names a label in the path.
*/
type ChangelogLabelRequest struct {
	Label string `path:"label"`
}
