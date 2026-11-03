package twofactor_test

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func newEncrypter(t *testing.T) *encryption.AppEncrypter {
	t.Helper()

	key, err := encryption.ParseAppKey(encryption.GenerateAppKey())
	require.NoError(t, err)

	enc, err := encryption.NewEncrypter(key)
	require.NoError(t, err)

	return enc
}

func newCache(t *testing.T) (*cache.Store, *redis.Client) {
	t.Helper()

	client := testkit.NewValkey(t)

	store, err := cache.Open(client, cache.Options{Keyspace: "cache"})
	require.NoError(t, err)
	t.Cleanup(store.Close)

	return store, client
}

func newClock() *clock.Fake {
	return clock.NewFake(time.Date(2026, 11, 2, 9, 0, 10, 0, time.UTC))
}

/*
enroll returns a TOTP factor and a fresh credential with its base32
secret, as an authenticator app would hold it.
*/
func enroll(t *testing.T, store *cache.Store, clk clock.Clock) (*twofactor.TOTP, twofactor.Credential, string) {
	t.Helper()

	factor := twofactor.NewTOTP("Acme", newEncrypter(t), store, clk)

	sealed, err := factor.NewSecret()
	require.NoError(t, err)

	secret, _, err := factor.Setup(sealed, "ada@example.com")
	require.NoError(t, err)

	return factor, twofactor.Credential{ID: ids.New(), Type: twofactor.TOTPType, Secret: sealed}, secret
}

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()

	code, err := totp.GenerateCode(secret, at)
	require.NoError(t, err)

	return code
}

func TestTOTP(t *testing.T) {
	t.Parallel()

	store, _ := newCache(t)

	t.Run("sets up an authenticator app", func(t *testing.T) {
		t.Parallel()
		factor := twofactor.NewTOTP("Acme", newEncrypter(t), store, newClock())
		sealed, err := factor.NewSecret()
		require.NoError(t, err)

		secret, link, err := factor.Setup(sealed, "ada@example.com")

		require.NoError(t, err)
		assert.Len(t, secret, 32)
		assert.NotContains(t, string(sealed), secret)
		parsed, err := url.Parse(link)
		require.NoError(t, err)
		assert.Equal(t, "otpauth", parsed.Scheme)
		assert.Equal(t, "/Acme:ada@example.com", parsed.Path)
		assert.Equal(t, secret, parsed.Query().Get("secret"))
		assert.Equal(t, "SHA1", parsed.Query().Get("algorithm"))
		assert.Equal(t, "6", parsed.Query().Get("digits"))
		assert.Equal(t, "30", parsed.Query().Get("period"))
	})

	t.Run("accepts the current code and remembers its step", func(t *testing.T) {
		t.Parallel()
		clk := newClock()
		factor, cred, secret := enroll(t, store, clk)

		require.NoError(t, factor.Verify(t.Context(), &cred, codeAt(t, secret, clk.Now())))

		assert.Equal(t, clk.Now().Unix()/30, cred.Counter)
	})

	t.Run("accepts a code one step either side, typed with spaces", func(t *testing.T) {
		t.Parallel()
		clk := newClock()
		factor, cred, secret := enroll(t, store, clk)
		code := codeAt(t, secret, clk.Now().Add(-30*time.Second))

		require.NoError(t, factor.Verify(t.Context(), &cred, code[:3]+" "+code[3:]))
		require.NoError(t, factor.Verify(t.Context(), &cred, codeAt(t, secret, clk.Now().Add(30*time.Second))))
	})

	t.Run("refuses codes outside the window or of the wrong shape", func(t *testing.T) {
		t.Parallel()
		clk := newClock()
		factor, cred, secret := enroll(t, store, clk)
		stale := codeAt(t, secret, clk.Now().Add(-90*time.Second))

		for _, code := range []string{stale, "12345", "1234567", "abcdef"} {
			require.ErrorIs(t, factor.Verify(t.Context(), &cred, code), twofactor.ErrInvalidCode, code)
		}
	})

	t.Run("never accepts a step twice", func(t *testing.T) {
		t.Parallel()
		clk := newClock()
		factor, cred, secret := enroll(t, store, clk)
		code := codeAt(t, secret, clk.Now())
		stale := cred

		require.NoError(t, factor.Verify(t.Context(), &cred, code))

		require.ErrorIs(t, factor.Verify(t.Context(), &cred, code), twofactor.ErrCodeReused)
		require.ErrorIs(t, factor.Verify(t.Context(), &stale, code), twofactor.ErrCodeReused)
		earlier := codeAt(t, secret, clk.Now().Add(-30*time.Second))
		require.ErrorIs(t, factor.Verify(t.Context(), &cred, earlier), twofactor.ErrCodeReused)
	})

	t.Run("lets only one of two servers racing on a code through", func(t *testing.T) {
		t.Parallel()
		clk := newClock()
		factor, cred, secret := enroll(t, store, clk)
		code := codeAt(t, secret, clk.Now())

		var (
			wg     sync.WaitGroup
			mu     sync.Mutex
			passed int
		)

		for range 2 {
			wg.Go(func() {
				copied := cred
				if factor.Verify(t.Context(), &copied, code) == nil {
					mu.Lock()
					passed++
					mu.Unlock()
				}
			})
		}

		wg.Wait()
		assert.Equal(t, 1, passed)
	})

	t.Run("fails when the seed cannot be opened", func(t *testing.T) {
		t.Parallel()
		clk := newClock()
		_, cred, secret := enroll(t, store, clk)
		other := twofactor.NewTOTP("Acme", newEncrypter(t), store, clk)

		err := other.Verify(t.Context(), &cred, codeAt(t, secret, clk.Now()))

		require.ErrorIs(t, err, encryption.ErrDecrypt)
		_, _, err = other.Setup(cred.Secret, "ada@example.com")
		require.ErrorIs(t, err, encryption.ErrDecrypt)
	})

	t.Run("needs an account name to set up", func(t *testing.T) {
		t.Parallel()
		factor, cred, _ := enroll(t, store, newClock())

		_, _, err := factor.Setup(cred.Secret, "")

		require.Error(t, err)
	})

	t.Run("fails when the cache is down", func(t *testing.T) {
		t.Parallel()
		down, client := newCache(t)
		clk := newClock()
		factor, cred, secret := enroll(t, down, clk)
		client.Close()

		err := factor.Verify(t.Context(), &cred, codeAt(t, secret, clk.Now()))

		require.Error(t, err)
		require.NotErrorIs(t, err, twofactor.ErrInvalidCode)
	})
}

