/*
|--------------------------------------------------------------------------
| List Security Events
|--------------------------------------------------------------------------
|
| A person can always see what happened to their own account: where it
| signed in, who failed to, and which passwords, factors and sessions
| changed. Only the account itself reads its log, newest first, a page
| at a time.
|
*/

package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
SecurityEventPosition is where a page of the security log starts: just
past the entry written at At with the id ID.
*/
type SecurityEventPosition struct {
	At time.Time `json:"t"`
	ID ids.UUID  `json:"i"`
}

/*
ListSecurityEventsInput names the user, the page size, and where the
page starts, nil for the newest entries.
*/
type ListSecurityEventsInput struct {
	UserID ids.UUID
	From   *SecurityEventPosition
	Limit  int
}

/*
SecurityEventPage is one page of the security log, and whether more
follow.
*/
type SecurityEventPage struct {
	Events []models.SecurityEvent
	More   bool
}

/*
ListSecurityEvents returns a page of a user's own security log.

Errors: a wrapped database error.
*/
type ListSecurityEvents struct {
	DB *database.DB
}

/*
Handle returns the page.
*/
func (a *ListSecurityEvents) Handle(ctx context.Context, in ListSecurityEventsInput) (SecurityEventPage, error) {
	params := models.ListUserSecurityEventsParams{UserID: in.UserID, RowLimit: int64(in.Limit + 1)}
	if in.From != nil {
		params.CursorAt, params.CursorID = &in.From.At, &in.From.ID
	}

	rows, err := models.New(a.DB).ListUserSecurityEvents(ctx, params)
	if err != nil {
		return SecurityEventPage{}, fmt.Errorf("list security events: %w", err)
	}

	if len(rows) > in.Limit {
		return SecurityEventPage{Events: rows[:in.Limit], More: true}, nil
	}

	return SecurityEventPage{Events: rows}, nil
}
