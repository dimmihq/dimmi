/*
|--------------------------------------------------------------------------
| Sending Domain States
|--------------------------------------------------------------------------
|
| Where a check leaves a domain, and what follows from it. A domain that
| becomes verified sends for every address on it from the next reply on;
| one that degrades or fails sends from the default sender at once, since
| unsigned mail from the customer's domain would land in spam. Checks are
| frequent right after someone asked and thin out after, a verified
| domain is looked at daily, spread over the day so the install never
| checks every domain at once.
|
*/

package email

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
verifiedCheckSpread is the window the daily checks of verified domains
spread over.
*/
const verifiedCheckSpread = 6 * time.Hour

/*
transition is where a check moves a domain: the new status, why it
failed, and how many checks in a row did not pass.
*/
type transition struct {
	status   string
	reason   string
	attempts int32
}

/*
decide returns where the outcome moves the row.
*/
func (a *CheckEmailDomain) decide(row models.EmailDomain, found checkOutcome, now time.Time) transition {
	stay := transition{status: row.Status, attempts: row.CheckAttempts}
	if found.result.Unreachable() {
		return stay
	}

	switch row.Status {
	case DomainVerifying:
		return a.decideVerifying(row, found, now)
	case DomainVerified:
		return decideVerified(row, found)
	case DomainDegraded:
		return decideDegraded(row, found, now)
	}

	return stay
}

/*
decideVerifying verifies a passing domain no other workspace still holds,
and fails one that did not pass within the window.
*/
func (a *CheckEmailDomain) decideVerifying(row models.EmailDomain, found checkOutcome, now time.Time) transition {
	if found.passing && !found.contested {
		return transition{status: DomainVerified}
	}

	if row.VerifyingSince != nil && now.Sub(*row.VerifyingSince) > verifyWindow {
		return transition{status: DomainFailed, reason: FailureNotVerified}
	}

	return transition{status: DomainVerifying, attempts: row.CheckAttempts + 1}
}

/*
decideVerified degrades a domain after checks in a row did not pass.
*/
func decideVerified(row models.EmailDomain, found checkOutcome) transition {
	if found.passing {
		return transition{status: DomainVerified}
	}

	attempts := row.CheckAttempts + 1
	if attempts >= degradeAfter {
		return transition{status: DomainDegraded, reason: FailureRecordsRemoved, attempts: attempts}
	}

	return transition{status: DomainVerified, attempts: attempts}
}

/*
decideDegraded restores a domain whose records came back, and fails one
degraded too long.
*/
func decideDegraded(row models.EmailDomain, found checkOutcome, now time.Time) transition {
	if found.passing {
		return transition{status: DomainVerified}
	}

	if row.DegradedAt != nil && now.Sub(*row.DegradedAt) > degradedWindow {
		return transition{status: DomainFailed, reason: FailureRecordsRemoved}
	}

	return transition{status: DomainDegraded, reason: FailureRecordsRemoved, attempts: row.CheckAttempts + 1}
}

/*
save stores the check and applies what the new state means.
*/
func (a *CheckEmailDomain) save(
	ctx context.Context, tx *database.Tx, row models.EmailDomain, found checkOutcome,
) error {
	now := a.Clock.Now()
	next := a.decide(row, found, now)
	warnings := found.result.Warnings

	if found.holder && found.contested {
		warnings = append(warnings, dns.Warning{Code: WarningDomainClaimed, Host: row.Domain})
	}

	params := a.checkParams(row, next, now)
	params.Records, params.Warnings = encode(found.result.Records), encode(nonNil(warnings))
	params.SesIdentityStatus = optional(found.inspection.Status)

	if found.restarted {
		params.SesDkimRestartedAt = &now
	}

	if next.status == DomainVerified && found.result.Passing(dns.PurposeOwnership) {
		params.OwnershipVerifiedAt = &now
	}

	q := models.New(tx)
	if next.status == DomainVerified && row.Status != DomainVerified && found.holder {
		if err := a.takeOver(ctx, tx, row, now); err != nil {
			return err
		}
	}

	saved, err := q.SaveEmailDomainCheck(ctx, params)
	if err != nil {
		return fmt.Errorf("save email domain check: %w", err)
	}

	if err := a.finishRotation(ctx, q, saved, found); err != nil {
		return err
	}

	return a.apply(ctx, tx, row, saved)
}

