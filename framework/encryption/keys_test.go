package encryption_test

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/encryption/drivertest"
)

func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("round trips values bound to their associated data", func(t *testing.T) {
		t.Parallel()
		keys := newKeys(t, encryption.KeysOptions{})

		ciphertext, err := keys.Encrypt(t.Context(), tenant("1"), []byte("token"), []byte("channels/secret/7"))
		require.NoError(t, err)

		plaintext, err := keys.Decrypt(t.Context(), ciphertext, []byte("channels/secret/7"))
		require.NoError(t, err)
		assert.Equal(t, "token", string(plaintext))

		_, err = keys.Decrypt(t.Context(), ciphertext, []byte("channels/secret/8"))
		require.ErrorIs(t, err, encryption.ErrDecrypt)
	})

	t.Run("keeps scopes apart", func(t *testing.T) {
		t.Parallel()
		keys := newKeys(t, encryption.KeysOptions{})
		ciphertext, err := keys.Encrypt(t.Context(), tenant("1"), []byte("token"), nil)
		require.NoError(t, err)
		_, err = keys.Encrypt(t.Context(), tenant("2"), []byte("token"), nil)
		require.NoError(t, err)

		moved := bytes.Replace(ciphertext, []byte("tenant\x011"), []byte("tenant\x012"), 1)
		_, err = keys.Decrypt(t.Context(), moved, nil)
		require.ErrorIs(t, err, encryption.ErrDecrypt)

		missing := bytes.Replace(ciphertext, []byte("tenant\x011"), []byte("tenant\x019"), 1)
		_, err = keys.Decrypt(t.Context(), missing, nil)
		require.ErrorIs(t, err, encryption.ErrKeysetNotFound)
	})

	t.Run("refuses malformed ciphertext", func(t *testing.T) {
		t.Parallel()
		keys := newKeys(t, encryption.KeysOptions{})

		for _, ciphertext := range [][]byte{nil, {2}, {1}, {1, 0x80}, {1, 200}, {1, 1, 'a'}, {1, 1, 'a', 9, 'b'}} {
			_, err := keys.Decrypt(t.Context(), ciphertext, nil)
			require.ErrorIs(t, err, encryption.ErrMalformed, "%v", ciphertext)
		}
	})

	t.Run("requires a kek and a store", func(t *testing.T) {
		t.Parallel()

		_, err := encryption.NewKeys(encryption.KeysOptions{Store: drivertest.NewPostgresStore(t)})
		require.ErrorIs(t, err, encryption.ErrInvalidOptions)
		_, err = encryption.NewKeys(encryption.KeysOptions{KEK: localKEK(t)})
		require.ErrorIs(t, err, encryption.ErrInvalidOptions)
	})

	t.Run("shares keysets between servers on one store", func(t *testing.T) {
		t.Parallel()
		store, kek := drivertest.NewPostgresStore(t), localKEK(t)
		servers := make([]*encryption.Keys, 6)
		for i := range servers {
			servers[i] = newKeys(t, encryption.KeysOptions{KEK: kek, Store: store})
		}

		ciphertexts := make([][]byte, len(servers))
		var wg sync.WaitGroup
		for i, server := range servers {
			wg.Go(func() {
				ciphertext, err := server.Encrypt(t.Context(), tenant("new"), []byte("x"), nil)
				assert.NoError(t, err)
				ciphertexts[i] = ciphertext
			})
		}
		wg.Wait()

		for _, ciphertext := range ciphertexts {
			_, err := servers[0].Decrypt(t.Context(), ciphertext, nil)
			require.NoError(t, err)
		}
	})
}

/*
sealedBy returns the id of the key that sealed a ciphertext of tenant
"1": the four bytes after the ten byte header and Tink's prefix byte.
*/
func sealedBy(ciphertext []byte) []byte {
	return ciphertext[11:15]
}

