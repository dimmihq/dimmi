/*
|--------------------------------------------------------------------------
| Feedback Status Resource
|--------------------------------------------------------------------------
|
| A stage feature requests move through, as the app lists it: its key,
| the name readers see and its color, in the workspace's chosen order.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/actions/feedback"

/*
FeedbackStatus is one status: ID is its stored key.
*/
type FeedbackStatus struct {
	Object string `json:"object"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Color  string `json:"color"`
}

/*
NewFeedbackStatuses returns the statuses in the order given.
*/
func NewFeedbackStatuses(list []feedback.Status) []FeedbackStatus {
	out := make([]FeedbackStatus, len(list))
	for i, s := range list {
		out[i] = FeedbackStatus{Object: "feedback_status", ID: s.Key, Name: s.Name, Color: s.Color}
	}
	return out
}
