package hashing_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/hashing"
)

func TestDigests(t *testing.T) {
	t.Parallel()

	t.Run("computes sha-256 per fips 180-2", func(t *testing.T) {
		t.Parallel()
		want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"

		assert.Equal(t, want, hashing.SHA256Hex([]byte("abc")))
		assert.Equal(t, want, hex.EncodeToString(hashing.SHA256([]byte("abc"))))

		stream := hashing.NewSHA256()
		_, _ = stream.Write([]byte("a"))
		_, _ = stream.Write([]byte("bc"))
		assert.Equal(t, want, hex.EncodeToString(stream.Sum(nil)))
	})

	t.Run("computes hmac-sha256 per rfc 4231", func(t *testing.T) {
		t.Parallel()

		mac := hashing.HMAC([]byte("Jefe"), []byte("what do ya want for nothing?"))

		assert.Equal(t, "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843", hex.EncodeToString(mac))
	})

	t.Run("compares bytes for equality", func(t *testing.T) {
		t.Parallel()

		assert.True(t, hashing.Equal([]byte("same"), []byte("same")))
		assert.False(t, hashing.Equal([]byte("same"), []byte("sane")))
		assert.False(t, hashing.Equal([]byte("same"), []byte("same!")))
		assert.True(t, hashing.Equal(nil, []byte{}))
	})
}