func TestRotation(t *testing.T) {
	t.Parallel()

	t.Run("decrypts across a rotation made by another server", func(t *testing.T) {
		t.Parallel()
		store, kek := drivertest.NewPostgresStore(t), localKEK(t)
		first := newKeys(t, encryption.KeysOptions{KEK: kek, Store: store})
		second := newKeys(t, encryption.KeysOptions{KEK: kek, Store: store})
		before, err := first.Encrypt(t.Context(), tenant("1"), []byte("before"), nil)
		require.NoError(t, err)
		mac, err := first.MAC(t.Context(), tenant("1"), []byte("a@example.com"))
		require.NoError(t, err)

		require.NoError(t, second.Rotate(t.Context(), tenant("1")))
		after, err := second.Encrypt(t.Context(), tenant("1"), []byte("after"), nil)
		require.NoError(t, err)

		for want, ciphertext := range map[string][]byte{"before": before, "after": after} {
			plaintext, err := first.Decrypt(t.Context(), ciphertext, nil)
			require.NoError(t, err)
			assert.Equal(t, want, string(plaintext))
		}
		assert.NotEqual(t, sealedBy(before), sealedBy(after))

		again, err := second.MAC(t.Context(), tenant("1"), []byte("a@example.com"))
		require.NoError(t, err)
		assert.Equal(t, mac, again)
	})

	t.Run("refuses a ciphertext the stored keyset cannot open", func(t *testing.T) {
		t.Parallel()
		keys := newKeys(t, encryption.KeysOptions{})
		ciphertext, err := keys.Encrypt(t.Context(), tenant("1"), []byte("x"), nil)
		require.NoError(t, err)

		ciphertext[len(ciphertext)-1] ^= 1
		_, err = keys.Decrypt(t.Context(), ciphertext, nil)

		require.ErrorIs(t, err, encryption.ErrDecrypt)
	})

	t.Run("rewraps every keyset for a new kek", func(t *testing.T) {
		t.Parallel()
		store, old, current := drivertest.NewPostgresStore(t), localKEK(t), localKEK(t)
		before := newKeys(t, encryption.KeysOptions{KEK: old, Store: store})
		ciphertext, err := before.Encrypt(t.Context(), tenant("1"), []byte("x"), nil)
		require.NoError(t, err)
		_, err = before.MAC(t.Context(), encryption.Scope{Kind: "system"}, []byte("y"))
		require.NoError(t, err)

		moving := newKeys(t, encryption.KeysOptions{KEK: current, Previous: []encryption.KEK{old}, Store: store})
		require.NoError(t, moving.RewrapAll(t.Context()))

		after := newKeys(t, encryption.KeysOptions{KEK: current, Store: store})
		plaintext, err := after.Decrypt(t.Context(), ciphertext, nil)
		require.NoError(t, err)
		assert.Equal(t, "x", string(plaintext))

		stale := newKeys(t, encryption.KeysOptions{KEK: old, Store: store})
		_, err = stale.Decrypt(t.Context(), ciphertext, nil)
		require.ErrorIs(t, err, encryption.ErrDecrypt)
		require.ErrorIs(t, stale.RewrapAll(t.Context()), encryption.ErrDecrypt)
		require.ErrorIs(t, stale.Rotate(t.Context(), tenant("1")), encryption.ErrDecrypt)
	})

	t.Run("reloads a keyset once the cache expires", func(t *testing.T) {
		t.Parallel()
		store, kek := drivertest.NewPostgresStore(t), localKEK(t)
		c := clock.NewFake(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))
		cached := newKeys(t, encryption.KeysOptions{KEK: kek, Store: store, Clock: c, CacheTTL: time.Minute})
		other := newKeys(t, encryption.KeysOptions{KEK: kek, Store: store})
		first, err := cached.Encrypt(t.Context(), tenant("1"), nil, nil)
		require.NoError(t, err)

		require.NoError(t, other.Rotate(t.Context(), tenant("1")))
		c.Travel(2 * time.Minute)
		second, err := cached.Encrypt(t.Context(), tenant("1"), nil, nil)
		require.NoError(t, err)

		assert.NotEqual(t, first[:12], second[:12])
	})
}

