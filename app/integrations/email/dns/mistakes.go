/*
|--------------------------------------------------------------------------
| Common Mistakes
|--------------------------------------------------------------------------
|
| Nearly every failed domain setup is one of a few copying mistakes, and a
| record table alone does not explain them. Each detector looks for one
| and names it by a code the page turns into plain words: what is wrong,
| which record, and what to do. A code here always has its words in the
| language files, and the product's own tests fail when one does not.
|
*/

package dns

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	fdns "github.com/dimmihq/dimmi/framework/dns"
)

/*
The mistakes a check recognizes.
*/
const (
	WarningDoubledDomain     = "doubled_domain"
	WarningCloudflareProxied = "cloudflare_proxied"
	WarningDuplicateSPF      = "duplicate_spf"
	WarningSPFTooManyLookups = "spf_too_many_lookups"
	WarningSPFMissingInclude = "spf_missing_include"
	WarningDMARCMissing      = "dmarc_missing"
	WarningDMARCMultiple     = "dmarc_multiple"
	WarningDMARCReject       = "dmarc_reject_without_alignment"
	WarningMXConflict        = "mx_conflict_return_path"
	WarningCNAMEWithOthers   = "cname_with_other_records"
	WarningWrongRecordType   = "wrong_selector_record_type"
	WarningOtherToken        = "ownership_token_other_workspace"
	WarningPropagating       = "propagating"
)

/*
Warning is one mistake found: its code, the record's host and, where it
helps, the value that fixes it.
*/
type Warning struct {
	Code  string `json:"code"`
	Host  string `json:"host,omitempty"`
	Value string `json:"value,omitempty"`
}

/*
WarningCodes lists every code a check can return.
*/
func WarningCodes() []string {
	return []string{
		WarningDoubledDomain, WarningCloudflareProxied, WarningDuplicateSPF, WarningSPFTooManyLookups,
		WarningSPFMissingInclude, WarningDMARCMissing, WarningDMARCMultiple, WarningDMARCReject, WarningMXConflict,
		WarningCNAMEWithOthers, WarningWrongRecordType, WarningOtherToken, WarningPropagating,
	}
}

/*
Limits on reading SPF: the lookups RFC 7208 allows, and how deep includes
are followed, so a loop in someone's records ends.
*/
const (
	maxSPFLookups = 10
	maxSPFDepth   = 5
)

/*
cloudflareRanges are the addresses a proxied Cloudflare record answers
with, which a signing alias must never do.
*/
func cloudflareRanges() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("104.16.0.0/13"), netip.MustParsePrefix("104.24.0.0/14"),
		netip.MustParsePrefix("172.64.0.0/13"), netip.MustParsePrefix("162.158.0.0/15"),
		netip.MustParsePrefix("188.114.96.0/20"), netip.MustParsePrefix("2606:4700::/32"),
	}
}

/*
detect runs every detector. It reads through the first resolver only:
mistakes are about what the host holds, not about propagation.
*/
func (c *Checker) detect(ctx context.Context, spec Spec, records []Record, others []string) []Warning {
	if len(c.Resolvers) == 0 {
		return nil
	}

	r := c.Resolvers[0]

	var found []Warning

	in := scan{resolver: r, spec: spec, others: others}
	for _, record := range records {
		found = append(found, in.recordMistakes(ctx, record)...)
	}

	found = append(found, spfMistakes(ctx, r, spec)...)
	found = append(found, dmarcMistakes(ctx, r, spec, records)...)

	if slices.ContainsFunc(records, func(rec Record) bool { return rec.Status == StatusPropagating }) {
		found = append(found, Warning{Code: WarningPropagating})
	}

	return found
}

/*
scan is what record checks read through: the resolver, the domain's spec
and the ownership tokens of other workspaces claiming it.
*/
type scan struct {
	resolver fdns.Resolver
	spec     Spec
	others   []string
}

