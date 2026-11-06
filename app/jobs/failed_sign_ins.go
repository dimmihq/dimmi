/*
|--------------------------------------------------------------------------
| Failed Sign-Ins
|--------------------------------------------------------------------------
|
| A failed sign-in answers the same way whether or not the email has an
| account, so the answer cannot tell anyone which emails do. Each
| failure therefore hands this job the email it was for and nothing
| else, and what only an account has happens here, after the answer: the
| entry in its security log, at most one failure a minute from one
| address so guessing cannot flood the log, and the warning to its owner
| once strangers lock it, at most once a day. An email without an
| account is simply dropped.
|
*/

package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
suspiciousSignInWarning is the least time between two warnings to one
account that strangers keep guessing its password.
*/
const suspiciousSignInWarning = 24 * time.Hour

/*
RecordFailedSignInArgs is one failed sign-in: the email tried, what
happened, the method, the client it came from, and whether it locked
the account against strangers.
*/
type RecordFailedSignInArgs struct {
	Email     string `json:"email"`
	Event     string `json:"event"`
	Method    string `json:"method"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	Warn      bool   `json:"warn"`
}

/*
Kind names the job.
*/
func (RecordFailedSignInArgs) Kind() string {
	return "record_failed_sign_in"
}

/*
RecordFailedSignIn records a failed sign-in on the account its email
names, when there is one.
*/
type RecordFailedSignIn struct {
	DB          *database.DB
	Mail        *mail.Manager
	Limiter     *ratelimit.Limiter
	Links       appauth.Links
	SecurityLog *appauth.SecurityLog
}

/*
Handle writes the security log entry and warns the owner as due.
*/
func (j *RecordFailedSignIn) Handle(ctx context.Context, job *queue.Job[RecordFailedSignInArgs]) error {
	failed := job.Args

	user, err := models.New(j.DB).FindUserByEmail(ctx, failed.Email)
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find user of failed sign-in: %w", err)
	}

	if err := j.log(ctx, user, failed); err != nil {
		return err
	}

	if !failed.Warn {
		return nil
	}

	return j.warn(ctx, user)
}

/*
log writes the failure to the account's security log, one failure a
minute from one address.
*/
func (j *RecordFailedSignIn) log(ctx context.Context, user models.User, failed RecordFailedSignInArgs) error {
	if failed.Event == appauth.EventLoginFailed {
		due, err := appauth.Within(ctx, j.Limiter, appauth.Rate{
			Key: "security_event:login_failed:" + user.ID.String() + ":" + failed.IP, Limit: 1, Window: time.Minute,
		})
		if err != nil || !due {
			return err
		}
	}

	return j.SecurityLog.Record(ctx, appauth.Entry{
		UserID: user.ID, Type: failed.Event, Client: appauth.Client{IP: failed.IP, UserAgent: failed.UserAgent},
		Metadata: map[string]any{"method": failed.Method},
	})
}

/*
warn mails the owner that strangers keep guessing the password, at most
once a day.
*/
func (j *RecordFailedSignIn) warn(ctx context.Context, user models.User) error {
	due, err := appauth.Within(ctx, j.Limiter, appauth.Rate{
		Key: "mail:suspicious_sign_in:" + user.ID.String(), Limit: 1, Window: suspiciousSignInWarning,
	})
	if err != nil || !due {
		return err
	}

	err = appmail.ToUser(j.Mail, user).Queue(ctx, appmail.SuspiciousSignInAttemptsMail{
		Name: user.Name, SecurityURL: j.Links.Security(),
	})
	if err != nil {
		return fmt.Errorf("queue suspicious sign-in warning: %w", err)
	}

	return nil
}