func TestMAC(t *testing.T) {
	t.Parallel()

	t.Run("is deterministic per scope", func(t *testing.T) {
		t.Parallel()
		keys := newKeys(t, encryption.KeysOptions{})

		a, err := keys.MAC(t.Context(), tenant("1"), []byte("a@example.com"))
		require.NoError(t, err)
		again, err := keys.MAC(t.Context(), tenant("1"), []byte("a@example.com"))
		require.NoError(t, err)
		other, err := keys.MAC(t.Context(), tenant("2"), []byte("a@example.com"))
		require.NoError(t, err)

		assert.Len(t, a, 32)
		assert.Equal(t, a, again)
		assert.NotEqual(t, a, other)
	})
}

func TestStreams(t *testing.T) {
	t.Parallel()

	t.Run("round trips a stream larger than one segment", func(t *testing.T) {
		t.Parallel()
		keys := newKeys(t, encryption.KeysOptions{})
		body := bytes.Repeat([]byte("backup "), 400_000)

		var sealed bytes.Buffer
		w, err := keys.NewStreamWriter(t.Context(), tenant("1"), &sealed)
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
		require.NoError(t, w.Close())

		r, err := keys.NewStreamReader(t.Context(), tenant("1"), bytes.NewReader(sealed.Bytes()))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, body, got)
	})

	t.Run("refuses a stream of another scope", func(t *testing.T) {
		t.Parallel()
		keys := newKeys(t, encryption.KeysOptions{})
		var sealed bytes.Buffer
		w, err := keys.NewStreamWriter(t.Context(), tenant("1"), &sealed)
		require.NoError(t, err)
		require.NoError(t, w.Close())
		_, err = keys.NewStreamWriter(t.Context(), tenant("2"), io.Discard)
		require.NoError(t, err)

		_, err = keys.NewStreamReader(t.Context(), tenant("3"), &sealed)
		require.ErrorIs(t, err, encryption.ErrKeysetNotFound)

		r, err := keys.NewStreamReader(t.Context(), tenant("2"), &sealed)
		if err == nil {
			_, err = io.ReadAll(r)
		}
		require.Error(t, err)
	})
}

func TestSealed(t *testing.T) {
	t.Parallel()

	type credentials struct {
		Token string `json:"token"`
	}
	slot := encryption.Slot{Scope: tenant("1"), AAD: []byte("channels/credentials/7")}

	t.Run("seals and opens a value for its slot", func(t *testing.T) {
		t.Parallel()
		keys := newKeys(t, encryption.KeysOptions{})

		sealed, err := encryption.Seal(t.Context(), keys, slot, credentials{Token: "xoxb-1"})
		require.NoError(t, err)
		assert.NotContains(t, string(sealed), "xoxb")

		got, err := encryption.Open[credentials](t.Context(), keys, sealed, slot.AAD)
		require.NoError(t, err)
		assert.Equal(t, credentials{Token: "xoxb-1"}, got)

		_, err = encryption.Open[credentials](t.Context(), keys, sealed, []byte("channels/credentials/8"))
		require.ErrorIs(t, err, encryption.ErrDecrypt)
	})

	t.Run("reports values that do not encode or decode", func(t *testing.T) {
		t.Parallel()
		keys := newKeys(t, encryption.KeysOptions{})

		_, err := encryption.Seal(t.Context(), keys, slot, func() {})
		require.ErrorContains(t, err, "encode sealed value")

		sealed, err := encryption.Seal(t.Context(), keys, slot, "text")
		require.NoError(t, err)
		_, err = encryption.Open[int](t.Context(), keys, sealed, slot.AAD)
		require.ErrorContains(t, err, "decode sealed value")

		long := encryption.Slot{Scope: encryption.Scope{Kind: "tenant", ID: strings.Repeat("x", 256)}}
		_, err = encryption.Seal(t.Context(), keys, long, 1)
		require.ErrorIs(t, err, encryption.ErrInvalidScope)
	})
}
