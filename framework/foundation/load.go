/*
|--------------------------------------------------------------------------
| Configuration Loading
|--------------------------------------------------------------------------
|
| The framework's configuration is loaded from the application's files
| and environment, then completed with working defaults for every
| service that has drivers: a fresh install hashes with argon2id, keeps
| files on local disk, logs mail instead of sending it and terminates
| TLS elsewhere, with nothing configured. Only then is it checked, so the
| defaults are checked too.
|
*/

package foundation

import (
	"cmp"
	"context"
	"path/filepath"

	"github.com/dimmihq/dimmi/framework/config"
)

/*
LoadConfig loads the framework configuration the options point at and
checks it, then loads the application's own configuration into
opts.Config when it is set. When opts.Configure is set it then adjusts
the framework configuration, which is checked again. A configuration
that cannot work returns a *ConfigError.
*/
func LoadConfig(ctx context.Context, opts Options) (*Config, error) {
	sources := config.LoadOptions{
		Name:      opts.Name,
		Files:     opts.ConfigFiles,
		EnvPrefix: opts.EnvPrefix,
		Environ:   opts.Environ,
	}

	cfg, err := config.Load[Config](ctx, sources)
	if err != nil {
		return nil, &ConfigError{Err: err, Redacted: map[string]any{}}
	}

	cfg.complete(opts)

	if err := cfg.check(); err != nil {
		return nil, &ConfigError{Err: err, Redacted: config.Redacted(cfg)}
	}

	if opts.Config != nil {
		if err := config.LoadInto(ctx, sources, opts.Config); err != nil {
			return nil, &ConfigError{Err: err, Redacted: config.Redacted(opts.Config)}
		}
	}

	if opts.Configure == nil {
		return cfg, nil
	}

	opts.Configure(cfg)

	if err := cfg.check(); err != nil {
		return nil, &ConfigError{Err: err, Redacted: config.Redacted(cfg)}
	}

	return cfg, nil
}

/*
complete fills what the operator left empty with defaults that work on
one machine, and allows fakes only in the testing environment.
*/
func (c *Config) complete(opts Options) {
	if c.Telemetry.ServiceName == "" || c.Telemetry.ServiceName == "app" {
		c.Telemetry.ServiceName = cmp.Or(opts.Name, "app")
	}

	c.Telemetry.Environment = cmp.Or(c.Telemetry.Environment, string(c.App.Env))
	c.Redis.KeyPrefix = cmp.Or(c.Redis.KeyPrefix, cmp.Or(opts.Name, "app")+":")

	c.Hashing.Default = cmp.Or(c.Hashing.Default, "argon2id")
	c.Hashing.Hashers = orSections(c.Hashing.Hashers, "argon2id", "bcrypt")
	c.Filesystems.Default = cmp.Or(c.Filesystems.Default, "local")
	c.Filesystems.Disks = orSection(c.Filesystems.Disks, "local",
		config.Section{"driver": "local", "root": filepath.Join(c.App.DataDir, "app")})
	c.Mail.Mailers = orSection(c.Mail.Mailers, c.Mail.Default, config.Section{"driver": "log"})
	c.Malware.Default = cmp.Or(c.Malware.Default, "none")
	c.Malware.Scanners = orSections(c.Malware.Scanners, "none")
	c.Encryption.KEK = cmp.Or(c.Encryption.KEK, "local")
	c.Encryption.KEKs = orSections(c.Encryption.KEKs, "local")
	c.Languages.Default = cmp.Or(c.Languages.Default, "none")
	c.Languages.Detectors = orSections(c.Languages.Detectors, "none", "whatlang")
	c.CustomDomains.Default = cmp.Or(c.CustomDomains.Default, "none")
	c.CustomDomains.TLS = orSections(c.CustomDomains.TLS, "none")

	fakes := c.App.Env == EnvTesting
	c.Mail.AllowFakes = fakes
	c.Malware.AllowFakes = fakes
	c.Socialite.AllowFakes = fakes
	c.HTTPClient.AllowFakes = fakes
	c.DNS.AllowFakes = fakes
}

/*
orSection returns the sections, or one section under the name when
there are none.
*/
func orSection(sections map[string]config.Section, name string, section config.Section) map[string]config.Section {
	if len(sections) > 0 {
		return sections
	}

	return map[string]config.Section{name: section}
}

/*
orSections returns the sections, or one section per driver named after
it when there are none.
*/
func orSections(sections map[string]config.Section, drivers ...string) map[string]config.Section {
	if len(sections) > 0 {
		return sections
	}

	sections = make(map[string]config.Section, len(drivers))
	for _, driver := range drivers {
		sections[driver] = config.Section{"driver": driver}
	}

	return sections
}
