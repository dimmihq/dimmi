package jwt_test

import (
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/jwt"
)

/*
The example JWT of RFC 7515 appendix A.1, signed with HS256.
*/
const (
	rfcToken = "eyJ0eXAiOiJKV1QiLA0KICJhbGciOiJIUzI1NiJ9." +
		"eyJpc3MiOiJqb2UiLA0KICJleHAiOjEzMDA4MTkzODAsDQogImh0dHA6Ly9leGFtcGxlLmNvbS9pc19yb290Ijp0cnVlfQ." +
		"dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	rfcKey = "AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"
)

func TestVerify(t *testing.T) {
	t.Parallel()

	t.Run("accepts the rfc 7515 example token", func(t *testing.T) {
		t.Parallel()
		raw, err := base64.RawURLEncoding.DecodeString(rfcKey)
		require.NoError(t, err)
		key, err := jwt.NewKey("", jwt.HS256, raw)
		require.NoError(t, err)
		opts := jwt.Options{Algorithms: []jwt.Algorithm{jwt.HS256}, Clock: clock.NewFake(time.Unix(1300819380-30, 0))}

		claims, err := jwt.Verify(rfcToken, []jwt.Key{key}, opts)

		require.NoError(t, err)
		assert.Equal(t, "joe", claims.Issuer)
		assert.Equal(t, time.Unix(1300819380, 0), claims.Expiry)
		assert.Equal(t, map[string]any{"http://example.com/is_root": true}, claims.Extra)
	})

	t.Run("round trips every algorithm", func(t *testing.T) {
		t.Parallel()
		materials := map[jwt.Algorithm]any{
			jwt.HS256: secret(),
			jwt.RS256: rsaKey(t, 2048),
			jwt.ES256: ecKey(t, elliptic.P256()),
			jwt.EdDSA: edKey(t),
		}
		sent := valid()
		sent.Issuer, sent.ID, sent.Audience = "https://idp.example.com", "jti-1", []string{"app"}
		sent.NotBefore = testNow().Add(-time.Minute)
		sent.Extra = map[string]any{"email": "a@example.com", "sub": "ignored"}

		for alg, material := range materials {
			key, err := jwt.NewKey("k-"+string(alg), alg, material)
			require.NoError(t, err)
			token, err := jwt.Sign(sent, key)
			require.NoError(t, err)

			opts := options()
			opts.Algorithms, opts.Audience, opts.Issuer = []jwt.Algorithm{alg}, "app", sent.Issuer
			got, err := jwt.Verify(token, []jwt.Key{hmacKey(t, "other"), key}, opts)
			require.NoError(t, err, alg)

			assert.Equal(t, "user-42", got.Subject)
			assert.Equal(t, sent.Audience, got.Audience)
			assert.Equal(t, "jti-1", got.ID)
			assert.True(t, sent.Expiry.Equal(got.Expiry))
			assert.True(t, sent.NotBefore.Equal(got.NotBefore))
			assert.Equal(t, map[string]any{"email": "a@example.com"}, got.Extra)
		}
	})

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		t.Parallel()
		token, err := jwt.Sign(valid(), hmacKey(t, ""))
		require.NoError(t, err)

		_, err = jwt.Verify("  "+token+"\n", []jwt.Key{hmacKey(t, "")}, options())

		require.NoError(t, err)
	})
}

