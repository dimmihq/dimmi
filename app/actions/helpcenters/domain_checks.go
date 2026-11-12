/*
|--------------------------------------------------------------------------
| Checking Help Center Domains
|--------------------------------------------------------------------------
|
| A domain is checked from the queue, never inside a request, since DNS
| can be slow. A domain that does not verify yet is looked at again
| soon, then less often, and failed with the reason once the schedule
| gives up. A verified domain is looked at again every day: a lapse keeps
| it serving while the schedule runs, so a passing DNS hiccup never takes
| a help center offline, and only a lapse that outlasts the schedule
| takes the domain away.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
recheckEvery is how often a verified domain is looked at again.
*/
const recheckEvery = 24 * time.Hour

/*
Handle checks one domain of the workspace and stores what it found. A
domain removed since the check was queued is skipped.
*/
func (d *Domains) Handle(ctx context.Context, workspaceID, id ids.UUID) error {
	scoped := tenancy.With(ctx, workspaceID.String())
	var verified string
	err := d.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := q.FindHelpCenterDomainForUpdate(ctx, models.FindHelpCenterDomainForUpdateParams{
			WorkspaceID: workspaceID, ID: id,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find help center domain: %w", err)
		}
		dressed := d.dress(row)
		result, err := d.Verifier.Check(ctx, row.Hostname, customdomain.Expect{
			TXTName: dressed.TXTName, TXTValue: dressed.TXTValue, CNAME: d.Target, Addrs: d.targetAddrs(ctx),
			ProbeToken: row.VerificationToken,
		})
		if err != nil {
			result = customdomain.Result{Findings: []error{err}}
		}
		next, err := d.save(ctx, q, row, result)
		if err == nil && next.Status == DomainVerified && row.Status != DomainVerified {
			verified = row.Hostname
		}
		return err
	})
	if err == nil && verified != "" && d.TLS != nil {
		_ = d.TLS.Ensure(ctx, verified)
	}
	return err
}

/*
save stores the outcome of a check and points the help center at the
domain, or away from it, when that changed.
*/
func (d *Domains) save(ctx context.Context, q *models.Queries, row models.HelpCenterDomain,
	result customdomain.Result,
) (models.HelpCenterDomain, error) {
	now := d.Clock.Now()
	p := models.SaveHelpCenterDomainCheckParams{
		Status: row.Status, CheckCount: row.CheckCount + 1, FirstCheckedAt: row.FirstCheckedAt,
		LastCheckedAt: &now, VerifiedAt: row.VerifiedAt, Now: now, WorkspaceID: row.WorkspaceID, ID: row.ID,
	}
	if result.Verified() {
		passed(&p, row, now)
	} else {
		d.lapsed(&p, row, result, now)
	}
	out, err := q.SaveHelpCenterDomainCheck(ctx, p)
	if err != nil {
		return out, fmt.Errorf("save domain check: %w", err)
	}
	serving := p.Status == DomainVerified
	if serving == (row.Status == DomainVerified) {
		return out, nil
	}
	var name *string
	if serving {
		name = &row.Hostname
	}
	err = q.SetHelpCenterCustomDomain(ctx, models.SetHelpCenterCustomDomainParams{
		CustomDomain: name, Now: now, WorkspaceID: row.WorkspaceID, ID: row.HelpCenterID,
	})
	if err != nil {
		return out, fmt.Errorf("point help center at domain: %w", err)
	}
	return out, nil
}

/*
passed marks a domain verified and due its daily check.
*/
func passed(p *models.SaveHelpCenterDomainCheckParams, row models.HelpCenterDomain, now time.Time) {
	p.Status, p.CheckCount, p.FirstCheckedAt, p.FailureReason = DomainVerified, 0, nil, nil
	p.NextCheckAt = new(now.Add(recheckEvery))
	if row.Status != DomainVerified {
		p.VerifiedAt = &now
	}
}

/*
lapsed keeps why a check failed and when to look again, failing the
domain once the schedule gives up.
*/
func (d *Domains) lapsed(p *models.SaveHelpCenterDomainCheckParams, row models.HelpCenterDomain,
	result customdomain.Result, now time.Time,
) {
	if p.FirstCheckedAt == nil {
		p.FirstCheckedAt = &now
	}
	p.FailureReason = new(reasonOf(d.dress(row), result))
	wait, more := d.Schedule.Next(int(p.CheckCount), now.Sub(*p.FirstCheckedAt))
	if more {
		p.NextCheckAt = new(now.Add(wait))
		return
	}
	p.Status, p.NextCheckAt = DomainFailed, nil
}

/*
targetAddrs returns the addresses the target resolves to, which a
domain whose CNAME chain runs on past the target, or a provider that
flattens it, resolves to as well.
*/
func (d *Domains) targetAddrs(ctx context.Context) []netip.Addr {
	if d.Resolver == nil {
		return nil
	}
	addrs, err := d.Resolver.Addrs(ctx, d.Target)
	if err != nil {
		return nil
	}
	out := make([]netip.Addr, len(addrs))
	for i, a := range addrs {
		out[i] = a.Unmap()
	}
	return out
}

/*
reasonOf says in words what the team has to fix.
*/
func reasonOf(d Domain, result customdomain.Result) string {
	var out []string
	for _, f := range result.Findings {
		switch {
		case errors.Is(f, customdomain.ErrOwnershipMissing):
			out = append(out, "No TXT record at "+d.TXTName+" yet.")
		case errors.Is(f, customdomain.ErrOwnershipMismatch):
			out = append(out, "The TXT record at "+d.TXTName+" does not hold the value shown.")
		case errors.Is(f, customdomain.ErrRoutingMismatch):
			out = append(out, d.Hostname+" does not point at "+d.Target+" yet.")
		default:
			out = append(out, "The DNS lookup did not answer; we will try again.")
		}
	}
	if len(out) == 0 {
		return "The records do not match yet."
	}
	return strings.Join(out, " ")
}

/*
forgetDomain deletes the domain of the help center the parameters name
and stops it answering there.
*/
func forgetDomain(ctx context.Context, q *models.Queries, h models.SetHelpCenterCustomDomainParams) error {
	if err := q.DeleteHelpCenterDomain(ctx, models.DeleteHelpCenterDomainParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	}); err != nil {
		return fmt.Errorf("delete help center domain: %w", err)
	}
	h.CustomDomain = nil
	if err := q.SetHelpCenterCustomDomain(ctx, h); err != nil {
		return fmt.Errorf("clear help center domain: %w", err)
	}
	return nil
}
