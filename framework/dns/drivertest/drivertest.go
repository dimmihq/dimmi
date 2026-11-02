/*
|--------------------------------------------------------------------------
| Resolver Contract
|--------------------------------------------------------------------------
|
| Code tested against the fake zone must behave the same against real
| servers. Each case describes a zone; the constructor returns a resolver
| that answers from it, whether by serving it from a real DNS server or
| by loading it into the fake.
|
|   func TestResolver(t *testing.T) {
|       drivertest.Run(t, newResolver)
|   }
|
*/

package drivertest

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
)

/*
NewResolver returns a resolver answering from the zone, keyed by name.
*/
type NewResolver func(t *testing.T, zone map[string]dns.Records) dns.Resolver

/*
Zone is the zone every case reads. The long TXT record exceeds the 255
bytes one DNS string can hold, so servers split it and the resolver must
join it again.
*/
func Zone() map[string]dns.Records {
	return map[string]dns.Records{
		"example.test": {
			TXT:   []string{"v=spf1 -all", "token=" + strings.Repeat("a", 300)},
			MX:    []dns.MX{{Host: "backup.example.test", Pref: 20}, {Host: "mail.example.test", Pref: 10}},
			Addrs: []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1")},
		},
		"www.example.test":  {CNAME: "example.test"},
		"text.example.test": {TXT: []string{"only text"}},
	}
}

/*
Run runs the contract against resolvers from the constructor.
*/
func Run(t *testing.T, newResolver NewResolver) {
	t.Helper()

	t.Run("records", func(t *testing.T) { t.Parallel(); runRecords(t, newResolver(t, Zone())) })
	t.Run("aliases", func(t *testing.T) { t.Parallel(); runAliases(t, newResolver(t, Zone())) })
	t.Run("absent", func(t *testing.T) { t.Parallel(); runAbsent(t, newResolver(t, Zone())) })
}

func runRecords(t *testing.T, r dns.Resolver) {
	t.Helper()

	t.Run("returns every txt record joined whole", func(t *testing.T) {
		t.Parallel()

		got, err := r.TXT(t.Context(), "example.test")

		require.NoError(t, err)
		assert.ElementsMatch(t, Zone()["example.test"].TXT, got)
	})

	t.Run("returns mail exchangers lowest preference first", func(t *testing.T) {
		t.Parallel()

		got, err := r.MX(t.Context(), "example.test")

		require.NoError(t, err)
		assert.Equal(t, []dns.MX{{Host: "mail.example.test", Pref: 10}, {Host: "backup.example.test", Pref: 20}}, got)
	})

	t.Run("returns both address families", func(t *testing.T) {
		t.Parallel()

		got, err := r.Addrs(t.Context(), "example.test")

		require.NoError(t, err)
		assert.ElementsMatch(t, Zone()["example.test"].Addrs, got)
	})

	t.Run("matches names regardless of case and root dot", func(t *testing.T) {
		t.Parallel()

		got, err := r.TXT(t.Context(), "Text.Example.TEST.")

		require.NoError(t, err)
		assert.Equal(t, []string{"only text"}, got)
	})
}

func runAliases(t *testing.T, r dns.Resolver) {
	t.Helper()

	t.Run("returns the target of an alias", func(t *testing.T) {
		t.Parallel()

		got, err := r.CNAME(t.Context(), "www.example.test")

		require.NoError(t, err)
		assert.Equal(t, "example.test", got)
	})

	t.Run("returns a name that is not an alias as its own canonical name", func(t *testing.T) {
		t.Parallel()

		got, err := r.CNAME(t.Context(), "example.test")

		require.NoError(t, err)
		assert.Equal(t, "example.test", got)
	})

	t.Run("follows an alias to its addresses", func(t *testing.T) {
		t.Parallel()

		got, err := r.Addrs(t.Context(), "www.example.test")

		require.NoError(t, err)
		assert.ElementsMatch(t, Zone()["example.test"].Addrs, got)
	})
}

func runAbsent(t *testing.T, r dns.Resolver) {
	t.Helper()

	t.Run("reports a name that does not exist", func(t *testing.T) {
		t.Parallel()

		_, err := r.TXT(t.Context(), "missing.example.test")
		require.ErrorIs(t, err, dns.ErrNotFound)
		_, err = r.MX(t.Context(), "missing.example.test")
		require.ErrorIs(t, err, dns.ErrNotFound)
		_, err = r.CNAME(t.Context(), "missing.example.test")
		require.ErrorIs(t, err, dns.ErrNotFound)
		_, err = r.Addrs(t.Context(), "missing.example.test")
		require.ErrorIs(t, err, dns.ErrNotFound)
	})

	t.Run("reports a record type the name does not have", func(t *testing.T) {
		t.Parallel()

		_, err := r.MX(t.Context(), "text.example.test")
		require.ErrorIs(t, err, dns.ErrNotFound)
		_, err = r.Addrs(t.Context(), "text.example.test")
		require.ErrorIs(t, err, dns.ErrNotFound)
	})
}
