/*
|--------------------------------------------------------------------------
| User Registered
|--------------------------------------------------------------------------
|
| Someone created an account by signing up. Onboarding and metrics
| listen; nothing outside the product sees it.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
UserRegistered is dispatched in the transaction that created the
account.
*/
type UserRegistered struct {
	UserID ids.UUID
}
