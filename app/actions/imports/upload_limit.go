/*
|--------------------------------------------------------------------------
| Upload Limit
|--------------------------------------------------------------------------
|
| Each teammate may upload a handful of files an hour, counted in Valkey
| and shared by every server, so a script cannot fill the database with
| drafts. Past the limit the upload is refused with how long to wait.
|
*/

package imports

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/auth/authz"
)

/*
uploadsPerHour is how many files one teammate may upload in an hour.
*/
const uploadsPerHour = 10

/*
ErrUploadLimit refuses an upload past the hourly limit.
*/
var ErrUploadLimit = fault.New(fault.RateLimited, "import_upload_limited",
	"You've uploaded too many files in the last hour. Try again later.")

/*
throttle counts the upload against the teammate's hourly limit.
*/
func (a *Imports) throttle(ctx context.Context, m authz.Membership) error {
	if a.Limiter == nil {
		return nil
	}

	res, err := a.Limiter.Attempt(ctx, "imports:upload:"+m.Member.ID.String(), uploadsPerHour, time.Hour)
	if err != nil {
		return fmt.Errorf("count import uploads: %w", err)
	}

	if res.Allowed {
		return nil
	}

	minutes := max(int((res.RetryAfter+time.Minute-1)/time.Minute), 1)
	unit := "minutes"
	if minutes == 1 {
		unit = "minute"
	}

	return fault.New(fault.RateLimited, ErrUploadLimit.Code(), fmt.Sprintf(
		"You've uploaded %d files in the last hour. Try again in %d %s.", uploadsPerHour, minutes, unit,
	)).With("retry_after", max(int((res.RetryAfter+time.Second-1)/time.Second), 1))
}
