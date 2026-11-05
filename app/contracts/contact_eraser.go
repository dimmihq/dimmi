/*
|--------------------------------------------------------------------------
| Contact Eraser
|--------------------------------------------------------------------------
|
| Erasing a customer has to reach every place a part of the product kept
| something about them: widget sessions, raw email, delivery payloads,
| notifications, prompts. Each part that stores a customer's personal
| data registers an eraser, and the erase job runs every one before the
| request counts as done. An eraser is run again whenever the job is
| retried, so it must succeed when there is nothing left to remove.
|
*/

package contracts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
RetentionContact is the kind of subject an erasure asks the retention
guard about: one customer.
*/
const RetentionContact = "contact"

/*
ContactEraser removes what one part of the product keeps about a
customer.
*/
type ContactEraser interface {
	/*
		Name says what the eraser removes, such as "widget_sessions", for
		errors and the erasure's counts.
	*/
	Name() string

	/*
		Erase removes everything the part keeps about the contact of the
		workspace. It runs while the contact and its conversations still
		exist, and again on a retry, when it must find nothing to do.
	*/
	Erase(ctx context.Context, workspaceID, contactID ids.UUID) error
}