func TestRecoveryCodes(t *testing.T) {
	t.Parallel()

	format := regexp.MustCompile(`^[0-9a-hjkmnp-tv-z]{5}-[0-9a-hjkmnp-tv-z]{5}$`)

	t.Run("are readable, distinct and stored only as sealed digests", func(t *testing.T) {
		t.Parallel()
		rc := twofactor.NewRecoveryCodes(newEncrypter(t))

		codes, sealed, err := rc.Generate(10)

		require.NoError(t, err)
		require.Len(t, codes, 10)
		seen := map[string]bool{}

		for _, code := range codes {
			assert.Regexp(t, format, code)
			assert.NotContains(t, string(sealed), code)
			seen[code] = true
		}

		assert.Len(t, seen, 10)
		remaining, err := rc.Remaining(twofactor.Credential{Secret: sealed})
		require.NoError(t, err)
		assert.Equal(t, 10, remaining)
	})

	t.Run("work once each, however they are typed", func(t *testing.T) {
		t.Parallel()
		rc := twofactor.NewRecoveryCodes(newEncrypter(t))
		codes, sealed, err := rc.Generate(3)
		require.NoError(t, err)
		cred := twofactor.Credential{ID: ids.New(), Type: twofactor.RecoveryCodeType, Secret: sealed}
		typed := " " + strings.ToUpper(strings.ReplaceAll(codes[1], "-", " ")) + " "

		require.NoError(t, rc.Verify(t.Context(), &cred, typed))

		require.ErrorIs(t, rc.Verify(t.Context(), &cred, codes[1]), twofactor.ErrInvalidCode)
		remaining, err := rc.Remaining(cred)
		require.NoError(t, err)
		assert.Equal(t, 2, remaining)
		require.NoError(t, rc.Verify(t.Context(), &cred, codes[0]))
	})

	t.Run("refuse a code that was never issued", func(t *testing.T) {
		t.Parallel()
		rc := twofactor.NewRecoveryCodes(newEncrypter(t))
		_, sealed, err := rc.Generate(2)
		require.NoError(t, err)

		err = rc.Verify(t.Context(), &twofactor.Credential{Secret: sealed}, "00000-00000")

		require.ErrorIs(t, err, twofactor.ErrInvalidCode)
	})

	t.Run("fail when the list cannot be opened or read", func(t *testing.T) {
		t.Parallel()
		enc := newEncrypter(t)
		rc := twofactor.NewRecoveryCodes(enc)
		_, sealed, err := rc.Generate(2)
		require.NoError(t, err)
		notJSON, err := enc.Encrypt([]byte("not json"))
		require.NoError(t, err)
		other := twofactor.NewRecoveryCodes(newEncrypter(t))

		require.ErrorIs(t, other.Verify(t.Context(), &twofactor.Credential{Secret: sealed}, "x"), encryption.ErrDecrypt)
		_, err = other.Remaining(twofactor.Credential{Secret: sealed})
		require.ErrorIs(t, err, encryption.ErrDecrypt)
		_, err = rc.Remaining(twofactor.Credential{Secret: notJSON})
		require.Error(t, err)
	})
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	store, _ := newCache(t)
	clk := newClock()
	factor, cred, secret := enroll(t, store, clk)
	registry := twofactor.NewRegistry(factor, twofactor.NewRecoveryCodes(newEncrypter(t)))

	t.Run("lists the registered types", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{twofactor.RecoveryCodeType, twofactor.TOTPType}, registry.Types())
		found, ok := registry.Find(twofactor.TOTPType)
		assert.True(t, ok)
		assert.Equal(t, twofactor.TOTPType, found.Type())
	})

	t.Run("verifies with the factor of the credential's type", func(t *testing.T) {
		t.Parallel()
		copied := cred

		require.NoError(t, registry.Verify(t.Context(), &copied, codeAt(t, secret, clk.Now())))
	})

	t.Run("refuses a type nobody registered", func(t *testing.T) {
		t.Parallel()
		unknown := twofactor.Credential{Type: "webauthn"}

		require.ErrorIs(t, registry.Verify(t.Context(), &unknown, "x"), twofactor.ErrUnknownFactor)
	})
}

