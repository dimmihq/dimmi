/*
|--------------------------------------------------------------------------
| Changing Destinations
|--------------------------------------------------------------------------
|
| A channel's hearing can change at any time, and turning a stopped one
| back on clears its error so it posts again. A test posts a card right
| away and says what Slack answered, so a broken channel is found from
| the settings and not from silence. The channels on offer are the ones
| the app's bot is in.
|
*/

package notifications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Update replaces what a destination hears about. A member's direct
message only turns on or off.
*/
func (a *ManageDestinations) Update(ctx context.Context, m authz.Membership, id ids.UUID, h Hearing,
) (models.NotificationDestination, error) {
	if err := requireWrite(m); err != nil {
		return models.NotificationDestination{}, err
	}

	var row models.NotificationDestination

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := changeable(ctx, q, m, id)
		if err != nil {
			return err
		}

		p, err := a.changes(m, current, h)
		if err != nil {
			return err
		}

		row, err = q.UpdateNotificationDestination(ctx, p)
		if err != nil {
			return fmt.Errorf("update notification destination: %w", err)
		}

		return nil
	})

	return row, err
}

/*
changes are the destination's new fields.
*/
func (a *ManageDestinations) changes(m authz.Membership, row models.NotificationDestination, h Hearing,
) (models.UpdateNotificationDestinationParams, error) {
	status := StatusActive
	if !h.Enabled {
		status = "disabled"
	}

	p := models.UpdateNotificationDestinationParams{
		Name: row.Name, Triggers: row.Triggers, InboxIds: row.InboxIds, IncludeAiHandled: row.IncludeAiHandled,
		ShowPreviews: row.ShowPreviews, Digests: row.Digests, DigestTime: row.DigestTime, Status: status,
		Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	}

	if row.Scope == ScopeMember {
		return p, nil
	}

	if err := checkHearing(h); err != nil {
		return p, err
	}

	p.Name, p.Triggers, p.InboxIds = h.Name, orEmpty(h.Triggers), h.InboxIDs
	p.IncludeAiHandled, p.ShowPreviews = h.IncludeAIHandled, h.ShowPreviews
	p.Digests, p.DigestTime = orEmpty(h.Digests), h.DigestTime
	p.NextDigestAt = nextDigest(a.Clock.Now(), m.Workspace.Timezone, h)

	if p.Name == "" {
		p.Name = row.Name
	}

	return p, nil
}

/*
Delete removes a destination.
*/
func (a *ManageDestinations) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if err := requireWrite(m); err != nil {
		return err
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := changeable(ctx, q, m, id); err != nil {
			return err
		}

		_, err := q.DeleteNotificationDestination(ctx, models.DeleteNotificationDestinationParams{
			WorkspaceID: m.Workspace.ID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("delete notification destination: %w", err)
		}

		return nil
	})
}

/*
TestResult is what Slack answered a test post: sent, or failed with
Slack's code and the reason.
*/
type TestResult struct {
	Status  string
	Code    string
	Message string
}

/*
Test posts a test card to the destination now.
*/
func (a *ManageDestinations) Test(ctx context.Context, m authz.Membership, id ids.UUID) (TestResult, error) {
	if err := requireWrite(m); err != nil {
		return TestResult{}, err
	}

	var row models.NotificationDestination

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = changeable(ctx, models.New(tx), m, id)

		return err
	})
	if err != nil {
		return TestResult{}, err
	}

	card := slackdriver.Card{Text: "dimmi notifications work here", Lines: []string{
		"*dimmi notifications work here*",
		"Alerts for " + slackdriver.Escape(m.Workspace.Name) + " will show up like this.",
	}}

	_, err = a.Channel.Post(ctx, *RouteOf(row), card)
	if err == nil {
		return TestResult{Status: "sent"}, nil
	}

	return TestResult{Status: "failed", Code: codeOf(err), Message: messageOf(err)}, nil
}

/*
Channels lists the Slack channels the installation's bot is in.
*/
func (a *ManageDestinations) Channels(ctx context.Context, m authz.Membership, installation ids.UUID,
) ([]slackdriver.Channel, error) {
	if !m.Can(ManagePermission) {
		return nil, exceptions.ErrForbidden
	}

	api, err := a.api(ctx, m.Workspace.ID, installation)
	if err != nil {
		return nil, err
	}

	identity, err := api.AuthTest(ctx)
	if err != nil {
		return nil, fmt.Errorf("check slack app: %w", err)
	}

	channels, err := api.MemberChannels(ctx, identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("list slack channels: %w", err)
	}

	return channels, nil
}

/*
visible returns a destination the member may see: the workspace's, or
their own.
*/
func visible(ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID,
) (models.NotificationDestination, error) {
	row, err := q.FindNotificationDestination(ctx, models.FindNotificationDestinationParams{
		WorkspaceID: m.Workspace.ID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find notification destination: %w", err)
	}

	if row.Scope == ScopeMember && (row.WorkspaceMemberID == nil || *row.WorkspaceMemberID != m.Member.ID) {
		return row, exceptions.ErrNotFound
	}

	return row, nil
}

/*
changeable returns a destination the member may change: their own, or
the workspace's when they manage them.
*/
func changeable(ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID,
) (models.NotificationDestination, error) {
	row, err := visible(ctx, q, m, id)
	if err != nil {
		return row, err
	}

	if row.Scope == ScopeWorkspace && !m.Can(ManagePermission) {
		return row, exceptions.ErrForbidden
	}

	return row, nil
}

/*
codeOf is the product's code of Slack's refusal.
*/
func codeOf(err error) string {
	if code := slackdriver.Code(err); code != "" {
		return code
	}

	if gone(err) {
		return CodeIntegrationGone
	}

	return CodeSlackRefused
}

/*
gone reports whether the installation no longer works.
*/
func gone(err error) bool {
	return errors.Is(err, integrations.ErrIntegrationDisconnected) || errors.Is(err, integrations.ErrIntegrationBroken)
}

/*
messageOf is the sentence explaining Slack's refusal.
*/
func messageOf(err error) string {
	if sendErr, ok := errors.AsType[*contracts.SendError](err); ok {
		return sendErr.Message
	}

	if gone(err) {
		return "The Slack installation must be reconnected."
	}

	return "Slack did not accept the post. Try again in a moment."
}

/*
Removal is an installation that is gone: its workspace, the
installation and when.
*/
type Removal struct {
	WorkspaceID    ids.UUID
	InstallationID ids.UUID
	At             time.Time
}

/*
Uninstalled stops every destination of an installation that is gone,
in the transaction that removed it.
*/
func Uninstalled(ctx context.Context, q *models.Queries, r Removal) error {
	_, err := q.FailInstallationNotificationDestinations(ctx, models.FailInstallationNotificationDestinationsParams{
		Code: CodeIntegrationGone, Message: "Slack was disconnected. Connect it again to post here.", Now: r.At,
		WorkspaceID: r.WorkspaceID, IntegrationInstallationID: r.InstallationID,
	})
	if err != nil {
		return fmt.Errorf("stop notification destinations: %w", err)
	}

	return nil
}
