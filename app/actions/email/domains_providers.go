/*
|--------------------------------------------------------------------------
| Domain Providers
|--------------------------------------------------------------------------
|
| The two kinds of provider that can set a domain up over an API. SES
| gives three signing aliases and a return path, and on the cloud ties
| the domain to the workspace's own tenant, so one workspace's reputation
| never reaches another's identities. A provider with a domain API hands
| back its own records and says when it signs for the domain.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/models"
)

/*
SES statuses the checks read.
*/
const (
	sesSuccess = "SUCCESS"
	sesFailed  = "FAILED"
)

/*
SESIdentities is the part of SES a domain needs.
*/
type SESIdentities = mail.IdentityManager

/*
SESDomains sets domains up at SES. Connect returns the identities API,
built on first use since loading AWS credentials needs a context. With
TenantPerWorkspace each workspace's identities belong to its own tenant;
Account and Region make an identity's resource name for that.
*/
type SESDomains struct {
	DB                 *database.DB
	Clock              clock.Clock
	Connect            func(ctx context.Context) (SESIdentities, error)
	TenantPerWorkspace bool
	Account            string
	Region             string
}

/*
Provision creates the identity, points its return path at the subdomain
and ties it to the workspace's tenant.
*/
func (s *SESDomains) Provision(ctx context.Context, ws ids.UUID, domain, mailFrom string) (Provisioned, error) {
	identities, err := s.Connect(ctx)
	if err != nil {
		return Provisioned{}, err
	}

	identity, err := identities.Create(ctx, domain)
	if err != nil {
		return Provisioned{}, err
	}

	returnPath := mailFrom + "." + domain
	if err := identities.SetMailFrom(ctx, domain, returnPath); err != nil {
		return Provisioned{}, err
	}

	if s.TenantPerWorkspace && s.Account != "" {
		if err := s.associate(ctx, identities, ws, domain); err != nil {
			return Provisioned{}, err
		}
	}

	return Provisioned{Tokens: identity.Tokens, Zone: identity.SigningHostedZone, MailFrom: returnPath}, nil
}

/*
associate ties the identity to the workspace's tenant, naming the tenant
in the workspace's sending profile the first time.
*/
func (s *SESDomains) associate(ctx context.Context, identities SESIdentities, ws ids.UUID, domain string) error {
	var profile models.EmailSendingProfile

	err := s.DB.Transaction(inWorkspace(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		var err error

		profile, err = models.New(tx).EnsureEmailSendingProfile(ctx, models.EnsureEmailSendingProfileParams{
			WorkspaceID: ws, SesTenantName: new("ws-" + ws.String()), Now: s.Clock.Now(),
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("ensure email sending profile: %w", err)
	}

	arn := fmt.Sprintf("arn:aws:ses:%s:%s:identity/%s", s.Region, s.Account, domain)

	return identities.AssociateTenant(ctx, *profile.SesTenantName, arn)
}

/*
Inspect reads the identity's state. An identity SES no longer has was
removed by hand; it signs nothing and is set up again on the next add.
*/
func (s *SESDomains) Inspect(ctx context.Context, row models.EmailDomain) (Inspection, error) {
	identities, err := s.Connect(ctx)
	if err != nil {
		return Inspection{}, err
	}

	identity, err := identities.Get(ctx, row.Domain)
	if errors.Is(err, mail.ErrIdentityNotFound) {
		return Inspection{Status: "NOT_FOUND"}, nil
	}

	if err != nil {
		return Inspection{}, err
	}

	return Inspection{
		Signs:       identity.DKIMStatus == sesSuccess,
		Status:      identity.DKIMStatus + "/" + identity.MailFromStatus,
		Restartable: identity.DKIMStatus == sesFailed,
	}, nil
}

/*
Restart makes SES look for the signing records again.
*/
func (s *SESDomains) Restart(ctx context.Context, row models.EmailDomain) error {
	identities, err := s.Connect(ctx)
	if err != nil {
		return err
	}

	return identities.RestartDKIM(ctx, row.Domain)
}

/*
Remove leaves the identity in place: SES keeps one per domain for the
whole account, and another workspace may still be verifying the same
domain with it.
*/
func (s *SESDomains) Remove(context.Context, models.EmailDomain) error {
	return nil
}

/*
SetMailFrom points the domain's return path at the name.
*/
func (s *SESDomains) SetMailFrom(ctx context.Context, domain, mailFrom string) error {
	identities, err := s.Connect(ctx)
	if err != nil {
		return err
	}

	return identities.SetMailFrom(ctx, domain, mailFrom)
}

/*
APIDomains sets domains up through a provider's domain API.
*/
type APIDomains struct {
	Manager mail.DomainManager
}

/*
Provision creates the domain at the provider. A domain whose tracking
could not be turned off is still kept, since its id must not be lost.
*/
func (p *APIDomains) Provision(ctx context.Context, _ ids.UUID, domain, _ string) (Provisioned, error) {
	created, err := p.Manager.CreateDomain(ctx, domain)
	if err != nil && !errors.Is(err, mail.ErrDomainTrackingLeftOn) {
		return Provisioned{}, err
	}

	return Provisioned{ProviderID: created.ID, Records: created.Records}, nil
}

/*
Inspect asks the provider to look at DNS now. A domain the provider
lost signs nothing.
*/
func (p *APIDomains) Inspect(ctx context.Context, row models.EmailDomain) (Inspection, error) {
	if row.ProviderDomainID == nil {
		return Inspection{}, nil
	}

	got, err := p.Manager.VerifyDomain(ctx, *row.ProviderDomainID)
	if errors.Is(err, mail.ErrDomainNotFound) {
		return Inspection{Status: "not_found"}, nil
	}

	if err != nil {
		return Inspection{}, err
	}

	status := "pending"
	if got.Verified {
		status = "verified"
	}

	return Inspection{Signs: got.Verified, Status: status, Records: got.Records}, nil
}

/*
Restart does nothing: such a provider looks again on every Inspect.
*/
func (p *APIDomains) Restart(context.Context, models.EmailDomain) error {
	return nil
}

/*
Remove deletes the domain at the provider.
*/
func (p *APIDomains) Remove(ctx context.Context, row models.EmailDomain) error {
	if row.ProviderDomainID == nil {
		return nil
	}

	err := p.Manager.DeleteDomain(ctx, *row.ProviderDomainID)
	if err != nil && !errors.Is(err, mail.ErrDomainNotFound) {
		return fmt.Errorf("delete provider domain: %w", err)
	}

	return nil
}