func TestVerifyAlgorithms(t *testing.T) {
	t.Parallel()

	t.Run("rejects alg none before looking at keys", func(t *testing.T) {
		t.Parallel()
		token := encode(`{"alg":"none"}`) + "." + encode(`{"sub":"x","exp":1793530800}`) + "."

		opts := options()
		opts.Algorithms = append(opts.Algorithms, "none")
		_, err := jwt.Verify(token, nil, opts)

		require.ErrorIs(t, err, jwt.ErrAlgNotAllowed)
	})

	t.Run("rejects an algorithm the caller did not allow", func(t *testing.T) {
		t.Parallel()
		token, err := jwt.Sign(valid(), hmacKey(t, ""))
		require.NoError(t, err)

		opts := options()
		opts.Algorithms = []jwt.Algorithm{jwt.RS256}
		_, err = jwt.Verify(token, []jwt.Key{hmacKey(t, "")}, opts)

		require.ErrorIs(t, err, jwt.ErrAlgNotAllowed)
	})

	t.Run("refuses an hmac token signed with an rsa public key", func(t *testing.T) {
		t.Parallel()
		private := rsaKey(t, 2048)
		der, err := x509.MarshalPKIXPublicKey(&private.PublicKey)
		require.NoError(t, err)
		confused, err := jwt.NewKey("k", jwt.HS256, der)
		require.NoError(t, err)
		token, err := jwt.Sign(valid(), confused)
		require.NoError(t, err)
		pinned, err := jwt.NewKey("k", jwt.RS256, &private.PublicKey)
		require.NoError(t, err)

		opts := options()
		opts.Algorithms = []jwt.Algorithm{jwt.HS256, jwt.RS256}
		_, err = jwt.Verify(token, []jwt.Key{pinned}, opts)

		require.ErrorIs(t, err, jwt.ErrAlgMismatch)
	})

	t.Run("finds keys only by kid or as the single key", func(t *testing.T) {
		t.Parallel()
		named, err := jwt.Sign(valid(), hmacKey(t, "a"))
		require.NoError(t, err)
		unnamed, err := jwt.Sign(valid(), hmacKey(t, ""))
		require.NoError(t, err)

		_, err = jwt.Verify(named, []jwt.Key{hmacKey(t, "b")}, options())
		require.ErrorIs(t, err, jwt.ErrUnknownKey)
		_, err = jwt.Verify(unnamed, []jwt.Key{hmacKey(t, "a"), hmacKey(t, "b")}, options())
		require.ErrorIs(t, err, jwt.ErrUnknownKey)
		_, err = jwt.Verify(unnamed, []jwt.Key{hmacKey(t, "a")}, options())
		require.NoError(t, err)
	})

	t.Run("rejects a wrong signature", func(t *testing.T) {
		t.Parallel()
		token, err := jwt.Sign(valid(), hmacKey(t, ""))
		require.NoError(t, err)
		other, err := jwt.NewKey("", jwt.HS256, []byte(strings.Repeat("x", 32)))
		require.NoError(t, err)

		_, err = jwt.Verify(token, []jwt.Key{other}, options())

		require.ErrorIs(t, err, jwt.ErrSignatureInvalid)
	})

	t.Run("ignores headers that point at other keys", func(t *testing.T) {
		t.Parallel()
		token := forge(`{"alg":"HS256","jku":"https://evil.example.com/jwks","x5u":"https://evil.example.com"}`,
			payload(t, valid()))

		_, err := jwt.Verify(token, []jwt.Key{hmacKey(t, "")}, options())

		require.NoError(t, err)
	})
}

func TestVerifyFormat(t *testing.T) {
	t.Parallel()
	good := payload(t, valid())

	malformed := map[string]string{
		"crit header":           forge(`{"alg":"HS256","crit":["exp"]}`, good),
		"duplicate payload key": forge(`{"alg":"HS256"}`, `{"sub":"a","sub":"b","exp":1793530800}`),
		"duplicate header key":  forge(`{"alg":"HS256","alg":"HS256"}`, good),
		"duplicate nested key":  forge(`{"alg":"HS256"}`, `{"exp":1793530800,"a":[{"b":1,"b":2}]}`),
		"payload array":         forge(`{"alg":"HS256"}`, `[1]`),
		"payload string":        forge(`{"alg":"HS256"}`, `"sub"`),
		"trailing data":         forge(`{"alg":"HS256"}`, good+`{}`),
		"truncated json":        forge(`{"alg":"HS256"}`, `{"exp":1793530800`),
		"invalid json":          forge(`{"alg":"HS256"}`, `{"exp":1793530800,}`),
		"array value invalid":   forge(`{"alg":"HS256"}`, `{"a":[1,}`),
		"non-string key":        forge(`{"alg":"HS256"}`, `{1:2}`),
		"deep nesting":          forge(`{"alg":"HS256"}`, `{"a":`+strings.Repeat("[", 40)+strings.Repeat("]", 40)+`}`),
		"deep object nesting":   forge(`{"alg":"HS256"}`, strings.Repeat(`{"a":`, 40)+"1"+strings.Repeat("}", 40)),
		"exp as string":         forge(`{"alg":"HS256"}`, `{"exp":"1793530800"}`),
		"alg as number":         forge(`{"alg":256}`, good),
		"unexpected typ":        forge(`{"alg":"HS256","typ":"at+jwt"}`, good),
		"two parts":             encode(`{"alg":"HS256"}`) + "." + encode(good),
		"padded base64":         forge(`{"alg":"HS256"}`, good) + "=",
		"standard base64":       strings.Replace(forge(`{"alg":"HS256"}`, `{"exp":1793530800,"n":"~~~"}`), "_", "/", 1),
		"bearer prefix":         "Bearer " + forge(`{"alg":"HS256"}`, good),
		"empty":                 "",
	}

	for name, token := range malformed {
		t.Run("rejects "+name, func(t *testing.T) {
			t.Parallel()

			_, err := jwt.Verify(token, []jwt.Key{hmacKey(t, "")}, options())

			require.ErrorIs(t, err, jwt.ErrMalformed)
		})
	}

	t.Run("rejects a token over the size limit", func(t *testing.T) {
		t.Parallel()
		claims := valid()
		claims.Extra = map[string]any{"blob": strings.Repeat("x", 9<<10)}
		token, err := jwt.Sign(claims, hmacKey(t, ""))
		require.NoError(t, err)

		_, err = jwt.Verify(token, []jwt.Key{hmacKey(t, "")}, options())

		require.ErrorIs(t, err, jwt.ErrTooLarge)
	})

	t.Run("applies the typ rule", func(t *testing.T) {
		t.Parallel()
		keys := []jwt.Key{hmacKey(t, "")}
		typed := options()
		typed.Type = "at+jwt"

		for token, opts := range map[string]jwt.Options{
			forge(`{"alg":"HS256","typ":"JWT"}`, good):                options(),
			forge(`{"alg":"HS256","typ":"application/jwt"}`, good):    options(),
			forge(`{"alg":"HS256"}`, good):                            options(),
			forge(`{"alg":"HS256","typ":"application/at+jwt"}`, good): typed,
		} {
			_, err := jwt.Verify(token, keys, opts)
			require.NoError(t, err, token)
		}

		_, err := jwt.Verify(forge(`{"alg":"HS256"}`, good), keys, typed)
		require.ErrorIs(t, err, jwt.ErrMalformed)
	})
}

