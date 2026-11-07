package urlmatch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/urlmatch"
)

func TestMatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, pattern, value string
		want                 bool
	}{
		{"matches an exact path", "/pricing", "/pricing", true},
		{"ignores case", "/Pricing", "/pricing", true},
		{"ignores a trailing slash", "/pricing/", "/pricing", true},
		{"keeps the root path", "/", "/", true},
		{"single star stays in one segment", "/docs/*", "/docs/setup", true},
		{"single star refuses deeper paths", "/docs/*", "/docs/setup/keys", false},
		{"single star needs the prefix", "/docs/*", "/doc", false},
		{"double star crosses segments", "/docs/**", "/docs/setup/keys", true},
		{"star inside a segment", "/blog/*-launch", "/blog/big-launch", true},
		{"star in the middle", "/a/*/c", "/a/b/c", true},
		{"star in the middle refuses two segments", "/a/*/c", "/a/b/x/c", false},
		{"double star in the middle", "/a/**/c", "/a/b/x/c", true},
		{"whole value must match", "/pricing", "/pricing/team", false},
		{"different text fails", "/pricing", "/billing", false},
		{"lone star matches a segment", "*", "pricing", true},
		{"empty pattern matches only empty", "", "", true},
		{"empty pattern refuses text", "", "/x", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, urlmatch.Match(tc.pattern, tc.value))
		})
	}
}