func TestChallenges(t *testing.T) {
	t.Parallel()

	store, _ := newCache(t)

	begin := func(t *testing.T, challenges *twofactor.Challenges) (string, twofactor.Challenge) {
		t.Helper()

		token, ch, err := challenges.Begin(t.Context(), twofactor.Challenge{
			UserID:  ids.New(),
			Methods: []string{twofactor.TOTPType, twofactor.RecoveryCodeType},
			Data:    map[string]string{"redirect": "/home"},
		})
		require.NoError(t, err)

		return token, ch
	}

	t.Run("remember who passed the first step", func(t *testing.T) {
		t.Parallel()
		clk := newClock()
		challenges := twofactor.NewChallenges(store, clk, 0, 0)
		token, begun := begin(t, challenges)

		found, err := challenges.Find(t.Context(), token)

		require.NoError(t, err)
		assert.Equal(t, begun.UserID, found.UserID)
		assert.Equal(t, begun.Methods, found.Methods)
		assert.Equal(t, "/home", found.Data["redirect"])
		assert.Equal(t, clk.Now().Add(5*time.Minute), found.ExpiresAt.UTC())
	})

	t.Run("expire after their time", func(t *testing.T) {
		t.Parallel()
		clk := newClock()
		challenges := twofactor.NewChallenges(store, clk, time.Minute, 5)
		token, _ := begin(t, challenges)

		clk.Travel(time.Minute)
		_, err := challenges.Find(t.Context(), token)

		require.ErrorIs(t, err, twofactor.ErrChallengeExpired)
		_, err = challenges.Find(t.Context(), "unknown")
		require.ErrorIs(t, err, twofactor.ErrChallengeExpired)
	})

	t.Run("burn after too many wrong answers", func(t *testing.T) {
		t.Parallel()
		challenges := twofactor.NewChallenges(store, newClock(), time.Minute, 3)
		token, _ := begin(t, challenges)

		for _, want := range []int{2, 1} {
			left, err := challenges.Fail(t.Context(), token)
			require.NoError(t, err)
			assert.Equal(t, want, left)
		}

		_, err := challenges.Fail(t.Context(), token)
		require.ErrorIs(t, err, twofactor.ErrChallengeExpired)
		_, err = challenges.Find(t.Context(), token)
		require.ErrorIs(t, err, twofactor.ErrChallengeExpired)
	})

	t.Run("complete once only", func(t *testing.T) {
		t.Parallel()
		challenges := twofactor.NewChallenges(store, newClock(), time.Minute, 5)
		token, begun := begin(t, challenges)

		done, err := challenges.Complete(t.Context(), token)
		require.NoError(t, err)
		assert.Equal(t, begun.UserID, done.UserID)

		_, err = challenges.Complete(t.Context(), token)
		require.ErrorIs(t, err, twofactor.ErrChallengeExpired)
	})

	t.Run("let only one of two racing completions through", func(t *testing.T) {
		t.Parallel()
		challenges := twofactor.NewChallenges(store, newClock(), time.Minute, 5)
		token, _ := begin(t, challenges)

		results := make(chan error, 2)
		for range 2 {
			go func() {
				_, err := challenges.Complete(t.Context(), token)
				results <- err
			}()
		}

		first, second := <-results, <-results
		assert.True(t, (first == nil) != (second == nil))
	})

	t.Run("fail on a corrupted challenge", func(t *testing.T) {
		t.Parallel()
		challenges := twofactor.NewChallenges(store, newClock(), time.Minute, 5)
		key := "twofactor:challenge:" + hashing.SHA256Hex([]byte("corrupt"))
		require.NoError(t, store.Set(t.Context(), key, []byte("{"), time.Minute))

		_, err := challenges.Find(t.Context(), "corrupt")

		require.Error(t, err)
		require.NotErrorIs(t, err, twofactor.ErrChallengeExpired)
	})

	t.Run("fail when the cache is down", func(t *testing.T) {
		t.Parallel()
		down, client := newCache(t)
		challenges := twofactor.NewChallenges(down, newClock(), time.Minute, 5)
		token, _ := begin(t, challenges)
		client.Close()
		ctx := context.WithoutCancel(t.Context())

		_, _, err := challenges.Begin(ctx, twofactor.Challenge{UserID: ids.New()})
		require.Error(t, err)
		_, err = challenges.Find(ctx, token)
		require.Error(t, err)
		_, err = challenges.Fail(ctx, token)
		require.Error(t, err)
		_, err = challenges.Complete(ctx, token)
		require.Error(t, err)
	})
}

