/*
|--------------------------------------------------------------------------
| Environment Overrides
|--------------------------------------------------------------------------
|
| Every configuration key can be overridden by one environment variable:
| the prefix, then the key path in upper case with a double underscore
| between levels, so mail.mailers.system.driver becomes
| APP_MAIL__MAILERS__SYSTEM__DRIVER. A variable ending in _FILE names a
| file holding the value instead, which is how container platforms hand
| out secrets. A file value beats a plain value for the same key, and an
| unreadable file stops the load rather than leaving the key empty.
|
*/

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
)

const (
	levelSeparator = "__"
	secretSuffix   = "_FILE"
)

func (o LoadOptions) loadEnv(k *koanf.Koanf) error {
	if o.EnvPrefix == "" {
		return nil
	}

	prefix := strings.TrimSuffix(o.EnvPrefix, "_") + "_"

	if err := k.Load(o.envProvider(prefix, plainValue(prefix)), nil); err != nil {
		return fmt.Errorf("read environment: %w", err)
	}

	secrets := &secretFiles{prefix: prefix}
	if err := k.Load(o.envProvider(prefix, secrets.read), nil); err != nil {
		return fmt.Errorf("read secret files: %w", err)
	}

	return errors.Join(secrets.errs...)
}

func (o LoadOptions) envProvider(prefix string, transform func(name, value string) (string, any)) *env.Env {
	opt := env.Opt{Prefix: prefix, TransformFunc: transform}
	if o.Environ != nil {
		opt.EnvironFunc = func() []string { return o.Environ }
	}

	return env.Provider(".", opt)
}

/*
plainValue maps each variable to its key, skipping secret file variables
so they can be applied after every plain value.
*/
func plainValue(prefix string) func(name, value string) (string, any) {
	return func(name, value string) (string, any) {
		if isSecretFile(name) {
			return "", nil
		}

		return envKey(prefix, name), value
	}
}

/*
secretFiles reads the files named by _FILE variables, keeping every read
error so one load reports all missing secrets at once.
*/
type secretFiles struct {
	prefix string
	errs   []error
}

func (s *secretFiles) read(name, path string) (string, any) {
	if !isSecretFile(name) {
		return "", nil
	}

	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		s.errs = append(s.errs, fmt.Errorf("read %s: %w", name, err))

		return "", nil
	}

	return envKey(s.prefix, strings.TrimSuffix(name, secretSuffix)), strings.TrimSpace(string(contents))
}

/*
isSecretFile reports whether the variable names a secret file. A name
ending in __FILE is a plain key called file, not a secret.
*/
func isSecretFile(name string) bool {
	return strings.HasSuffix(name, secretSuffix) && !strings.HasSuffix(name, levelSeparator+"FILE")
}

func envKey(prefix, name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(name, prefix)), levelSeparator, ".")
}
