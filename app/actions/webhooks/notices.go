/*
|--------------------------------------------------------------------------
| Endpoint Notices
|--------------------------------------------------------------------------
|
| Who hears about an endpoint: every owner when one is added or starts
| receiving internal notes, since either sends data out of the
| workspace; owners and admins when one has failed for a day and when
| it is turned off. Every mail is queued in the transaction of the
| change it reports, and names the host only.
|
*/

package webhooks

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
mailAdded tells every owner that the endpoint now receives the
workspace's data, and who set it up.
*/
func (w *Webhooks) mailAdded(
	ctx context.Context, tx *database.Tx, m authz.Membership, e models.WebhookEndpoint,
) error {
	if w.Mail == nil {
		return nil
	}

	q := models.New(tx)

	owners, err := q.ListActiveOwners(ctx, e.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list owners to tell: %w", err)
	}

	actor := w.actorName(ctx, q, m)
	for _, owner := range owners {
		err := appmail.ToUser(w.Mail, owner.User).QueueTx(ctx, tx, appmail.WebhookEndpointAddedMail{
			Name: owner.User.Name, Workspace: m.Workspace.Name, Host: Host(e.Url), Actor: actor,
			SettingsURL: w.Links.WebhookEndpoint(EndpointPublicID(e.ID)),
		})
		if err != nil {
			return fmt.Errorf("queue webhook endpoint mail: %w", err)
		}
	}

	return nil
}

/*
actorName is how a notice names who made the change.
*/
func (w *Webhooks) actorName(ctx context.Context, q *models.Queries, m authz.Membership) string {
	user, err := q.FindUser(ctx, m.Member.UserID)
	if err != nil || user.Name == "" {
		return "A teammate"
	}

	return user.Name
}

/*
mailAlert tells every active owner and admin about an endpoint, with
the mail the build function makes for each of them.
*/
func (w *Webhooks) mailAlert(
	ctx context.Context, tx *database.Tx, e models.WebhookEndpoint, build func(name, workspace string) any,
) error {
	if w.Mail == nil {
		return nil
	}

	q := models.New(tx)

	workspace, err := q.FindWorkspace(ctx, e.WorkspaceID)
	if err != nil {
		return fmt.Errorf("find workspace to tell: %w", err)
	}

	people, err := q.ListWebhookAlertRecipients(ctx, e.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list webhook alert recipients: %w", err)
	}

	for _, person := range people {
		if err := w.queueAlert(ctx, tx, person.User, build(person.User.Name, workspace.Name)); err != nil {
			return err
		}
	}

	return nil
}

/*
queueAlert queues one alert mail to one person.
*/
func (w *Webhooks) queueAlert(ctx context.Context, tx *database.Tx, user models.User, mailable any) error {
	var err error

	switch m := mailable.(type) {
	case appmail.WebhookEndpointFailingMail:
		err = appmail.ToUser(w.Mail, user).QueueTx(ctx, tx, m)
	case appmail.WebhookEndpointDisabledMail:
		err = appmail.ToUser(w.Mail, user).QueueTx(ctx, tx, m)
	}

	return wrapf("queue webhook alert mail", err)
}

/*
eachWorkspace runs fn in the transaction of every workspace that has
an endpoint, one after the other.
*/
func (w *Webhooks) eachWorkspace(
	ctx context.Context, fn func(ctx context.Context, ws ids.UUID, tx *database.Tx, q *models.Queries) error,
) error {
	workspaces, err := models.New(w.DB).ListWorkspacesWithWebhookEndpoints(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces with webhooks: %w", err)
	}

	for _, ws := range workspaces {
		scoped := tenancy.With(ctx, ws.String())

		err := w.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			return fn(ctx, ws, tx, models.New(tx))
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
notFound turns a missing row into not found, wrapping any other error.
*/
func notFound(err error, what string) error {
	return notFoundAs(err, exceptions.ErrNotFound, what)
}

/*
notFoundAs turns a missing row into the given error, wrapping any other
error.
*/
func notFoundAs(err, missing error, what string) error {
	if errors.Is(err, database.ErrNotFound) {
		return missing
	}

	return wrapf(what, err)
}
