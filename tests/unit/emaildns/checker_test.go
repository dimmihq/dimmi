package emaildns_test

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fdns "github.com/dimmihq/dimmi/framework/dns"

	"github.com/dimmihq/dimmi/app/integrations/email/dns"
)

/*
broken is a resolver whose every lookup fails, like a server that is down.
*/
type broken struct{}

var errDown = errors.New("server down")

func (broken) TXT(context.Context, string) ([]string, error)       { return nil, errDown }
func (broken) MX(context.Context, string) ([]fdns.MX, error)       { return nil, errDown }
func (broken) CNAME(context.Context, string) (string, error)       { return "", errDown }
func (broken) Addrs(context.Context, string) ([]netip.Addr, error) { return nil, errDown }
func (broken) NS(context.Context, string) ([]string, error)        { return nil, errDown }

/*
published returns a zone holding every record of the spec as expected.
*/
func published(spec dns.Spec) *fdns.Fake {
	zone := fdns.NewFake()
	txt := map[string][]string{}

	for _, record := range dns.Expected(spec) {
		switch record.Type {
		case dns.TypeCNAME:
			zone.Set(record.Host, fdns.Records{CNAME: record.Value}).Set(record.Value, target())
		case dns.TypeMX:
			zone.Set(record.Host, fdns.Records{MX: []fdns.MX{{Host: record.Value, Pref: 10}}, TXT: txt[record.Host]})
		default:
			txt[record.Host] = append(txt[record.Host], record.Value)
		}
	}

	for host, values := range txt {
		zone.Set(host, fdns.Records{TXT: values, MX: mxOf(spec, host)})
	}

	return zone
}

/*
target is what an alias points at: a name that exists.
*/
func target() fdns.Records {
	return fdns.Records{Addrs: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}
}

/*
mxOf keeps the return path's mail server when its text is set too.
*/
func mxOf(spec dns.Spec, host string) []fdns.MX {
	if host != spec.MailFrom {
		return nil
	}

	return []fdns.MX{{Host: "feedback-smtp." + spec.Region + ".amazonses.com", Pref: 10}}
}

/*
check runs one check of the spec's records against the resolvers.
*/
func check(t *testing.T, spec dns.Spec, others []string, resolvers ...fdns.Resolver) dns.Result {
	t.Helper()

	checker := dns.Checker{Resolvers: resolvers}

	return checker.Check(t.Context(), spec, dns.Expected(spec), others)
}

/*
codes returns the warning codes of a result.
*/
func codes(result dns.Result) []string {
	found := make([]string, 0, len(result.Warnings))
	for _, warning := range result.Warnings {
		found = append(found, warning.Code)
	}

	return found
}

/*
statusOf returns the status of the first record of the purpose.
*/
func statusOf(result dns.Result, purpose string) dns.Record {
	index := slices.IndexFunc(result.Records, func(r dns.Record) bool { return r.Purpose == purpose })

	return result.Records[index]
}

