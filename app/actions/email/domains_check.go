/*
|--------------------------------------------------------------------------
| Checking a Sending Domain
|--------------------------------------------------------------------------
|
| A check reads the domain's records back and moves it through its
| states. A domain is verified once its ownership record and its signing
| records are in place, and the provider signs for it. A verified domain
| whose records vanish is only degraded after two checks an hour apart
| agree, since one bad answer from a resolver must never move mail off a
| customer's domain; a lookup that fails says nothing at all. A domain
| verifying for three days without passing fails, and a degraded one
| fails after a week.
|
| Only one workspace holds a verified domain. A second workspace that
| proves ownership while the holder's own record is gone takes it over;
| while both records are there, the first keeps it.
|
*/

package email

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Timing of the status machine: how long a domain may verify, stay
degraded, how many failed checks degrade it, and how often SES may be
asked to look for the signing records again.
*/
const (
	verifyWindow     = 72 * time.Hour
	degradedWindow   = 7 * 24 * time.Hour
	degradeAfter     = 2
	sesRestartPeriod = 24 * time.Hour
)

/*
WarningDomainClaimed says another workspace holds the domain and its
proof is still published.
*/
const WarningDomainClaimed = "domain_claimed"

/*
CheckEmailDomain reads a domain's records back and moves it to the state
they prove. It runs from the queue, scheduled or asked for.

Rules:
  - Pending, failed and removed domains are left alone.
  - A domain set up for another driver than the install's is set up
    again and verifies anew; mail falls back to the default sender
    meanwhile.

Errors: a wrapped database or provider error, so the job tries again.
*/
type CheckEmailDomain struct {
	*Domains
}

/*
checkOutcome is what a check found: the records, what the provider said,
whether the domain passes, and who else holds it.
*/
type checkOutcome struct {
	result     dns.Result
	inspection Inspection
	passing    bool
	holder     bool
	contested  bool
	restarted  bool
}

/*
Handle checks the domain of the workspace.
*/
func (a *CheckEmailDomain) Handle(ctx context.Context, workspaceID, id ids.UUID) error {
	return a.DB.Transaction(inWorkspace(ctx, workspaceID), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := q.FindEmailDomainForUpdate(ctx, models.FindEmailDomainForUpdateParams{
			WorkspaceID: workspaceID, ID: id,
		})
		if isNotFound(err) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find email domain: %w", err)
		}

		if row.Status == DomainPending || row.Status == DomainFailed {
			return nil
		}

		if row.Driver != a.Driver {
			return a.switchDriver(ctx, tx, row)
		}

		found, err := a.inspect(ctx, q, row)
		if err != nil {
			return err
		}

		return a.save(ctx, tx, row, found)
	})
}

/*
inspect reads the records and asks the provider about the domain.
*/
func (a *CheckEmailDomain) inspect(
	ctx context.Context, q *models.Queries, row models.EmailDomain,
) (checkOutcome, error) {
	var found checkOutcome

	if a.Provider != nil {
		inspection, err := a.Provider.Inspect(ctx, row)
		if err != nil {
			return found, providerError(err)
		}

		found.inspection = inspection
	}

	expected, err := a.expectedNow(ctx, row, found.inspection.Records)
	if err != nil {
		return found, err
	}

	others, holder, err := a.claims(ctx, q, row)
	if err != nil {
		return found, err
	}

	checker := dns.Checker{Resolvers: a.resolvers()}
	found.result = checker.Check(ctx, a.spec(row, ""), expected, others)
	found.passing = found.result.Passing(dns.PurposeOwnership) && a.signs(row, found)

	if holder != "" && found.result.Passing(dns.PurposeOwnership) {
		found.holder = true
		found.contested = a.published(ctx, row, holder)
	}

	found.restarted = a.restartSES(ctx, row, found)

	return found, nil
}

/*
expectedNow returns the records the row needs today, a relay key waiting
to take over included.
*/
func (a *CheckEmailDomain) expectedNow(
	ctx context.Context, row models.EmailDomain, provided []mail.DomainRecord,
) ([]dns.Record, error) {
	current, err := a.publicKey(ctx, row, row.DkimSelector, row.DkimPrivateKeyEncrypted)
	if err != nil {
		return nil, err
	}

	expected := a.expected(row, current, provided)
	if row.DkimNextSelector == nil {
		return expected, nil
	}

	next, err := a.publicKey(ctx, row, row.DkimNextSelector, row.DkimNextPrivateKeyEncrypted)
	if err != nil {
		return nil, err
	}

	upcoming := row
	upcoming.DkimSelector = row.DkimNextSelector
	record := a.expected(upcoming, next, nil)[1]
	record.Required = false
	record.Purpose = PurposeNextDKIM

	return slices.Insert(expected, 2, record), nil
}

/*
PurposeNextDKIM marks the record of a relay key that takes over once it
is seen.
*/
const PurposeNextDKIM = "dkim_next"

/*
signs reports whether whoever sends will sign for the domain: SES once
its own check passed, a relay once our key is published, a provider with
a domain API once it says so, and one set up by hand once the operator
listed it.
*/
func (a *CheckEmailDomain) signs(row models.EmailDomain, found checkOutcome) bool {
	switch {
	case row.Driver == dns.DriverSES:
		return found.result.Passing(dns.PurposeDKIM) && found.inspection.Signs
	case row.Driver == dns.DriverSMTP:
		return found.result.Passing(dns.PurposeDKIM)
	case row.ProviderDomainID != nil && a.Provider != nil:
		return found.inspection.Signs
	}

	return a.trusted(row.Domain)
}

/*
claims returns the ownership tokens of every other workspace claiming
the domain, and the token of the one holding it, if any.
*/
func (a *CheckEmailDomain) claims(
	ctx context.Context, q *models.Queries, row models.EmailDomain,
) ([]string, string, error) {
	rows, err := q.ListEmailDomainClaims(ctx, row.Domain)
	if err != nil {
		return nil, "", fmt.Errorf("list email domain claims: %w", err)
	}

	var (
		others []string
		holder string
	)

	for _, claim := range rows {
		if claim.WorkspaceID == row.WorkspaceID {
			continue
		}

		others = append(others, claim.OwnershipToken)
		if claim.Status == DomainVerified || claim.Status == DomainDegraded {
			holder = claim.OwnershipToken
		}
	}

	return others, holder, nil
}

/*
published reports whether the holder's ownership token is still in DNS.
A failed lookup counts as published, so a domain never moves on a bad
answer.
*/
func (a *CheckEmailDomain) published(ctx context.Context, row models.EmailDomain, token string) bool {
	checker := dns.Checker{Resolvers: a.resolvers()}
	holder := row
	holder.OwnershipToken = token
	spec := a.spec(holder, "")
	result := checker.Check(ctx, spec, dns.Expected(spec)[:1], nil)

	return result.Records[0].Status != dns.StatusMissing && result.Records[0].Status != dns.StatusMismatch
}

/*
restartSES asks SES to look for the signing records again when it gave up
but they are now in place, at most once a day.
*/
func (a *CheckEmailDomain) restartSES(ctx context.Context, row models.EmailDomain, found checkOutcome) bool {
	if row.Driver != dns.DriverSES || !found.inspection.Restartable || !found.result.Passing(dns.PurposeDKIM) {
		return false
	}

	if row.SesDkimRestartedAt != nil && a.Clock.Now().Sub(*row.SesDkimRestartedAt) < sesRestartPeriod {
		return false
	}

	return a.Provider.Restart(ctx, row) == nil
}
