/*
|--------------------------------------------------------------------------
| Record Checker
|--------------------------------------------------------------------------
|
| Reads a domain's records back and says, per record, what was found. Two
| resolvers are asked and must agree before a record counts as verified,
| so a stale cache neither passes a domain nor fails it; when they
| disagree the record is propagating and nobody has to do anything. A
| lookup that fails says nothing about the record, so it never fails or
| degrades a domain: it is only reported as an error and tried again.
|
| Values compare without regard to case, whitespace or quotes, because DNS
| hosts show them in different ways. What was actually found is kept
| beside the expected value, so the page can show both.
|
*/

package dns

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	fdns "github.com/dimmihq/dimmi/framework/dns"
)

/*
Checker checks records against the given resolvers, each asked for every
record.
*/
type Checker struct {
	Resolvers []fdns.Resolver
}

/*
Result is what one check found: the records with their status, and the
mistakes it recognized.
*/
type Result struct {
	Records  []Record
	Warnings []Warning
}

/*
Unreachable reports whether no resolver answered for any required record,
the case where nothing may be concluded about the domain.
*/
func (r Result) Unreachable() bool {
	return !slices.ContainsFunc(r.Records, func(rec Record) bool {
		return rec.Required && rec.Status != StatusError
	})
}

/*
Passing reports whether every required record of the purpose is verified.
It is true when none of them is required.
*/
func (r Result) Passing(purpose string) bool {
	return !slices.ContainsFunc(r.Records, func(rec Record) bool {
		return rec.Required && rec.Purpose == purpose && rec.Status != StatusVerified
	})
}

/*
Check reads every record and looks for the mistakes people make when
copying them. Others holds ownership tokens of other workspaces claiming
the domain.
*/
func (c *Checker) Check(ctx context.Context, spec Spec, records []Record, others []string) Result {
	out := make([]Record, len(records))
	for i, record := range records {
		out[i] = c.checkRecord(ctx, record)
	}

	return Result{Records: out, Warnings: c.detect(ctx, spec, out, others)}
}

/*
observation is one resolver's view of a record: the values it holds, and
whether the lookup failed.
*/
type observation struct {
	values []string
	failed bool
}

/*
checkRecord asks every resolver for the record and combines their views.
*/
func (c *Checker) checkRecord(ctx context.Context, record Record) Record {
	views := make([]observation, 0, len(c.Resolvers))
	for _, resolver := range c.Resolvers {
		views = append(views, lookup(ctx, resolver, record))
	}

	record.Status, record.Observed = combine(record, views)

	return record
}

/*
combine turns the resolvers' views into a status and what was observed.
*/
func combine(record Record, views []observation) (string, string) {
	in := tally{views: len(views)}

	for _, view := range views {
		if view.failed {
			in.failed++

			continue
		}

		if len(view.values) > 0 && in.found == "" {
			in.found = strings.Join(view.values, " | ")
		}

		if matches(record, view.values) {
			in.matched++
		}
	}

	return verdict(record, in), in.found
}

/*
tally counts what the resolvers said about a record.
*/
type tally struct {
	views, matched, failed int
	found                  string
}

/*
verdict names the status a record's views add up to. Guidance records stay
guidance whatever is found.
*/
func verdict(record Record, in tally) string {
	switch {
	case record.Status == StatusInfo:
		return StatusInfo
	case in.views == 0 || in.failed == in.views:
		return StatusError
	case in.matched == in.views:
		return StatusVerified
	case in.matched > 0 || in.failed > 0:
		return StatusPropagating
	case in.found != "":
		return StatusMismatch
	}

	return StatusMissing
}

/*
lookup reads the record's type at its host.
*/
func lookup(ctx context.Context, resolver fdns.Resolver, record Record) observation {
	found, err := read(ctx, resolver, record)
	if errors.Is(err, fdns.ErrNotFound) {
		return observation{}
	}

	if err != nil {
		return observation{failed: true}
	}

	return observation{values: found}
}

/*
read returns what the resolver holds of the record's type at its host. An
alias that resolves to itself is no alias, so it counts as not found.
*/
func read(ctx context.Context, resolver fdns.Resolver, record Record) ([]string, error) {
	switch record.Type {
	case TypeMX:
		exchangers, err := resolver.MX(ctx, record.Host)
		if err != nil {
			return nil, fmt.Errorf("mx %s: %w", record.Host, err)
		}

		found := make([]string, 0, len(exchangers))
		for _, mx := range exchangers {
			found = append(found, mx.Host)
		}

		return found, nil
	case TypeCNAME:
		target, err := resolver.CNAME(ctx, record.Host)
		if err != nil {
			return nil, fmt.Errorf("cname %s: %w", record.Host, err)
		}

		if strings.EqualFold(target, record.Host) {
			return nil, fdns.ErrNotFound
		}

		return []string{target}, nil
	}

	found, err := resolver.TXT(ctx, record.Host)
	if err != nil {
		return nil, fmt.Errorf("txt %s: %w", record.Host, err)
	}

	return relevant(record, found), nil
}

/*
relevant keeps the TXT values that could be this record: other kinds of
text at the root, such as a site verification, are no mismatch.
*/
func relevant(record Record, found []string) []string {
	prefix := ""

	switch record.Purpose {
	case PurposeSPF, PurposeReturnPathSPF:
		prefix = "v=spf1"
	case PurposeDMARC:
		prefix = "v=dmarc1"
	case PurposeOwnership:
		prefix = ownershipValue
	}

	if prefix == "" {
		return found
	}

	kept := make([]string, 0, len(found))
	for _, value := range found {
		if strings.HasPrefix(strings.ToLower(normalize(value)), prefix) {
			kept = append(kept, value)
		}
	}

	return kept
}

/*
matches reports whether what was found satisfies the record. SPF passes
when the one record holds the needed mechanism and DMARC when any policy
exists, since we never replace a customer's own.
*/
func matches(record Record, found []string) bool {
	if len(found) == 0 {
		return false
	}

	switch record.Purpose {
	case PurposeDMARC:
		return len(found) == 1
	case PurposeSPF, PurposeReturnPathSPF:
		return len(found) == 1 && spfHolds(found[0], record.Value)
	}

	want := normalize(record.Value)

	return slices.ContainsFunc(found, func(value string) bool {
		return strings.EqualFold(normalize(value), want)
	})
}

/*
spfHolds reports whether the found record has every include of the wanted
one.
*/
func spfHolds(found, wanted string) bool {
	have := strings.Fields(strings.ToLower(found))

	for field := range strings.FieldsSeq(strings.ToLower(wanted)) {
		if strings.HasPrefix(field, "include:") && !slices.Contains(have, field) {
			return false
		}
	}

	return true
}

/*
normalize drops quotes, whitespace and any trailing root dot, which hosts
show differently.
*/
func normalize(value string) string {
	value = strings.ReplaceAll(value, `"`, "")
	value = strings.Join(strings.Fields(value), "")

	return strings.TrimSuffix(value, ".")
}
