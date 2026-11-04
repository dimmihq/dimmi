/*
|--------------------------------------------------------------------------
| Show Widget Session
|--------------------------------------------------------------------------
|
| What a messenger learns when it starts or resumes a session: who the
| visitor is, as little as it needs, how many of the team's replies wait
| unread and which conversations are still open, all counted on this widget
| only, so a person using two widgets sees each one's own threads.
|
| Resuming without an identity token works only for anonymous sessions; a
| signed-in session must prove its identity again each time it resumes.
|
*/

package widget

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SessionView is a session as its messenger sees it.
*/
type SessionView struct {
	Session             models.WidgetSession
	Contact             models.Contact
	UnreadCount         int64
	OpenConversationIDs []ids.UUID
}

/*
ShowWidgetSession describes visitors' sessions.
*/
type ShowWidgetSession struct {
	DB *database.DB
}

/*
Resume describes the session a visitor resumes without an identity token,
which only an anonymous session may do.
*/
func (a *ShowWidgetSession) Resume(ctx context.Context, auth Authenticated) (SessionView, error) {
	if auth.Session.Kind != string(enums.WidgetSessionAnonymous) {
		return SessionView{}, exceptions.ErrIdentityProofRequired
	}

	return a.Describe(ctx, auth)
}

/*
Describe returns the session with its contact and what waits for them.
*/
func (a *ShowWidgetSession) Describe(ctx context.Context, auth Authenticated) (SessionView, error) {
	out := SessionView{Session: auth.Session}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error

		out.Contact, err = q.FindContact(ctx, models.FindContactParams{
			WorkspaceID: auth.Session.WorkspaceID, ID: auth.Session.ContactID,
		})
		if err != nil {
			return fmt.Errorf("find session contact: %w", err)
		}

		summary, err := q.SummarizeWidgetConversations(ctx, models.SummarizeWidgetConversationsParams{
			WorkspaceID: auth.Session.WorkspaceID, ContactID: auth.Session.ContactID,
			WidgetID: models.PublicID(models.PrefixWidget, auth.Resolved.Widget.ID), InboxID: auth.Resolved.InboxID,
		})
		if err != nil {
			return fmt.Errorf("summarize widget conversations: %w", err)
		}

		out.UnreadCount, out.OpenConversationIDs = summary.UnreadCount, summary.OpenIds

		return nil
	})

	return out, err
}
