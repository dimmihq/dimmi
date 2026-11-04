package foundation_test

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/foundation"
)

/*
fixedTLS is a custom domain driver serving one self-signed certificate
and answering one made-up challenge path, standing in for an ACME
driver.
*/
type fixedTLS struct {
	cert tls.Certificate
}

func (fixedTLS) Ensure(context.Context, string) error { return nil }
func (fixedTLS) Remove(context.Context, string) error { return nil }

func (d fixedTLS) TLSConfig() *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{d.cert}, MinVersion: tls.VersionTLS12}
}

func (fixedTLS) HTTPHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/acme-challenge/token" {
			_, _ = w.Write([]byte("answer"))

			return
		}

		next.ServeHTTP(w, r)
	})
}

/*
selfSigned returns a certificate for localhost and a pool trusting it.
*/
func selfSigned(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

/*
tlsOptions are test options serving HTTPS with the fixed driver.
*/
func tlsOptions(t *testing.T, cert tls.Certificate, extra ...string) foundation.Options {
	t.Helper()

	env := environ(t, append([]string{
		"APP_HTTP__TLS_ADDR=127.0.0.1:0",
		"APP_CUSTOM_DOMAINS__DEFAULT=fixed",
		"APP_CUSTOM_DOMAINS__TLS__FIXED__DRIVER=fixed",
	}, extra...)...)

	return options(env, provider{register: func(a *foundation.Application) error {
		a.Domains.Extend("fixed", func(string, config.Section) (customdomain.TLS, error) {
			return fixedTLS{cert: cert}, nil
		})

		return nil
	}})
}

/*
rawGet sends one GET over the connection with the Host header given and
returns the status and the Location header of the answer. It speaks
HTTP/1.1 by hand so the test can choose the Host and see redirects.
*/
func rawGet(t *testing.T, conn net.Conn, host, path string) (status int, location string) {
	t.Helper()

	header := rawHeaders(t, conn, host, path)

	return header.status, header.values.Get("Location")
}

/*
answer is the status and headers of a response read by hand.
*/
type answer struct {
	status int
	values http.Header
}

/*
rawHeaders sends one GET over the connection and returns the answer.
*/
func rawHeaders(t *testing.T, conn net.Conn, host, path string) answer {
	t.Helper()
	t.Cleanup(func() { _ = conn.Close() })

	_, err := fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", path, host)
	require.NoError(t, err)

	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	return answer{status: response.StatusCode, values: response.Header}
}

/*
plainGet requests the path over plain HTTP from the address.
*/
func plainGet(t *testing.T, addr, host, path string) (status int, location string) {
	t.Helper()

	conn, err := new(net.Dialer).DialContext(t.Context(), "tcp", addr)
	require.NoError(t, err)

	return rawGet(t, conn, host, path)
}

/*
dialTLS completes a handshake with the address, trusting only the pool
and offering the protocols.
*/
func dialTLS(t *testing.T, addr string, pool *x509.CertPool, protocols ...string) (*tls.Conn, error) {
	t.Helper()

	dialer := &tls.Dialer{Config: &tls.Config{
		RootCAs:    pool,
		ServerName: "localhost",
		NextProtos: protocols,
		MinVersion: tls.VersionTLS12,
	}}
	conn, err := dialer.DialContext(t.Context(), "tcp", addr)
	if err != nil {
		return nil, err
	}

	secure, ok := conn.(*tls.Conn)
	require.True(t, ok)

	return secure, nil
}

func TestServeTLS(t *testing.T) {
	t.Parallel()

	cert, pool := selfSigned(t)

	t.Run("serves the application over verified https and offers http/2", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, tlsOptions(t, cert))
		require.NoError(t, app.Start(t.Context()))

		h2, err := dialTLS(t, app.TLSAddr(), pool, "h2", "http/1.1")
		require.NoError(t, err)
		assert.Equal(t, "h2", h2.ConnectionState().NegotiatedProtocol)
		require.NoError(t, h2.Close())

		conn, err := dialTLS(t, app.TLSAddr(), pool, "http/1.1")
		require.NoError(t, err)
		got := rawHeaders(t, conn, "localhost", foundation.PathReady)
		assert.Equal(t, http.StatusOK, got.status)
		assert.Equal(t, "max-age=31536000", got.values.Get("Strict-Transport-Security"))
	})

	t.Run("plain http answers health and challenges and redirects the rest", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, tlsOptions(t, cert))
		require.NoError(t, app.Start(t.Context()))

		for _, path := range []string{foundation.PathUp, foundation.PathReady, "/.well-known/acme-challenge/token"} {
			code, location := plainGet(t, app.Addr(), "staging.example.com", path)
			assert.Equal(t, http.StatusOK, code, path)
			assert.Empty(t, location, path)
		}

		conn, err := new(net.Dialer).DialContext(t.Context(), "tcp", app.Addr())
		require.NoError(t, err)
		assert.Empty(t, rawHeaders(t, conn, "localhost", foundation.PathUp).values.Get("Strict-Transport-Security"))

		code, location := plainGet(t, app.Addr(), "Staging.Example.com:8080", "/d/orders?tab=open")
		assert.Equal(t, http.StatusPermanentRedirect, code)
		assert.Equal(t, "https://staging.example.com/d/orders?tab=open", location)

		code, location = plainGet(t, app.Addr(), "bad host!", "/")
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Empty(t, location)
	})

	t.Run("plain mode stays the same when no https address is set", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t)))
		require.NoError(t, app.Start(t.Context()))

		code, location := plainGet(t, app.Addr(), "localhost", "/nothing-here")
		assert.Empty(t, app.TLSAddr())
		assert.Equal(t, http.StatusNotFound, code)
		assert.Empty(t, location)
	})

	t.Run("refuses an https address without a driver that terminates tls", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, "APP_HTTP__TLS_ADDR=127.0.0.1:0")))

		require.ErrorIs(t, app.Start(t.Context()), foundation.ErrNoTLSDriver)
	})

	t.Run("shuts both listeners down and stops accepting", func(t *testing.T) {
		t.Parallel()
		app, err := foundation.New(t.Context(), tlsOptions(t, cert))
		require.NoError(t, err)
		require.NoError(t, app.Start(t.Context()))
		addr := app.TLSAddr()

		require.NoError(t, app.Shutdown(context.WithoutCancel(t.Context())))

		_, err = dialTLS(t, addr, pool, "http/1.1")
		require.Error(t, err)
	})
}

