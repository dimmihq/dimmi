/*
|--------------------------------------------------------------------------
| Deliveries From Tools
|--------------------------------------------------------------------------
|
| A tool's delivery is checked before anything reads it: Linear with the
| app's webhook secret, Jira with the app's client secret, GitHub with
| the install's webhook secret, and the key-connected tools with the
| secret of the endpoint the workspace pasted, found through the
| installation the address names. Then each event goes where it
| belongs: an issue's news to every link of it, in whichever workspace;
| a comment to the tickets as a note; an incident's public update to its
| dimmi incident; a deployment to the drafts waiting for it. A new
| incident in an incident tool reaches only the tickets a teammate
| already linked to it: nothing is opened in dimmi on its own.
|
*/

package issues

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of a delivery.
*/
var (
	ErrUnsigned = fault.New(fault.Unauthenticated, "webhook_signature_invalid",
		"The delivery's signature does not match.")
	ErrUnknownTool = fault.New(fault.NotFound, "webhook_unknown", "Nothing receives deliveries here.")
)

/*
Delivery is what a tool sent: its provider, the installation its
address names, nil for an app-wide address, and the signed request.
*/
type Delivery struct {
	Provider       string
	InstallationID *ids.UUID
	Signed         trackers.Signed
}

/*
Receive checks a delivery and applies each event in it.
*/
func (s *Issues) Receive(ctx context.Context, d Delivery) error {
	tracker, ok := s.Trackers.Get(d.Provider)
	if !ok {
		return ErrUnknownTool
	}

	row, err := s.addressed(ctx, d)
	if err != nil {
		return err
	}

	secret, err := s.secretFor(ctx, d.Provider, row)
	if err != nil {
		return err
	}

	if err := tracker.Verify(d.Signed, secret, s.Clock.Now()); err != nil {
		return ErrUnsigned
	}

	events, err := tracker.Parse(d.Signed.Body)
	if err != nil {
		return fmt.Errorf("read delivery: %w", err)
	}

	for _, e := range events {
		if err := s.route(ctx, d.Provider, row, e); err != nil {
			return err
		}
	}

	return nil
}

/*
addressed reads the installation a delivery's address names, nil for
an app-wide address. An address naming nothing of the tool leads
nowhere.
*/
func (s *Issues) addressed(ctx context.Context, d Delivery) (*models.IntegrationInstallation, error) {
	if d.InstallationID == nil {
		p, _ := s.Registry.Provider(d.Provider)
		if p.Key {
			return nil, ErrUnknownTool
		}

		return nil, nil
	}

	var owner models.InstallationOwnerRow

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		owner, err = models.New(tx).InstallationOwner(ctx, *d.InstallationID)

		return err
	})
	if err != nil || owner.Provider != d.Provider || owner.Status == "disconnected" {
		return nil, ErrUnknownTool
	}

	var row models.IntegrationInstallation

	err = s.inWorkspace(ctx, owner.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		row, err = q.FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{
			WorkspaceID: owner.WorkspaceID, ID: *d.InstallationID,
		})

		return err
	})
	if err != nil {
		return nil, ErrUnknownTool
	}

	return &row, nil
}

/*
secretFor returns the secret the delivery must be signed with.
*/
func (s *Issues) secretFor(ctx context.Context, provider string, row *models.IntegrationInstallation,
) (string, error) {
	switch provider {
	case "linear":
		return s.Services.Linear.WebhookSecret, nil
	case "jira":
		return s.Services.Jira.ClientSecret, nil
	case "github":
		return s.Services.Trackers.GitHubWebhookSecret, nil
	}

	secret, err := s.Tokens.SigningSecret(tenancy.With(ctx, row.WorkspaceID.String()), row.ID)
	if err != nil || secret == "" {
		return "", ErrUnsigned
	}

	return secret, nil
}

/*
route applies one event.
*/
func (s *Issues) route(ctx context.Context, provider string, row *models.IntegrationInstallation, e trackers.Event,
) error {
	if e.Kind == trackers.EventDeployment {
		if row == nil {
			return nil
		}

		return s.deployed(ctx, *row, e)
	}

	links, err := s.linksOf(ctx, provider, row, e.ExternalID)
	if err != nil {
		return err
	}

	for _, l := range links {
		if err := s.apply(ctx, l, e); err != nil {
			return err
		}
	}

	return nil
}

/*
apply applies one event to one link.
*/
func (s *Issues) apply(ctx context.Context, l models.TicketIssuesByExternalIDRow, e trackers.Event) error {
	switch e.Kind {
	case trackers.EventIssue, trackers.EventIncident:
		if e.Issue == nil {
			return nil
		}

		return s.applyIssue(ctx, l.WorkspaceID, l.ID, *e.Issue)
	case trackers.EventComment:
		return s.commentOn(ctx, l.WorkspaceID, l.ID, e)
	case trackers.EventUpdate:
		return s.publicUpdate(ctx, l.WorkspaceID, l.ID, e)
	}

	return nil
}

/*
linksOf finds every link to the tool's issue, only the addressed
workspace's when the address names an installation.
*/
func (s *Issues) linksOf(ctx context.Context, provider string, row *models.IntegrationInstallation, external string,
) ([]models.TicketIssuesByExternalIDRow, error) {
	if external == "" {
		return nil, nil
	}

	var found []models.TicketIssuesByExternalIDRow

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		found, err = models.New(tx).TicketIssuesByExternalID(ctx, models.TicketIssuesByExternalIDParams{
			Provider: provider, ExternalID: external,
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("find links: %w", err)
	}

	if row == nil {
		return found, nil
	}

	var mine []models.TicketIssuesByExternalIDRow

	for _, l := range found {
		if l.WorkspaceID == row.WorkspaceID {
			mine = append(mine, l)
		}
	}

	return mine, nil
}

/*
publicUpdate posts an update from the tool to the linked dimmi
incident's public log.
*/
func (s *Issues) publicUpdate(ctx context.Context, ws, linkID ids.UUID, e trackers.Event) error {
	if e.Body == "" {
		return nil
	}

	return s.asTool(ctx, ws, linkID, func(ctx context.Context, w *tickets.Writer, link models.TicketIssue) error {
		t, err := w.Lock(ctx, ws, link.TicketID)
		if err != nil || t.DeletedAt != nil || t.Kind != tickets.KindIncident {
			return nil //nolint:nilerr // only a live incident keeps a public log
		}

		_, err = w.PublicUpdate(ctx, t, tickets.PublicUpdateInput{Body: clip(e.Body, 2000), Source: link.Provider})

		var f *fault.Error
		if errors.As(err, &f) && f.Kind() == fault.Invalid {
			return nil
		}

		return err
	})
}
