/*
|--------------------------------------------------------------------------
| DMARC Alignment
|--------------------------------------------------------------------------
|
| SPF and DKIM vouch for some domain, not necessarily the one in From.
| DMARC ties them together: a pass counts only when its domain matches
| the From domain, exactly under strict alignment or by organizational
| domain under relaxed. The policy is looked up at the From domain, then
| at its organizational domain, where the subdomain policy applies.
|
*/

package authcheck

import (
	"context"
	"errors"

	"github.com/emersion/go-msgauth/dmarc"
	"golang.org/x/net/publicsuffix"
)

/*
dmarc fills the DMARC result, policy and aligned domain.
*/
func (c *Checker) dmarc(ctx context.Context, results *Results) {
	if results.FromDomain == "" {
		results.DMARC = PermError

		return
	}

	record, result := c.lookupDMARC(ctx, results)

	modes := dmarc.Record{DKIMAlignment: dmarc.AlignmentRelaxed, SPFAlignment: dmarc.AlignmentRelaxed}
	if record != nil {
		modes = *record
	}

	results.AlignedDomain = aligned(*results, modes)

	switch {
	case result != "":
		results.DMARC = result
	case results.AlignedDomain != "":
		results.DMARC = Pass
	default:
		results.DMARC = Fail
	}
}

/*
lookupDMARC finds the record that applies to the From domain. A result
is returned when the lookup decides it: none without a record, or an
error.
*/
func (c *Checker) lookupDMARC(ctx context.Context, results *Results) (*dmarc.Record, Result) {
	options := &dmarc.LookupOptions{LookupTXT: lookupTXT(ctx, c.resolver)}

	record, err := dmarc.LookupWithOptions(results.FromDomain, options)

	org := organizational(results.FromDomain)
	if errors.Is(err, dmarc.ErrNoPolicy) && org != results.FromDomain {
		record, err = dmarc.LookupWithOptions(org, options)
		if err == nil && record.SubdomainPolicy != "" {
			record.Policy = record.SubdomainPolicy
		}
	}

	switch {
	case errors.Is(err, dmarc.ErrNoPolicy):
		return nil, None
	case dmarc.IsTempFail(err):
		return nil, TempError
	case err != nil:
		return nil, PermError
	}

	results.DMARCPolicy = string(record.Policy)

	return record, ""
}

/*
aligned returns the domain of the first passing DKIM signature, or else
the passing SPF domain, that aligns with the From domain.
*/
func aligned(results Results, modes dmarc.Record) string {
	for _, signature := range results.DKIM {
		if signature.Result == Pass && matches(signature.Domain, results.FromDomain, modes.DKIMAlignment) {
			return signature.Domain
		}
	}

	if results.SPF == Pass && matches(results.SPFDomain, results.FromDomain, modes.SPFAlignment) {
		return results.SPFDomain
	}

	return ""
}

func matches(domain, from string, mode dmarc.AlignmentMode) bool {
	if mode == dmarc.AlignmentStrict {
		return domain == from
	}

	return organizational(domain) == organizational(from)
}

/*
organizational returns the registrable domain, such as example.co.uk
for mail.example.co.uk, or the domain itself when it has none.
*/
func organizational(domain string) string {
	org, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return domain
	}

	return org
}
