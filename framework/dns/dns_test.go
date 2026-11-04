package dns_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/dns/drivertest"
)

func TestResolver(t *testing.T) {
	t.Parallel()

	t.Run("real server", func(t *testing.T) {
		t.Parallel()
		drivertest.Run(t, func(t *testing.T, zone map[string]dns.Records) dns.Resolver {
			t.Helper()

			return dns.New(dns.Config{Nameserver: serveZone(t, zone)})
		})
	})

	t.Run("fake", func(t *testing.T) {
		t.Parallel()
		drivertest.Run(t, func(_ *testing.T, zone map[string]dns.Records) dns.Resolver {
			fake := dns.NewFake()
			for name, records := range zone {
				fake.Set(name, records)
			}

			return fake
		})
	})
}

func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("gives up after the timeout", func(t *testing.T) {
		t.Parallel()
		silent := silentServer(t)
		client := dns.New(dns.Config{Nameserver: silent, Timeout: 50 * time.Millisecond})

		_, err := client.TXT(t.Context(), "example.test")

		require.Error(t, err)
		require.NotErrorIs(t, err, dns.ErrNotFound)
	})

	t.Run("refuses a fake unless fakes are allowed", func(t *testing.T) {
		t.Parallel()
		client := dns.New(dns.Config{})

		assert.PanicsWithValue(t, dns.ErrFakesNotAllowed, func() { client.Fake() })
	})

	t.Run("answers from the fake once faked", func(t *testing.T) {
		t.Parallel()
		client := dns.New(dns.Config{AllowFakes: true})
		addr := netip.MustParseAddr("192.0.2.7")
		records := dns.Records{TXT: []string{"t"}, MX: []dns.MX{{Host: "mx.example.test"}}, Addrs: []netip.Addr{addr}}
		client.Fake().
			Set("example.test", records).
			Set("alias.example.test", dns.Records{CNAME: "example.test"})

		txt, err := client.TXT(t.Context(), "example.test")
		require.NoError(t, err)
		mx, err := client.MX(t.Context(), "example.test")
		require.NoError(t, err)
		canonical, err := client.CNAME(t.Context(), "alias.example.test")
		require.NoError(t, err)
		addrs, err := client.Addrs(t.Context(), "alias.example.test")
		require.NoError(t, err)

		assert.Equal(t, []string{"t"}, txt)
		assert.Equal(t, []dns.MX{{Host: "mx.example.test"}}, mx)
		assert.Equal(t, "example.test", canonical)
		assert.Equal(t, []netip.Addr{addr}, addrs)
	})
}

func TestVia(t *testing.T) {
	t.Parallel()

	t.Run("asks the named server", func(t *testing.T) {
		t.Parallel()
		server := serveZone(t, map[string]dns.Records{"example.test": {TXT: []string{"from server"}}})
		via := dns.New(dns.Config{AllowFakes: true}).Via(server)

		txt, err := via.TXT(t.Context(), "example.test")

		require.NoError(t, err)
		assert.Equal(t, []string{"from server"}, txt)
	})

	t.Run("answers from the fake once the client is faked", func(t *testing.T) {
		t.Parallel()
		silent := silentServer(t)
		client := dns.New(dns.Config{AllowFakes: true, Timeout: 50 * time.Millisecond})
		via := client.Via(silent)
		addr := netip.MustParseAddr("192.0.2.9")
		client.Fake().Set("example.test", dns.Records{
			TXT: []string{"t"}, MX: []dns.MX{{Host: "mx.example.test"}}, Addrs: []netip.Addr{addr},
			NS: []string{"ns.example.test"},
		}).Set("alias.example.test", dns.Records{CNAME: "example.test"})

		txt, err := via.TXT(t.Context(), "example.test")
		require.NoError(t, err)
		mx, err := via.MX(t.Context(), "example.test")
		require.NoError(t, err)
		canonical, err := via.CNAME(t.Context(), "alias.example.test")
		require.NoError(t, err)
		addrs, err := via.Addrs(t.Context(), "example.test")
		require.NoError(t, err)
		ns, err := via.NS(t.Context(), "example.test")
		require.NoError(t, err)

		assert.Equal(t, []string{"t"}, txt)
		assert.Equal(t, []dns.MX{{Host: "mx.example.test"}}, mx)
		assert.Equal(t, "example.test", canonical)
		assert.Equal(t, []netip.Addr{addr}, addrs)
		assert.Equal(t, []string{"ns.example.test"}, ns)
	})
}

func TestFake(t *testing.T) {
	t.Parallel()

	t.Run("stops on a canceled context", func(t *testing.T) {
		t.Parallel()
		fake := dns.NewFake().Set("example.test", dns.Records{TXT: []string{"t"}})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := fake.TXT(ctx, "example.test")

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("ends an alias loop as not found", func(t *testing.T) {
		t.Parallel()
		fake := dns.NewFake().
			Set("a.example.test", dns.Records{CNAME: "b.example.test"}).
			Set("b.example.test", dns.Records{CNAME: "a.example.test"})

		_, err := fake.Addrs(t.Context(), "a.example.test")

		require.ErrorIs(t, err, dns.ErrNotFound)
	})
}