func TestChecker(t *testing.T) {
	t.Parallel()

	t.Run("verifies every record both resolvers see", func(t *testing.T) {
		t.Parallel()
		zone := published(sesSpec())

		got := check(t, sesSpec(), nil, zone, zone)

		for _, record := range got.Records {
			assert.Equal(t, dns.StatusVerified, record.Status, record.Host)
		}
		assert.True(t, got.Passing(dns.PurposeOwnership))
		assert.True(t, got.Passing(dns.PurposeDKIM))
		assert.False(t, got.Unreachable())
		assert.Empty(t, got.Warnings)
	})

	t.Run("calls a record one resolver lacks propagating", func(t *testing.T) {
		t.Parallel()

		got := check(t, sesSpec(), nil, published(sesSpec()), fdns.NewFake())

		assert.Equal(t, dns.StatusPropagating, statusOf(got, dns.PurposeOwnership).Status)
		assert.False(t, got.Passing(dns.PurposeOwnership))
		assert.Contains(t, codes(got), dns.WarningPropagating)
	})

	t.Run("reports a missing record and a different value", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().Set("_dimmi-verification.acme.test", fdns.Records{TXT: []string{"dimmi-verification=x"}})

		got := check(t, sesSpec(), nil, zone)

		ownership := statusOf(got, dns.PurposeOwnership)
		assert.Equal(t, dns.StatusMismatch, ownership.Status)
		assert.Equal(t, "dimmi-verification=x", ownership.Observed)
		assert.Equal(t, dns.StatusMissing, statusOf(got, dns.PurposeDKIM).Status)
	})

	t.Run("says nothing about a domain no resolver could read", func(t *testing.T) {
		t.Parallel()

		got := check(t, sesSpec(), nil, broken{}, broken{})

		assert.True(t, got.Unreachable())
		assert.Equal(t, dns.StatusError, statusOf(got, dns.PurposeDKIM).Status)
		assert.Equal(t, dns.StatusError, check(t, sesSpec(), nil).Records[0].Status)
	})

	t.Run("ignores quotes, case and spacing hosts add to text", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().Set("_dimmi-verification.acme.test", fdns.Records{
			TXT: []string{`"Dimmi-Verification=TOK"`, "google-site-verification=x"},
		})

		got := check(t, sesSpec(), nil, zone)

		assert.Equal(t, dns.StatusVerified, statusOf(got, dns.PurposeOwnership).Status)
	})

	t.Run("keeps guidance as guidance whatever is found", func(t *testing.T) {
		t.Parallel()
		spec := dns.Spec{Domain: "acme.test", OwnershipToken: "tok", Driver: dns.DriverPostmark}

		got := check(t, spec, nil, published(spec))

		assert.Equal(t, dns.StatusInfo, statusOf(got, dns.PurposeReturnPath).Status)
	})

	t.Run("accepts any one spf record holding the relay include", func(t *testing.T) {
		t.Parallel()
		zone := published(relaySpec()).
			Set("acme.test", fdns.Records{TXT: []string{"v=spf1 include:_spf.google.com include:spf.relay.test -all"}})

		got := check(t, relaySpec(), nil, zone)

		assert.Equal(t, dns.StatusVerified, statusOf(got, dns.PurposeSPF).Status)
		assert.Empty(t, got.Warnings)
	})

	t.Run("does not take a name that points at itself for an alias", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().Set("t1._domainkey.acme.test", fdns.Records{TXT: []string{"v=DKIM1"}})

		got := check(t, sesSpec(), nil, zone)

		assert.Equal(t, dns.StatusMissing, statusOf(got, dns.PurposeDKIM).Status)
	})
}