func TestVerifyClaims(t *testing.T) {
	t.Parallel()
	at := testNow()

	cases := map[string]struct {
		claims jwt.Claims
		opts   func(*jwt.Options)
		err    error
	}{
		"missing exp": {claims: jwt.Claims{Subject: "x"}, err: jwt.ErrMissingExpiry},
		"expired":     {claims: jwt.Claims{Expiry: at.Add(-2 * time.Minute)}, err: jwt.ErrExpired},
		"issued in the future": {
			claims: jwt.Claims{IssuedAt: at.Add(2 * time.Minute), Expiry: at.Add(5 * time.Minute)},
			err:    jwt.ErrNotYetValid,
		},
		"not valid before": {
			claims: jwt.Claims{NotBefore: at.Add(2 * time.Minute), Expiry: at.Add(5 * time.Minute)},
			err:    jwt.ErrNotYetValid,
		},
		"exp in milliseconds": {
			claims: jwt.Claims{Expiry: time.Unix(at.Unix()*1000, 0)},
			err:    jwt.ErrLifetimeTooLong,
		},
		"thirty days": {
			claims: jwt.Claims{Expiry: at.Add(30 * 24 * time.Hour)},
			err:    jwt.ErrLifetimeTooLong,
		},
		"lifetime from iat": {
			claims: jwt.Claims{IssuedAt: at.Add(-23 * time.Hour), Expiry: at.Add(2 * time.Hour)},
			err:    jwt.ErrLifetimeTooLong,
		},
		"wrong audience": {
			claims: valid(),
			opts:   func(o *jwt.Options) { o.Audience = "app" },
			err:    jwt.ErrAudience,
		},
		"wrong issuer": {
			claims: valid(),
			opts:   func(o *jwt.Options) { o.Issuer = "idp" },
			err:    jwt.ErrIssuer,
		},
		"expired within leeway": {claims: jwt.Claims{Expiry: at.Add(-30 * time.Second)}},
		"custom lifetime": {
			claims: jwt.Claims{Expiry: at.Add(2 * time.Hour)},
			opts:   func(o *jwt.Options) { o.MaxLifetime = time.Hour },
			err:    jwt.ErrLifetimeTooLong,
		},
		"tiny leeway": {
			claims: jwt.Claims{Expiry: at.Add(-30 * time.Second)},
			opts:   func(o *jwt.Options) { o.Leeway = time.Nanosecond },
			err:    jwt.ErrExpired,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			token, err := jwt.Sign(tc.claims, hmacKey(t, ""))
			require.NoError(t, err)
			opts := options()
			if tc.opts != nil {
				tc.opts(&opts)
			}

			_, err = jwt.Verify(token, []jwt.Key{hmacKey(t, "")}, opts)

			if tc.err == nil {
				require.NoError(t, err)

				return
			}
			require.ErrorIs(t, err, tc.err)
		})
	}

	t.Run("uses the system clock by default", func(t *testing.T) {
		t.Parallel()
		token, err := jwt.Sign(jwt.Claims{Expiry: time.Unix(1300819380, 0)}, hmacKey(t, ""))
		require.NoError(t, err)

		_, err = jwt.Verify(token, []jwt.Key{hmacKey(t, "")}, jwt.Options{Algorithms: []jwt.Algorithm{jwt.HS256}})

		require.ErrorIs(t, err, jwt.ErrExpired)
	})
}

/*
payload returns the claims as the JSON Sign would put in a token.
*/
func payload(t *testing.T, claims jwt.Claims) string {
	t.Helper()

	data, err := json.Marshal(map[string]any{
		"sub": claims.Subject, "iat": claims.IssuedAt.Unix(), "exp": claims.Expiry.Unix(),
	})
	require.NoError(t, err)

	return string(data)
}
