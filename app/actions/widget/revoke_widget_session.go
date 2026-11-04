/*
|--------------------------------------------------------------------------
| Revoke Widget Session
|--------------------------------------------------------------------------
|
| Ending a session ends everything it opened: the token stops working at
| once, and every stream it holds hears why and closes, on every replica,
| so a shared computer forgets the visitor within seconds. Ending a session
| that already ended changes nothing, which keeps a repeated sign-out safe.
|
*/

package widget

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RevokeWidgetSessionInput is the session to end and why.
*/
type RevokeWidgetSessionInput struct {
	Session models.WidgetSession
	Reason  enums.WidgetRevokeReason
}

/*
RevokeWidgetSession ends visitors' sessions. Streams closes what they had
open.
*/
type RevokeWidgetSession struct {
	DB      *database.DB
	Clock   clock.Clock
	Streams SessionCloser
}

/*
Handle ends the session and closes its streams.
*/
func (a *RevokeWidgetSession) Handle(ctx context.Context, in RevokeWidgetSessionInput) error {
	var ended int64

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		ended, err = models.New(tx).RevokeWidgetSession(ctx, models.RevokeWidgetSessionParams{
			Now: a.Clock.Now(), Reason: string(in.Reason), WorkspaceID: in.Session.WorkspaceID, ID: in.Session.ID,
		})
		if err != nil {
			return fmt.Errorf("revoke widget session: %w", err)
		}

		return nil
	})
	if err != nil || ended == 0 {
		return err
	}

	return a.Streams.CloseSessions(ctx, []ids.UUID{in.Session.ID})
}
