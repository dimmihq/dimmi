/*
|--------------------------------------------------------------------------
| Failed Password Resets
|--------------------------------------------------------------------------
|
| A wrong reset link answers the same way whether or not the reset
| exists, so the answer cannot tell anyone which do. Each wrong try
| hands this job the reset it named and nothing else, and the guess is
| counted here, after the answer: the fifth wrong guess deletes the
| reset, so its link can be guessed no further. A reset that does not
| exist is simply dropped.
|
*/

package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RecordFailedResetArgs names the reset a wrong link was tried for, the
nil id when the link named none.
*/
type RecordFailedResetArgs struct {
	ResetID ids.UUID `json:"reset_id"`
}

/*
Kind names the job.
*/
func (RecordFailedResetArgs) Kind() string {
	return "record_failed_reset"
}

/*
RecordFailedReset counts a wrong guess against a password reset.
*/
type RecordFailedReset struct {
	DB *database.DB
}

/*
Handle counts the guess and deletes the reset at the last one allowed.
*/
func (j *RecordFailedReset) Handle(ctx context.Context, job *queue.Job[RecordFailedResetArgs]) error {
	q := models.New(j.DB)

	attempts, err := q.CountPasswordResetAttempt(ctx, job.Args.ResetID)
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("count wrong reset token: %w", err)
	}

	if attempts < appauth.MaxSecretAttempts {
		return nil
	}

	if _, err := q.DeletePasswordResetToken(ctx, job.Args.ResetID); err != nil {
		return fmt.Errorf("burn password reset: %w", err)
	}

	return nil
}