func TestDomainAuthorizer(t *testing.T) {
	t.Parallel()

	t.Run("the host of app.url always gets a certificate and others are refused", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, "APP_APP__URL=https://Staging.Example.com")))

		require.NoError(t, app.Domains.Ensure(t.Context(), "staging.example.com"))
		require.ErrorIs(t, app.Domains.Ensure(t.Context(), "other.example.com"), customdomain.ErrNotAuthorized)
	})

	t.Run("a plain http app.url gets no certificate", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, "APP_APP__URL=http://localhost:8080")))

		require.ErrorIs(t, app.Domains.Ensure(t.Context(), "localhost"), customdomain.ErrNotAuthorized)
	})

	t.Run("other hosts are up to the application's authorizer", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t, "APP_APP__URL=https://staging.example.com"))
		opts.Domains = func(_ context.Context, host string) error {
			if host == "help.customer.example" {
				return nil
			}

			return customdomain.ErrNotAuthorized
		}
		app := newApp(t, opts)

		require.NoError(t, app.Domains.Ensure(t.Context(), "staging.example.com"))
		require.NoError(t, app.Domains.Ensure(t.Context(), "help.customer.example"))
		require.ErrorIs(t, app.Domains.Ensure(t.Context(), "other.example.com"), customdomain.ErrNotAuthorized)
	})
}
