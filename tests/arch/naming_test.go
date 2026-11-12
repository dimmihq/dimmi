package arch_test

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
Files and functions are named for what they hold. A number that only
counts the parts a long file was cut into, as in catalog_inbox_2.go or
articleDocs3, tells a reader nothing, so a file that grows too long is
split by topic instead.
*/
var (
	numberedFile = regexp.MustCompile(`[_-]\d+((_test)?\.go|(\.test)?\.tsx?)$`)
	goFunction   = regexp.MustCompile(`^func (?:\([^)]*\) )?([A-Za-z_]\w*)[\[(]`)
	countedName  = regexp.MustCompile(`[a-z]\d+$`)

	/*
		technicalSuffix are names whose trailing number is part of a
		standard's or a format's name, not a count.
	*/
	technicalSuffix = regexp.MustCompile(
		`(?i)(ipv4|ipv6|pkcs\d+|sha\d+|base\d+|utf\d+|int\d+|float\d+|oauth2|http2|md5|` +
			`crc32|bm25|s3|x509|e2e|e164|ed25519|rfc\d+|rsa\d+|sat\d+|v\d+)$`)
)

/*
sourceRoots are the folders whose files the rule covers, from the
repository root.
*/
func sourceRoots() []string {
	return []string{
		"app", "bootstrap", "cmd", "config", "framework", "routes", "tests", "resources/js", "resources/widget",
	}
}

/*
skipped reports folders the rule leaves alone: dependencies, fixtures and
migrations, whose names are versions or data.
*/
func skipped(name string) bool {
	switch name {
	case "node_modules", "testdata", "fixtures", "migrations", ".git":
		return true
	}

	return false
}

func TestNaming(t *testing.T) {
	t.Parallel()

	t.Run("names files and functions for what they hold, never by a count", func(t *testing.T) {
		t.Parallel()

		checked := 0
		for _, root := range sourceRoots() {
			err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.IsDir() {
					return skipFolder(d.Name())
				}

				checked++
				checkFile(t, path)

				return nil
			})
			require.NoError(t, err)
		}

		assert.Positive(t, checked)
	})
}

/*
skipFolder skips a folder the rule leaves alone and walks any other.
*/
func skipFolder(name string) error {
	if skipped(name) {
		return filepath.SkipDir
	}

	return nil
}

/*
checkFile fails a file named by a count, and a Go file's functions named
the same way.
*/
func checkFile(t *testing.T, path string) {
	t.Helper()

	assert.False(t, numberedFile.MatchString(filepath.Base(path)), "%s: name the file for what it holds", path)
	if strings.HasSuffix(path, ".go") {
		checkFunctions(t, path)
	}
}

/*
checkFunctions fails a Go function whose name ends in a count.
*/
func checkFunctions(t *testing.T, path string) {
	t.Helper()

	file, err := os.Open(filepath.Clean(path))
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	lines := bufio.NewScanner(file)
	lines.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for number := 1; lines.Scan(); number++ {
		match := goFunction.FindStringSubmatch(lines.Text())
		if len(match) < 2 || !countedName.MatchString(match[1]) || technicalSuffix.MatchString(match[1]) {
			continue
		}

		assert.Fail(t, "name the function for what it does", "%s:%d: %s", path, number, match[1])
	}

	require.NoError(t, lines.Err())
}
