/*
|--------------------------------------------------------------------------
| Test Environment
|--------------------------------------------------------------------------
|
| Tests talk to the same kinds of services production does: PostgreSQL,
| Valkey and S3. They find them through environment variables, whose
| values come from the application's test file, named <app>.test.yaml
| beside go.mod: each of its keys is the variable of the same name in
| capitals, and a variable already set in the environment wins. A
| variable neither has fails the test loudly instead of skipping it, so
| a green run always means the code ran against real services.
|
*/

package testkit

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knadh/koanf/parsers/yaml"
)

/*
configPattern matches the application's test file beside go.mod.
*/
const configPattern = "*.test.yaml"

/*
Env returns the environment variable's value, failing the test when it
is not set.
*/
func Env(tb testing.TB, name string) string {
	tb.Helper()

	if value := os.Getenv(name); value != "" {
		return value
	}

	if err := loadConfig(); err != nil {
		tb.Fatalf("read the test file (%s beside go.mod): %v", configPattern, err)
	}

	value := os.Getenv(name)
	if value == "" {
		tb.Fatalf("%s is not set: add %s to the test file (%s beside go.mod)", name, strings.ToLower(name),
			configPattern)
	}

	return value
}

/*
loadConfig sets every variable the config file names that the environment
has not set, so code that reads the environment itself, such as the AWS
SDK, sees them too. Having no test file is not an error.
*/
func loadConfig() error {
	path := configPath()
	if path == "" {
		return nil
	}

	raw, err := os.ReadFile(path) //nolint:gosec // G304: the repository's own test config.
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	values, err := yaml.Parser().Unmarshal(raw)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	for key, value := range values {
		name := strings.ToUpper(key)
		if os.Getenv(name) != "" {
			continue
		}

		if err := os.Setenv(name, fmt.Sprint(value)); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}

	return nil
}

/*
configPath finds the test file: the first match of the pattern in the
nearest directory at or above the working directory that holds go.mod.
*/
func configPath() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			found, _ := filepath.Glob(filepath.Join(dir, configPattern))
			if len(found) == 0 {
				return ""
			}

			return found[0]
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}

		dir = parent
	}
}

/*
Switch reports whether the environment variable is set to 1, for runs
that change how every test behaves, such as connecting as another role.
*/
func Switch(name string) bool {
	return os.Getenv(name) == "1"
}

/*
UniqueName returns the prefix followed by ten random lowercase letters
and digits, safe as a database, role, bucket or key-prefix name.
*/
func UniqueName(prefix string) string {
	return prefix + strings.ToLower(rand.Text()[:10])
}