/*
recordMistakes looks for the mistakes one record's name can show.
*/
func (in scan) recordMistakes(ctx context.Context, rec Record) []Warning {
	if rec.Status == StatusVerified || rec.Status == StatusError {
		return nil
	}

	var found []Warning

	if doubled(ctx, in.resolver, in.spec, rec) {
		found = append(found, Warning{
			Code: WarningDoubledDomain, Host: rec.Host, Value: rec.Host + "." + in.spec.Domain,
		})
	}

	switch rec.Purpose {
	case PurposeDKIM:
		found = append(found, dkimMistakes(ctx, in.resolver, rec)...)
	case PurposeOwnership:
		found = append(found, ownershipMistakes(ctx, in.resolver, rec, in.others)...)
	case PurposeReturnPath:
		found = append(found, returnPathMistakes(rec)...)
	}

	return found
}

/*
doubled reports whether the record sits at its name with the domain added
again, which happens when a full name is pasted into a host that appends
the domain.
*/
func doubled(ctx context.Context, r fdns.Resolver, spec Spec, rec Record) bool {
	name := rec.Host + "." + spec.Domain

	switch rec.Type {
	case TypeCNAME:
		target, err := r.CNAME(ctx, name)

		return err == nil && !strings.EqualFold(target, name)
	case TypeMX:
		exchangers, err := r.MX(ctx, name)

		return err == nil && len(exchangers) > 0
	}

	texts, err := r.TXT(ctx, name)

	return err == nil && len(texts) > 0
}

/*
dkimMistakes tells a proxied alias and a record of the wrong type from a
missing one.
*/
func dkimMistakes(ctx context.Context, r fdns.Resolver, rec Record) []Warning {
	if rec.Type == TypeCNAME {
		if texts, err := r.TXT(ctx, rec.Host); err == nil && len(texts) > 0 {
			return []Warning{{Code: WarningWrongRecordType, Host: rec.Host}}
		}

		addrs, err := r.Addrs(ctx, rec.Host)
		if err == nil && slices.ContainsFunc(addrs, proxied) {
			return []Warning{{Code: WarningCloudflareProxied, Host: rec.Host}}
		}

		return nil
	}

	target, err := r.CNAME(ctx, rec.Host)
	if err == nil && !strings.EqualFold(target, rec.Host) && rec.Status != StatusInfo {
		return []Warning{{Code: WarningWrongRecordType, Host: rec.Host}}
	}

	return nil
}

/*
proxied reports whether the address is in Cloudflare's proxy ranges.
*/
func proxied(addr netip.Addr) bool {
	return slices.ContainsFunc(cloudflareRanges(), func(prefix netip.Prefix) bool { return prefix.Contains(addr) })
}

/*
ownershipMistakes finds an alias where text belongs, and a token copied
from another workspace.
*/
func ownershipMistakes(ctx context.Context, r fdns.Resolver, rec Record, others []string) []Warning {
	var found []Warning

	if target, err := r.CNAME(ctx, rec.Host); err == nil && !strings.EqualFold(target, rec.Host) {
		found = append(found, Warning{Code: WarningCNAMEWithOthers, Host: rec.Host})
	}

	texts, err := r.TXT(ctx, rec.Host)
	if err != nil {
		return found
	}

	for _, text := range texts {
		if slices.Contains(others, strings.TrimPrefix(normalize(text), ownershipValue)) {
			return append(found, Warning{Code: WarningOtherToken, Host: rec.Host})
		}
	}

	return found
}

/*
returnPathMistakes finds another mail server already on the return path
name, which SES would refuse, so a different subdomain is the fix.
*/
func returnPathMistakes(rec Record) []Warning {
	if rec.Status != StatusMismatch {
		return nil
	}

	return []Warning{{Code: WarningMXConflict, Host: rec.Host, Value: "dimmi-bounce"}}
}

/*
rootSPF returns the SPF records among the domain's root TXT records. A
name without text simply has none.
*/
func rootSPF(ctx context.Context, r fdns.Resolver, domain string) ([]string, error) {
	texts, err := r.TXT(ctx, domain)
	if errors.Is(err, fdns.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("spf %s: %w", domain, err)
	}

	var spf []string

	for _, text := range texts {
		if strings.HasPrefix(strings.ToLower(normalize(text)), "v=spf1") {
			spf = append(spf, text)
		}
	}

	return spf, nil
}

