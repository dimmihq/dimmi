package arch_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
TestDashboardPagesAreReservedSlugs keeps every top-level dashboard page
off the workspace addresses the server hands out. Dashboard addresses
start with the workspace's slug, so a page the server would give away
as a slug reads as a workspace someone is not in.
*/
func TestDashboardPagesAreReservedSlugs(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir("../../resources/js/routes/d")
	require.NoError(t, err)

	source, err := os.ReadFile("../../app/actions/workspaces/slugs.go")
	require.NoError(t, err)

	list := regexp.MustCompile(`(?s)func reservedSlug\(.*?\{(.*?)\}, slug\)`).FindSubmatch(source)
	require.Len(t, list, 2, "slugs.go has no reservedSlug list")

	pages := 0

	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".tsx")
		if name == "index" || name == "route" {
			continue
		}

		pages++
		assert.Contains(t, string(list[1]), `"`+name+`"`, "dashboard page %q is not a reserved slug", name)
	}

	assert.Greater(t, pages, 10)
}
