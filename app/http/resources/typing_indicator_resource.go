/*
|--------------------------------------------------------------------------
| Typing Indicator Resource
|--------------------------------------------------------------------------
|
| A typing report as the server accepted it, and when the other side
| stops showing it unless the client reports again.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/conversations"
)

/*
TypingIndicator is an accepted typing report.
*/
type TypingIndicator struct {
	Object    string    `json:"object"`
	State     string    `json:"state"`
	ExpiresAt time.Time `json:"expires_at"`
}

/*
NewTypingIndicator shapes the accepted report.
*/
func NewTypingIndicator(t conversations.TypingIndicator) TypingIndicator {
	return TypingIndicator{Object: "typing_indicator", State: t.State, ExpiresAt: t.ExpiresAt}
}