/*
nonNil returns an empty list instead of nil, so the column holds [].
*/
func nonNil(warnings []dns.Warning) []dns.Warning {
	if warnings == nil {
		return []dns.Warning{}
	}

	return warnings
}

/*
checkParams copies the row into the save, with the transition applied.
*/
func (a *CheckEmailDomain) checkParams(
	row models.EmailDomain, next transition, now time.Time,
) models.SaveEmailDomainCheckParams {
	params := models.SaveEmailDomainCheckParams{
		WorkspaceID: row.WorkspaceID, ID: row.ID, Now: now, Status: next.status, CheckAttempts: next.attempts,
		OwnershipVerifiedAt: row.OwnershipVerifiedAt, ProviderTrustedAt: row.ProviderTrustedAt,
		LastCheckedAt: &now, VerifiedAt: row.VerifiedAt, DegradedAt: row.DegradedAt, FailedAt: row.FailedAt,
		DkimTokens: row.DkimTokens, DkimSigningHostedZone: row.DkimSigningHostedZone,
		ProviderDomainID: row.ProviderDomainID, Driver: row.Driver, VerifyingSince: row.VerifyingSince,
		SesDkimRestartedAt: row.SesDkimRestartedAt, DkimSelector: row.DkimSelector,
		DkimPrivateKeyEncrypted: row.DkimPrivateKeyEncrypted, DkimNextSelector: row.DkimNextSelector,
		DkimNextPrivateKeyEncrypted: row.DkimNextPrivateKeyEncrypted, MailFromDomain: row.MailFromDomain,
		FailureReason: optional(next.reason),
	}

	if manual(row.Driver) && row.ProviderDomainID == nil {
		params.ProviderTrustedAt = nil
		if a.trusted(row.Domain) {
			params.ProviderTrustedAt = &now
		}
	}

	if next.status != row.Status {
		stamp(&params, next.status, now)
	}

	params.NextCheckAt = a.nextCheck(row, next, now)

	return params
}

/*
stamp records when the domain entered its new state.
*/
func stamp(params *models.SaveEmailDomainCheckParams, status string, now time.Time) {
	switch status {
	case DomainVerified:
		params.VerifiedAt, params.DegradedAt, params.FailedAt = &now, nil, nil
	case DomainDegraded:
		params.DegradedAt = &now
	case DomainFailed:
		params.FailedAt = &now
	}
}

/*
nextCheck returns when the domain is checked again: often right after
someone asked, then less and less often; daily once verified; hourly
while degraded or after a failed check of a verified domain; never once
failed.
*/
func (a *CheckEmailDomain) nextCheck(row models.EmailDomain, next transition, now time.Time) *time.Time {
	var wait time.Duration

	switch {
	case next.status == DomainFailed:
		return nil
	case next.status == DomainVerified && next.attempts == 0:
		wait = 24*time.Hour - verifiedCheckSpread/2 + spread(row.ID)
	case next.status == DomainVerified, next.status == DomainDegraded:
		wait = time.Hour
	default:
		wait = verifyingWait(now.Sub(*cmpTime(row.CheckRequestedAt, row.VerifyingSince, &now)))
	}

	at := now.Add(wait)

	return &at
}

/*
verifyingWait is the pause between checks of a verifying domain, by how
long ago someone last asked.
*/
func verifyingWait(since time.Duration) time.Duration {
	switch {
	case since < 10*time.Minute:
		return time.Minute
	case since < time.Hour:
		return 5 * time.Minute
	case since < 24*time.Hour:
		return 15 * time.Minute
	}

	return time.Hour
}

/*
spread places a domain's daily check in the spread window by its id, so
the same domain is always checked at the same time of day.
*/
func spread(id ids.UUID) time.Duration {
	h := fnv.New64a()
	_, _ = h.Write(id[:])

	return time.Duration(h.Sum64() % uint64(verifiedCheckSpread))
}

/*
cmpTime returns the first time that is set.
*/
func cmpTime(times ...*time.Time) *time.Time {
	for _, t := range times {
		if t != nil {
			return t
		}
	}

	return nil
}