func TestMistakes(t *testing.T) {
	t.Parallel()

	t.Run("finds a record with the domain typed twice", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().
			Set("_dimmi-verification.acme.test.acme.test", fdns.Records{TXT: []string{"dimmi-verification=tok"}}).
			Set("t1._domainkey.acme.test.acme.test", fdns.Records{CNAME: "t1.dkim.amazonses.com"}).
			Set("t1.dkim.amazonses.com", target()).
			Set("bounce.acme.test.acme.test", fdns.Records{MX: []fdns.MX{{Host: "x", Pref: 1}}})

		got := check(t, sesSpec(), nil, zone)

		doubled := slices.DeleteFunc(slices.Clone(got.Warnings), func(w dns.Warning) bool {
			return w.Code != dns.WarningDoubledDomain
		})
		require.Len(t, doubled, 3)
		assert.Equal(t, "_dimmi-verification.acme.test.acme.test", doubled[0].Value)
	})

	t.Run("finds a signing alias proxied by cloudflare", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().Set("t1._domainkey.acme.test", fdns.Records{
			Addrs: []netip.Addr{netip.MustParseAddr("104.16.1.1")},
		})

		assert.Contains(t, codes(check(t, sesSpec(), nil, zone)), dns.WarningCloudflareProxied)
	})

	t.Run("finds text where an alias belongs and an alias where text belongs", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().
			Set("t1._domainkey.acme.test", fdns.Records{TXT: []string{"v=DKIM1; p=x"}}).
			Set("dimmi202611._domainkey.acme.test", fdns.Records{CNAME: "elsewhere.test"}).
			Set("elsewhere.test", target())

		assert.Contains(t, codes(check(t, sesSpec(), nil, zone)), dns.WarningWrongRecordType)
		assert.Contains(t, codes(check(t, relaySpec(), nil, zone)), dns.WarningWrongRecordType)
	})

	t.Run("finds an alias on the ownership name", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().Set("_dimmi-verification.acme.test", fdns.Records{CNAME: "other.test"}).
			Set("other.test", target())

		assert.Contains(t, codes(check(t, sesSpec(), nil, zone)), dns.WarningCNAMEWithOthers)
	})

	t.Run("finds the token of another workspace", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().Set("_dimmi-verification.acme.test", fdns.Records{
			TXT: []string{"dimmi-verification=theirs"},
		})

		assert.Contains(t, codes(check(t, sesSpec(), []string{"theirs"}, zone)), dns.WarningOtherToken)
		assert.NotContains(t, codes(check(t, sesSpec(), nil, zone)), dns.WarningOtherToken)
	})

	t.Run("offers another return path name when one has its own mail server", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().Set("bounce.acme.test", fdns.Records{MX: []fdns.MX{{Host: "mx.acme.test", Pref: 5}}})

		got := check(t, sesSpec(), nil, zone)

		index := slices.IndexFunc(got.Warnings, func(w dns.Warning) bool { return w.Code == dns.WarningMXConflict })
		require.GreaterOrEqual(t, index, 0)
		assert.Equal(t, "dimmi-bounce", got.Warnings[index].Value)
	})

	t.Run("merges two spf records into one", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().Set("acme.test", fdns.Records{TXT: []string{"v=spf1 mx ~all", "v=spf1 a ~all"}})

		got := check(t, relaySpec(), nil, zone)

		index := slices.IndexFunc(got.Warnings, func(w dns.Warning) bool { return w.Code == dns.WarningDuplicateSPF })
		require.GreaterOrEqual(t, index, 0)
		assert.Equal(t, "v=spf1 mx include:spf.relay.test ~all", got.Warnings[index].Value)
		assert.Contains(t, codes(check(t, sesSpec(), nil, zone)), dns.WarningDuplicateSPF)
	})

	t.Run("asks for the relay include in the spf record", func(t *testing.T) {
		t.Parallel()
		withSPF := fdns.NewFake().Set("acme.test", fdns.Records{TXT: []string{"v=spf1 mx ~all"}})

		assert.Contains(t, codes(check(t, relaySpec(), nil, withSPF)), dns.WarningSPFMissingInclude)
		assert.Contains(t, codes(check(t, relaySpec(), nil, fdns.NewFake())), dns.WarningSPFMissingInclude)
		assert.NotContains(t, codes(check(t, sesSpec(), nil, fdns.NewFake())), dns.WarningSPFMissingInclude)
	})

	t.Run("counts spf lookups through includes", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().
			Set("acme.test", fdns.Records{TXT: []string{"v=spf1 include:a.test include:spf.relay.test mx a ~all"}}).
			Set("a.test", fdns.Records{TXT: []string{"v=spf1 include:b.test a mx ptr exists:x a:y mx:z ~all"}}).
			Set("b.test", fdns.Records{TXT: []string{"v=spf1 include:a.test redirect=c.test"}})

		assert.Contains(t, codes(check(t, relaySpec(), nil, zone)), dns.WarningSPFTooManyLookups)
	})

	t.Run("reads the dmarc policy", func(t *testing.T) {
		t.Parallel()
		none := fdns.NewFake()
		twice := []string{"v=DMARC1; p=none", "v=DMARC1; p=none"}
		two := fdns.NewFake().Set("_dmarc.acme.test", fdns.Records{TXT: twice})
		other := fdns.NewFake().Set("_dmarc.acme.test", fdns.Records{TXT: []string{"something else"}})
		reject := fdns.NewFake().Set("_dmarc.acme.test", fdns.Records{TXT: []string{"v=DMARC1; p=reject"}})

		assert.Contains(t, codes(check(t, sesSpec(), nil, none)), dns.WarningDMARCMissing)
		assert.Contains(t, codes(check(t, sesSpec(), nil, other)), dns.WarningDMARCMissing)
		assert.Contains(t, codes(check(t, sesSpec(), nil, two)), dns.WarningDMARCMultiple)
		assert.Contains(t, codes(check(t, sesSpec(), nil, reject)), dns.WarningDMARCReject)
		assert.NotContains(t, codes(check(t, sesSpec(), nil, broken{})), dns.WarningDMARCMissing)
	})

	t.Run("finds nothing without a resolver to ask", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, check(t, sesSpec(), nil).Warnings)
	})

	t.Run("lists a code for every mistake", func(t *testing.T) {
		t.Parallel()

		assert.Len(t, dns.WarningCodes(), 13)
	})
}
