package arch_test

import (
	"fmt"
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
secretPatterns are what must never be committed under deploy, keyed by
what they mean. The repository is public, so even an account id counts.
A password is a quoted literal or a bare value ending the line, as in an
env file; code that only names a password, such as a field typed Secret or
a call reading one, is not a finding.
*/
func secretPatterns() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"aws access key": regexp.MustCompile(`\b(?:AKIA|ASIA|AGPA|AIDA|AROA|ANPA)[0-9A-Z]{16}\b`),
		"aws account id": regexp.MustCompile(`(^|[^0-9])[0-9]{12}([^0-9]|$)`),
		"password":       regexp.MustCompile(`(?i)password["']?\s*[:=]\s*(?:["'][^\s"']{4,}["']|[^\s"'(){}\[\],;]+\s*$)`),
		"private key":    regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY`),
	}
}

/*
skippedForSecrets reports whether the walk leaves the entry out: installed
packages, build output and saved stack outputs are not committed, and
lockfiles are full of hashes.
*/
func skippedForSecrets(entry fs.DirEntry) bool {
	if entry.IsDir() {
		return entry.Name() == "node_modules" || entry.Name() == "cdk.out" || entry.Name() == "outputs"
	}

	return entry.Name() == "pnpm-lock.yaml"
}

/*
scanForSecrets returns one finding per line of the tree that matches a
pattern, naming the kind of secret but never its value.
*/
func scanForSecrets(root string) ([]string, error) {
	var findings []string

	patterns := secretPatterns()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if skippedForSecrets(entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if entry.IsDir() {
			return nil
		}

		contents, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		for number, line := range strings.Split(string(contents), "\n") {
			for kind, pattern := range patterns {
				if pattern.MatchString(line) {
					findings = append(findings, fmt.Sprintf("%s:%d: %s", path, number+1, kind))
				}
			}
		}

		return nil
	})

	return findings, err
}

func TestDeploySecrets(t *testing.T) {
	t.Parallel()

	t.Run("nothing under deploy looks like a secret or an account id", func(t *testing.T) {
		t.Parallel()

		findings, err := scanForSecrets(filepath.Join("..", "..", "deploy"))

		require.NoError(t, err)
		assert.Empty(t, findings)
	})

	t.Run("the scanner catches each kind of secret", func(t *testing.T) {
		t.Parallel()

		seeds := map[string]string{
			"aws access key": "key: " + "AKIA" + strings.Repeat("A", 16),
			"aws account id": `arn:aws:iam::` + strings.Repeat("1", 12) + `:role/x`,
			"password":       `password = "` + "hunter2" + `"`,
			"private key":    "-----BEGIN " + "RSA PRIVATE KEY-----",
		}

		for kind, seed := range seeds {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "seed.ts"), []byte(seed), 0o600))

			findings, err := scanForSecrets(root)

			require.NoError(t, err)
			require.Len(t, findings, 1, kind)
			assert.Contains(t, findings[0], kind)
		}
	})

	t.Run("the scanner reads dot files and nested folders", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		nested := filepath.Join(root, "lib", "mail")
		require.NoError(t, os.MkdirAll(nested, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(nested, ".env.staging"), []byte("password=abc"), 0o600))

		findings, err := scanForSecrets(root)

		require.NoError(t, err)
		assert.Len(t, findings, 1)
	})

	t.Run("the scanner leaves packages, build output and lockfiles alone", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		seed := []byte(strings.Repeat("1", 12))

		for _, dir := range []string{"node_modules", "cdk.out", "outputs"} {
			require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o750))
			require.NoError(t, os.WriteFile(filepath.Join(root, dir, "file.json"), seed, 0o600))
		}

		require.NoError(t, os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), seed, 0o600))

		findings, err := scanForSecrets(root)

		require.NoError(t, err)
		assert.Empty(t, findings)
	})

	t.Run("code that only names a password is not a finding", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		lines := strings.Join([]string{
			"  readonly gatePassword: Secret;",
			`  PGPASSWORD: EcsSecret.fromSecretsManager(secret, "password"),`,
			`  const password = secret.secretValueFromJson("password").unsafeUnwrap();`,
			`  secretStringTemplate: JSON.stringify({ password: "" }),`,
		}, "\n")
		require.NoError(t, os.WriteFile(filepath.Join(root, "code.ts"), []byte(lines), 0o600))

		findings, err := scanForSecrets(root)

		require.NoError(t, err)
		assert.Empty(t, findings)
	})

	t.Run("a number of another length is not an account id", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		lines := strings.Repeat("1", 11) + "\n" + strings.Repeat("1", 13)
		require.NoError(t, os.WriteFile(filepath.Join(root, "numbers.txt"), []byte(lines), 0o600))

		findings, err := scanForSecrets(root)

		require.NoError(t, err)
		assert.Empty(t, findings)
	})
}
