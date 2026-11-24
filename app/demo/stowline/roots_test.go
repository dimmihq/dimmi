package stowline

import (
	"io/fs"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	resprompts "github.com/dimmihq/dimmi/resources/prompts"
)

/*
grouped writes a count the way the help center does, 1500 as 1,500.
*/
func grouped(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	return s[:len(s)-3] + "," + s[len(s)-3:]
}

/*
TestRootsHoldNoDemoData keeps the demo workspace out of what the agents
are told. A root system prompt and a template's guidance are tried out
on this workspace, so a price or an allowance of its plans written into
either would let them pass by reciting it. The figures are read from the
demo's own plans, so the check follows them when they change. A plan's
name is left out where it is a word any product uses, such as Free or
Pro.
*/
func TestRootsHoldNoDemoData(t *testing.T) {
	t.Parallel()

	telling := []string{Name}
	for _, p := range Plans() {
		if p.MonthlyUSD > 0 {
			telling = append(telling, "$"+strconv.Itoa(p.MonthlyUSD))
		}
		if p.Labels >= 1000 {
			telling = append(telling, grouped(p.Labels))
		}
	}
	require.Greater(t, len(telling), 4)

	var read int
	walk := func(tree fs.FS, dir string, keep func(string) bool) {
		require.NoError(t, fs.WalkDir(tree, dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !keep(path) {
				return err
			}
			raw, err := fs.ReadFile(tree, path)
			require.NoError(t, err)
			read++
			for _, word := range telling {
				assert.NotContains(t, string(raw), word, "%s holds the demo workspace's %q", path, word)
			}
			return nil
		}))
	}
	walk(resprompts.Files, "agent", func(path string) bool { return strings.HasSuffix(path, "sales.system.md") })
	walk(resprompts.Guidance, "guidance", func(string) bool { return true })
	assert.GreaterOrEqual(t, read, 4, "the sales root of each model family and the sales guidance were read")
}
