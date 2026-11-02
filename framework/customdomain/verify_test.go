package customdomain_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/dns"
)

const (
	txtName  = "_verify.docs.example.com"
	txtValue = "verify=k3j9"
	target   = "domains.platform.test"
)

/*
expect is what a subdomain of the tests must show.
*/
func expect() customdomain.Expect {
	return customdomain.Expect{TXTName: txtName, TXTValue: txtValue, CNAME: target}
}

/*
brokenResolver fails every lookup the way a timed out server does.
*/
type brokenResolver struct{}

var errTimeout = errors.New("i/o timeout")

func (brokenResolver) TXT(context.Context, string) ([]string, error) {
	return nil, errTimeout
}

func (brokenResolver) MX(context.Context, string) ([]dns.MX, error) {
	return nil, errTimeout
}

func (brokenResolver) CNAME(context.Context, string) (string, error) {
	return "", errTimeout
}

func (brokenResolver) Addrs(context.Context, string) ([]netip.Addr, error) {
	return nil, errTimeout
}

func TestVerifier(t *testing.T) {
	t.Parallel()

	t.Run("verifies an owned subdomain pointing at the target", func(t *testing.T) {
		t.Parallel()
		zone := dns.NewFake().
			Set(txtName, dns.Records{TXT: []string{"other", txtValue}}).
			Set("docs.example.com", dns.Records{CNAME: target + "."}).
			Set(target, dns.Records{Addrs: []netip.Addr{netip.MustParseAddr("192.0.2.10")}})

		result, err := customdomain.NewVerifier(zone, localClient(t), probe()).
			Check(t.Context(), "docs.example.com", expect())

		require.NoError(t, err)
		assert.True(t, result.Verified())
		assert.False(t, result.Probed)
		assert.Empty(t, result.Findings)
	})

	t.Run("finds a missing ownership record and missing routing", func(t *testing.T) {
		t.Parallel()

		result, err := customdomain.NewVerifier(dns.NewFake(), localClient(t), probe()).
			Check(t.Context(), "docs.example.com", expect())

		require.NoError(t, err)
		assert.False(t, result.Verified())
		require.Len(t, result.Findings, 2)
		require.ErrorIs(t, result.Findings[0], customdomain.ErrOwnershipMissing)
		require.ErrorIs(t, result.Findings[1], customdomain.ErrRoutingMismatch)
	})

	t.Run("finds an ownership record holding the wrong value", func(t *testing.T) {
		t.Parallel()
		zone := dns.NewFake().
			Set(txtName, dns.Records{TXT: []string{"verify=old"}}).
			Set("docs.example.com", dns.Records{CNAME: target}).
			Set(target, dns.Records{Addrs: []netip.Addr{netip.MustParseAddr("192.0.2.10")}})

		result, err := customdomain.NewVerifier(zone, localClient(t), probe()).
			Check(t.Context(), "docs.example.com", expect())

		require.NoError(t, err)
		assert.False(t, result.Ownership)
		assert.True(t, result.Routing)
		require.ErrorIs(t, result.Findings[0], customdomain.ErrOwnershipMismatch)
	})

	t.Run("routes an apex by its addresses", func(t *testing.T) {
		t.Parallel()
		zone := dns.NewFake().Set("example.com", dns.Records{Addrs: []netip.Addr{
			netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("::ffff:192.0.2.11"),
		}})
		apex := customdomain.Expect{Addrs: []netip.Addr{
			netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("192.0.2.11"),
		}}

		result, err := customdomain.NewVerifier(zone, localClient(t), probe()).Check(t.Context(), "example.com", apex)

		require.NoError(t, err)
		assert.True(t, result.Verified())
	})

	t.Run("refuses an apex that also points somewhere else", func(t *testing.T) {
		t.Parallel()
		zone := dns.NewFake().Set("example.com", dns.Records{Addrs: []netip.Addr{
			netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("198.51.100.1"),
		}})
		apex := customdomain.Expect{Addrs: []netip.Addr{netip.MustParseAddr("192.0.2.10")}}

		result, err := customdomain.NewVerifier(zone, localClient(t), probe()).Check(t.Context(), "example.com", apex)

		require.NoError(t, err)
		assert.False(t, result.Routing)
	})

	t.Run("refuses an apex without addresses", func(t *testing.T) {
		t.Parallel()
		apex := customdomain.Expect{Addrs: []netip.Addr{netip.MustParseAddr("192.0.2.10")}}

		result, err := customdomain.NewVerifier(dns.NewFake(), localClient(t), probe()).
			Check(t.Context(), "example.com", apex)

		require.NoError(t, err)
		assert.False(t, result.Routing)
	})

	t.Run("accepts routing proved by the probe behind a proxy", func(t *testing.T) {
		t.Parallel()
		host := serveProbe(t, probe())
		zone := dns.NewFake().Set(txtName, dns.Records{TXT: []string{txtValue}})
		proxied := expect()
		proxied.ProbeToken = "tok3n"

		result, err := customdomain.NewVerifier(zone, localClient(t), probe()).Check(t.Context(), host, proxied)

		require.NoError(t, err)
		assert.True(t, result.Verified())
		assert.True(t, result.Probed)
	})

	t.Run("returns lookups that could not be answered as errors", func(t *testing.T) {
		t.Parallel()
		verifier := customdomain.NewVerifier(brokenResolver{}, localClient(t), probe())
		apex := customdomain.Expect{Addrs: []netip.Addr{netip.MustParseAddr("192.0.2.10")}}

		_, ownershipErr := verifier.Check(t.Context(), "docs.example.com", expect())
		_, cnameErr := verifier.Check(t.Context(), "docs.example.com", customdomain.Expect{CNAME: target})
		_, addrsErr := verifier.Check(t.Context(), "example.com", apex)

		require.ErrorIs(t, ownershipErr, errTimeout)
		require.ErrorIs(t, cnameErr, errTimeout)
		require.ErrorIs(t, addrsErr, errTimeout)
	})
}
