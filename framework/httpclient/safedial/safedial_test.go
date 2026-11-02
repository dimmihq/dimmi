package safedial_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
)

/*
listen accepts connections on a loopback port until the test ends and
returns its host:port.
*/
func listen(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			_ = conn.Close()
		}
	}()

	return ln.Addr().String()
}

func newDialer(t *testing.T, opts safedial.Options) *safedial.Dialer {
	t.Helper()

	d, err := safedial.New(opts)
	require.NoError(t, err)

	return d
}

func TestDialer(t *testing.T) {
	t.Parallel()

	t.Run("refuses loopback by default", func(t *testing.T) {
		t.Parallel()
		addr := listen(t)

		_, err := newDialer(t, safedial.Options{}).DialContext(t.Context(), "tcp", addr)

		require.ErrorIs(t, err, safedial.ErrBlocked)
	})

	t.Run("checks the address a name resolves to", func(t *testing.T) {
		t.Parallel()
		_, port, _ := net.SplitHostPort(listen(t))

		_, err := newDialer(t, safedial.Options{}).DialContext(t.Context(), "tcp", net.JoinHostPort("localhost", port))

		require.ErrorIs(t, err, safedial.ErrBlocked)
	})

	t.Run("dials a host allowed by name", func(t *testing.T) {
		t.Parallel()
		addr := listen(t)
		d := newDialer(t, safedial.Options{AllowHosts: []string{addr}})

		conn, err := d.DialContext(t.Context(), "tcp", addr)

		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})

	t.Run("dials private addresses when private networks are allowed", func(t *testing.T) {
		t.Parallel()
		addr := listen(t)
		d := newDialer(t, safedial.Options{AllowPrivateNetworks: true})

		conn, err := d.DialContext(t.Context(), "tcp", addr)

		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})

	t.Run("refuses cloud metadata even with private networks allowed", func(t *testing.T) {
		t.Parallel()
		d := newDialer(t, safedial.Options{AllowPrivateNetworks: true})

		_, err := d.DialContext(t.Context(), "tcp", "169.254.169.254:80")

		require.ErrorIs(t, err, safedial.ErrBlocked)
		require.ErrorIs(t, d.Check("[fd00:ec2::254]:80"), safedial.ErrBlocked)
	})

	t.Run("rejects an allowed host without a port", func(t *testing.T) {
		t.Parallel()

		_, err := safedial.New(safedial.Options{AllowHosts: []string{"localhost"}})

		require.ErrorIs(t, err, safedial.ErrInvalidHost)
	})
}

func TestCheck(t *testing.T) {
	t.Parallel()

	d := newDialer(t, safedial.Options{AllowHosts: []string{"LocalHost:11434"}})

	cases := map[string]bool{
		"127.0.0.1:80":       false,
		"localhost:8080":     false,
		"10.1.2.3:443":       false,
		"[::1]:443":          false,
		"[fd12::1]:443":      false,
		"100.64.0.1:80":      false,
		"8.8.8.8:443":        true,
		"[2606:4700::1]:443": true,
		"example.com:443":    true,
		"localhost:11434":    true,
		"not an address":     true,
	}

	for address, allowed := range cases {
		t.Run("judges "+address, func(t *testing.T) {
			t.Parallel()

			err := d.Check(address)

			if allowed {
				assert.NoError(t, err)

				return
			}

			assert.ErrorIs(t, err, safedial.ErrBlocked)
		})
	}
}
