package str_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/support/str"
)

func TestClean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in, want string
	}{
		{"composes decomposed letters", "Ade\u0301la", "Adéla"},
		{"drops control characters", "Ada\tLove\x00lace", "AdaLovelace"},
		{"drops zero-width spaces and marks", "Ada\u200b \u200fLovelace\u2060", "Ada Lovelace"},
		{"keeps the joiner inside emoji sequences", "👩\u200d💻 Ada", "👩\u200d💻 Ada"},
		{"keeps right-to-left text", "  \u05e9\u05dc\u05d5\u05dd  ", "\u05e9\u05dc\u05d5\u05dd"},
		{"turns invisible-only text empty", "\u200b\u200c\t", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, str.Clean(tt.in))
		})
	}
}