func TestCode(t *testing.T) {
	t.Parallel()

	t.Run("make the code an authenticator app shows", func(t *testing.T) {
		t.Parallel()
		store, _ := newCache(t)
		clk := newClock()
		factor, cred, secret := enroll(t, store, clk)

		code, err := twofactor.Code(secret, clk.Now())

		require.NoError(t, err)
		assert.Equal(t, codeAt(t, secret, clk.Now()), code)
		require.NoError(t, factor.Verify(t.Context(), &cred, code))
	})

	t.Run("refuse a secret that is not base32", func(t *testing.T) {
		t.Parallel()

		_, err := twofactor.Code("not base32!", newClock().Now())

		require.Error(t, err)
	})
}

func TestQRCodeSVG(t *testing.T) {
	t.Parallel()

	t.Run("draw every module of the code inside a quiet zone", func(t *testing.T) {
		t.Parallel()
		link := "otpauth://totp/Acme:ada%40example.com?secret=JBSWY3DPEHPK3PXP&issuer=Acme"

		svg, err := twofactor.QRCodeSVG(link)

		require.NoError(t, err)
		side := regexp.MustCompile(`viewBox="0 0 (\d+) (\d+)"`).FindStringSubmatch(svg)
		require.Len(t, side, 3)
		assert.Equal(t, side[1], side[2])
		modules, err := strconv.Atoi(side[1])
		require.NoError(t, err)
		assert.Zero(t, (modules-8-17)%4, "a QR code is 17 plus a multiple of 4 modules wide")
		assert.True(t, strings.HasPrefix(svg, "<svg "))
		assert.Contains(t, svg, "M4 4h1v1h-1z")
		assert.Contains(t, svg, "M10 10h1v1h-1z")
		assert.NotContains(t, svg, "M3 3h1v1h-1z")
		assert.NotContains(t, svg, "M5 5h1v1h-1z")
	})

	t.Run("refuse a url that is not an otpauth url", func(t *testing.T) {
		t.Parallel()

		_, err := twofactor.QRCodeSVG("%zz")

		require.Error(t, err)
	})
}