/*
spfMistakes reads the root SPF record: two of them fail all mail, too many
lookups fail it at evaluation, and a relay must be included.
*/
func spfMistakes(ctx context.Context, r fdns.Resolver, spec Spec) []Warning {
	spf, err := rootSPF(ctx, r, spec.Domain)
	if err != nil {
		return nil
	}

	switch {
	case len(spf) > 1:
		merged := strings.TrimSpace(spf[0])
		if spec.SPFInclude != "" {
			merged = MergeSPF(merged, spec.SPFInclude)
		}

		return []Warning{{Code: WarningDuplicateSPF, Host: spec.Domain, Value: merged}}
	case len(spf) == 0:
		if spec.SPFInclude == "" {
			return nil
		}

		return []Warning{{Code: WarningSPFMissingInclude, Host: spec.Domain, Value: MergeSPF("", spec.SPFInclude)}}
	}

	var found []Warning

	if spec.SPFInclude != "" && !slices.Contains(strings.Fields(strings.ToLower(spf[0])), spec.SPFInclude) {
		found = append(found, Warning{
			Code: WarningSPFMissingInclude, Host: spec.Domain, Value: MergeSPF(spf[0], spec.SPFInclude),
		})
	}

	if spfLookups(ctx, r, spf[0], 0) > maxSPFLookups {
		found = append(found, Warning{Code: WarningSPFTooManyLookups, Host: spec.Domain})
	}

	return found
}

/*
spfLookups counts the DNS mechanisms of an SPF record, following includes.
*/
func spfLookups(ctx context.Context, r fdns.Resolver, record string, depth int) int {
	if depth > maxSPFDepth {
		return 0
	}

	count := 0

	for field := range strings.FieldsSeq(strings.ToLower(record)) {
		term := strings.TrimLeft(field, "+-~?")

		switch {
		case strings.HasPrefix(term, "include:"):
			count += 1 + spfIncluded(ctx, r, strings.TrimPrefix(term, "include:"), depth)
		case term == "a", term == "mx", term == "ptr", strings.HasPrefix(term, "a:"), strings.HasPrefix(term, "mx:"),
			strings.HasPrefix(term, "exists:"), strings.HasPrefix(term, "redirect="):
			count++
		}
	}

	return count
}

/*
spfIncluded counts the lookups of an included domain's SPF record.
*/
func spfIncluded(ctx context.Context, r fdns.Resolver, domain string, depth int) int {
	spf, err := rootSPF(ctx, r, domain)
	if err != nil || len(spf) == 0 {
		return 0
	}

	return spfLookups(ctx, r, spf[0], depth+1)
}

/*
dmarcMistakes reads the DMARC record: none, several, or a reject policy
while our records are not in place, in which case mail would be refused if
it left from this domain now.
*/
func dmarcMistakes(ctx context.Context, r fdns.Resolver, spec Spec, records []Record) []Warning {
	host := dmarcLabel + "." + spec.Domain

	texts, err := r.TXT(ctx, host)
	if err != nil && !errors.Is(err, fdns.ErrNotFound) {
		return nil
	}

	var policies []string

	for _, text := range texts {
		if strings.HasPrefix(strings.ToLower(normalize(text)), "v=dmarc1") {
			policies = append(policies, strings.ToLower(normalize(text)))
		}
	}

	switch {
	case len(policies) == 0:
		return []Warning{{Code: WarningDMARCMissing, Host: host}}
	case len(policies) > 1:
		return []Warning{{Code: WarningDMARCMultiple, Host: host}}
	}

	signed := !slices.ContainsFunc(records, func(rec Record) bool {
		return rec.Purpose == PurposeDKIM && rec.Required && rec.Status != StatusVerified
	})
	if strings.Contains(policies[0], "p=reject") && !signed {
		return []Warning{{Code: WarningDMARCReject, Host: host}}
	}

	return nil
}
