/*
|--------------------------------------------------------------------------
| Feedback Status Resource
|--------------------------------------------------------------------------
|
| A stage feature requests move through, as the app lists it: its key,
| the name readers see, its color, its category and whether the public
| roadmap shows it, in the workspace's chosen order.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/actions/feedback"

/*
FeedbackStatus is one status: ID is its stored key, which requests name
it by.
*/
type FeedbackStatus struct {
	Object   string `json:"object"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Color    string `json:"color"`
	Category string `json:"category"`
	Roadmap  bool   `json:"roadmap"`
}

/*
NewFeedbackStatus shapes one status.
*/
func NewFeedbackStatus(s feedback.Status) FeedbackStatus {
	return FeedbackStatus{
		Object: "feedback_status", ID: s.Key, Name: s.Name, Slug: s.Slug, Color: s.Color, Category: s.Category,
		Roadmap: s.OnRoadmap(),
	}
}

/*
NewFeedbackStatuses returns the statuses in the order given.
*/
func NewFeedbackStatuses(list []feedback.Status) []FeedbackStatus {
	out := make([]FeedbackStatus, len(list))
	for i, s := range list {
		out[i] = NewFeedbackStatus(s)
	}
	return out
}
