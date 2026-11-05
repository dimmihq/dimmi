/*
|--------------------------------------------------------------------------
| SSO Connection Upkeep
|--------------------------------------------------------------------------
|
| Two daily jobs keep companies' connections signing people in. One reads
| each connection's metadata address again, so a certificate the
| identity provider rolls is picked up before the old one stops working.
| The other warns the owners thirty, seven and one day before a
| certificate their identity provider signs with expires, since nobody
| can sign in through it after.
|
*/

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/saml"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RefreshSSOMetadataArgs carries nothing: every run refreshes every
connection set up from a metadata address.
*/
type RefreshSSOMetadataArgs struct{}

/*
Kind names the job.
*/
func (RefreshSSOMetadataArgs) Kind() string {
	return "refresh_sso_metadata"
}

/*
Options try the job a few times; the next day's run catches up anyway.
*/
func (RefreshSSOMetadataArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: expiryAttempts}
}

/*
RefreshSSOMetadata refreshes the metadata of every live connection.
*/
type RefreshSSOMetadata struct {
	DB      *database.DB
	Refresh *sso.RefreshMetadata
}

/*
Handle refreshes each connection on its own; a failure is recorded on
the connection.
*/
func (w *RefreshSSOMetadata) Handle(ctx context.Context, _ *queue.Job[RefreshSSOMetadataArgs]) error {
	connections, err := models.New(w.DB).ListActiveSSOConnections(ctx)
	if err != nil {
		return fmt.Errorf("list connections to refresh: %w", err)
	}

	for _, row := range connections {
		if err := w.Refresh.Handle(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

/*
WarnExpiringSSOCertificatesArgs carries nothing: every run looks at
every live connection.
*/
type WarnExpiringSSOCertificatesArgs struct{}

/*
Kind names the job.
*/
func (WarnExpiringSSOCertificatesArgs) Kind() string {
	return "warn_expiring_sso_certificates"
}

/*
Options try the job a few times; a warning a day late still helps.
*/
func (WarnExpiringSSOCertificatesArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: expiryAttempts}
}

/*
WarnExpiringSSOCertificates warns owners of certificates about to
expire.
*/
type WarnExpiringSSOCertificates struct {
	DB    *database.DB
	Clock clock.Clock
	Mail  *mail.Manager
	Links appauth.Links
}

/*
Handle mails the owners of each connection whose identity provider
certificate expires in exactly one of the warning days.
*/
func (w *WarnExpiringSSOCertificates) Handle(
	ctx context.Context, _ *queue.Job[WarnExpiringSSOCertificatesArgs],
) error {
	connections, err := models.New(w.DB).ListActiveSSOConnections(ctx)
	if err != nil {
		return fmt.Errorf("list connections to check: %w", err)
	}

	for _, row := range connections {
		expiry, due := w.due(row)
		if !due {
			continue
		}

		if err := w.warn(ctx, row, expiry); err != nil {
			return err
		}
	}

	return nil
}

/*
due returns the expiry of the connection's certificate that reaches a
warning day today, if one does.
*/
func (w *WarnExpiringSSOCertificates) due(row models.SSOConnection) (time.Time, bool) {
	conn, err := sso.Decode(row)
	if err != nil || row.Protocol != sso.ProtocolSAML {
		return time.Time{}, false
	}

	certificates, err := saml.ParseCertificates(conn.Config.IDPCertificates)
	if err != nil {
		return time.Time{}, false
	}

	today := w.Clock.Now().UTC().Truncate(24 * time.Hour)

	for _, certificate := range certificates {
		left := int(certificate.NotAfter.UTC().Truncate(24*time.Hour).Sub(today) / (24 * time.Hour))
		if warningDay(left) {
			return certificate.NotAfter, true
		}
	}

	return time.Time{}, false
}

/*
warn mails every owner of the connection's company.
*/
func (w *WarnExpiringSSOCertificates) warn(ctx context.Context, row models.SSOConnection, expiry time.Time) error {
	owners, err := models.New(w.DB).ListOrganizationOwners(ctx, row.OrganizationID)
	if err != nil {
		return fmt.Errorf("list owners to warn: %w", err)
	}

	seen := map[string]bool{}

	for _, owner := range owners {
		if seen[owner.Email] {
			continue
		}

		seen[owner.Email] = true

		pending := w.Mail.To(mail.Address{Name: owner.Name, Email: owner.Email})
		if owner.Locale != "" {
			pending.Locale(owner.Locale)
		}

		if err := pending.Queue(ctx, appmail.SSOCertificateExpiringMail{
			Name: owner.Name, Connection: row.Name, Date: expiry.UTC().Format("2 January 2006"),
			SettingsURL: w.Links.SingleSignOn(),
		}); err != nil {
			return fmt.Errorf("queue certificate warning: %w", err)
		}
	}

	return nil
}

/*
warningDay reports whether owners hear on the day the certificate has
this many days left: thirty, seven and one.
*/
func warningDay(left int) bool {
	return left == 30 || left == 7 || left == 1
}
