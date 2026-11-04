package arch_test

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
reflectMethodMarker is how the linker's dependency dump flags a function
that looks methods up by a name known only at run time. One such
function reachable from main makes the linker keep every exported
method of every type, roughly doubling the binary.
*/
const reflectMethodMarker = "<ReflectMethod>"

func TestBinarySize(t *testing.T) {
	t.Parallel()

	t.Run("an application keeps the linker's dead method elimination", func(t *testing.T) {
		t.Parallel()

		cmd := exec.CommandContext(t.Context(), "go", "build", "-ldflags=-dumpdep", "-o", "/dev/null",
			"./testdata/sizeapp")
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")

		dump, err := cmd.CombinedOutput()
		require.NoError(t, err, string(dump))
		require.Contains(t, string(dump), "main.main -> ", "the linker printed no dependencies")

		assert.Empty(t, flagged(dump), "functions that disable dead method elimination")
	})

	t.Run("the binary carries its own time zone database", func(t *testing.T) {
		t.Parallel()

		cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", "../../cmd/dimmi")

		deps, err := cmd.CombinedOutput()
		require.NoError(t, err, string(deps))
		assert.Contains(t, strings.Fields(string(deps)), "time/tzdata")
	})
}

/*
flagged returns the dependency lines whose caller looks methods up by a
run time name, which name the function to remove from the build.
*/
func flagged(dump []byte) []string {
	var lines []string

	scanner := bufio.NewScanner(bytes.NewReader(dump))
	for scanner.Scan() {
		if line := scanner.Text(); strings.Contains(line, reflectMethodMarker) && len(lines) < 20 {
			lines = append(lines, line)
		}
	}

	return lines
}
